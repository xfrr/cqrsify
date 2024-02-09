package event_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/xfrr/cqrsify/event"
)

type mockBus struct {
	lock sync.RWMutex

	publishCalls int
	publishFn    func(ctx context.Context, reason string, evt event.Event[any]) error

	subscribeCalls int
	subscribeFn    func(ctx context.Context, reason string) (<-chan event.Context[any], error)
}

func (m *mockBus) Publish(ctx context.Context, reason string, evt event.Event[any]) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.publishCalls++
	if m.publishFn != nil {
		return m.publishFn(ctx, reason, evt)
	}

	return nil
}

func (m *mockBus) Subscribe(ctx context.Context, reason string) (<-chan event.Context[any], error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.subscribeCalls++
	if m.subscribeFn != nil {
		return m.subscribeFn(ctx, reason)
	}

	return nil, nil
}

func TestHandler(t *testing.T) {
	var (
		mockSubscriber = &mockBus{}
	)

	t.Run("New", func(t *testing.T) {
		type args struct {
			name string
			fn   event.HandlerFunc[any]
		}

		cases := []struct {
			name string
			args args
		}{
			{
				name: "should create a new handler",
				args: args{
					name: "name",
					fn: func(ctx event.Context[any]) error {
						return nil
					},
				},
			},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				h := event.NewHandler[any](mockSubscriber)
				if h == nil {
					t.Error("expected handler to not be nil")
				}
			})
		}
	})
}

func TestHandle(t *testing.T) {
	var (
		mockreason = "reason"
	)

	t.Run("should return an error when handler is nil", func(t *testing.T) {
		mockSubscriber := &mockBus{
			subscribeFn: func(ctx context.Context, reason string) (<-chan event.Context[any], error) {
				return make(<-chan event.Context[any]), nil
			},
		}

		_, err := event.Subscribe[MockEventPayload](context.Background(), mockSubscriber, "reason", nil)
		if err == nil || !errors.Is(err, event.ErrNilHandler) {
			t.Fatalf("expected error to be %v, got %v", event.ErrNilHandler, err)
		}
	})

	t.Run("should return an error when subscribe fails", func(t *testing.T) {
		mockErr := errors.New("something went wrong")
		mockSubscriber := &mockBus{
			subscribeFn: func(ctx context.Context, reason string) (<-chan event.Context[any], error) {
				return nil, mockErr
			},
		}

		_, err := event.Subscribe[MockEventPayload](context.Background(), mockSubscriber, "reason", func(ctx event.Context[MockEventPayload]) error {
			return nil
		})
		if err, ok := err.(event.ErrSubscribeFailed); !ok {
			t.Fatalf("expected error to be %v, got %v", event.ErrSubscribeFailed{}, err)
		}

		expected := event.ErrSubscribeFailed{}.Wrap(mockErr)
		if err.Error() != expected.Error() {
			t.Fatalf("expected error to be %v, got %v", expected.Error(), err.Error())
		}

		unwrapped := errors.Unwrap(err)
		if unwrapped == nil || unwrapped.Error() != mockErr.Error() {
			t.Fatalf("expected error to be %v, got %v", mockErr, unwrapped)
		}
	})

	t.Run("should return an error when context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		mockSubscriber := &mockBus{
			subscribeFn: func(ctx context.Context, reason string) (<-chan event.Context[any], error) {
				return make(<-chan event.Context[any]), nil
			},
		}

		errs, err := event.Subscribe[MockEventPayload](ctx, mockSubscriber, "reason", func(ctx event.Context[MockEventPayload]) error {
			return nil
		})
		if err != nil {
			t.Fatalf("expected error to be nil, got %v", err)
		}

		select {
		case err, ok := <-errs:
			if !ok {
				t.Fatal("expected errors to be open")
			}

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected error to be %v, got %v", context.Canceled, err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("expected context to be canceled")
		}
	})

	t.Run("should return an error when casting context fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		ch := make(chan event.Context[any])

		mockSubscriber := &mockBus{
			subscribeFn: func(ctx context.Context, reason string) (<-chan event.Context[any], error) {
				return ch, nil
			},
		}

		errs, err := event.Subscribe[MockEventPayload](ctx, mockSubscriber, "reason",
			func(ctx event.Context[MockEventPayload]) error {
				return nil
			})
		if err != nil {
			t.Fatalf("expected error to be nil, got %v", err)
		}

		// publish invalid event context
		cctx := event.WithContext(ctx, event.New("id", "name", "invalid").Any())
		ch <- cctx

		defer cancel()
		// wait for context to be handled
		select {
		case err, ok := <-errs:
			if !ok {
				t.Fatal("expected errors to be open")
			}

			if errors.Unwrap(err) != event.ErrCastContext {
				t.Fatalf("expected error to be %v, got %v", event.ErrCastContext, err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("expected context to be canceled")
		}
	})

	t.Run("should return an error when handling event fails", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		ch := make(chan event.Context[any])
		mockSubscriber := &mockBus{
			subscribeFn: func(ctx context.Context, reason string) (<-chan event.Context[any], error) {
				return ch, nil
			},
		}

		errs, err := event.Subscribe[MockEventPayload](ctx, mockSubscriber, mockreason,
			func(ctx event.Context[MockEventPayload]) error {
				return errors.New("handler failed")
			})
		if err != nil {
			t.Fatalf("expected error to be nil, got %v", err)
		}

		evt := event.New("id", "name", MockEventPayload{
			Greeting: "hello",
		})

		// publish event context
		ch <- event.WithContext(ctx, evt.Any())

		defer cancel()
		// wait for event to be handled
		select {
		case err, ok := <-errs:
			if !ok {
				t.Fatal("expected errors to be open")
			}

			if err == nil || err.Error() != "handler failed" {
				t.Fatalf("expected error to be %v, got %v", "handler failed", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("expected event to be handled")
		}
	})

	t.Run("should stop handling events when context channel is closed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		ch := make(chan event.Context[any])
		mockSubscriber := &mockBus{
			subscribeFn: func(ctx context.Context, reason string) (<-chan event.Context[any], error) {
				return ch, nil
			},
		}

		errs, err := event.Subscribe[MockEventPayload](ctx, mockSubscriber, mockreason,
			func(ctx event.Context[MockEventPayload]) error {
				return nil
			})
		if err != nil {
			t.Fatalf("expected error to be nil, got %v", err)
		}

		// close event context channel
		close(ch)

		defer cancel()
		// wait for event to be handled
		select {
		case err, ok := <-errs:
			if ok {
				t.Fatalf("expected errors to be closed, got %v", err)
			}

			if err != nil {
				t.Fatalf("expected error to be nil, got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("expected event to be handled")
		}
	})

	t.Run("should handle a event without errors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		ch := make(chan event.Context[any])
		mockSubscriber := &mockBus{
			subscribeFn: func(ctx context.Context, reason string) (<-chan event.Context[any], error) {
				return ch, nil
			},
		}

		handled := make(chan struct{})
		errs, err := event.Subscribe[MockEventPayload](
			ctx, mockSubscriber, mockreason,
			func(ctx event.Context[MockEventPayload]) error {
				close(handled)
				return nil
			})
		if err != nil {
			t.Fatalf("expected error to be nil, got %v", err)
		}

		evt := event.New("id", "name", MockEventPayload{
			Greeting: "hello",
		})

		// publish event context
		ch <- event.WithContext(ctx, evt.Any())

		defer cancel()
		// wait for event to be handled
		select {
		case <-handled:
		case err, ok := <-errs:
			if !ok {
				t.Fatal("expected errors to be open")
			}
			if err != nil {
				t.Fatalf("expected error to be nil, got %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("expected event to be handled")
		}

	})
}
