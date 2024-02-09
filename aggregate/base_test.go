package aggregate_test

import (
	"testing"

	"github.com/xfrr/cqrsify/aggregate"
	"github.com/xfrr/cqrsify/event"
)

func TestBase(t *testing.T) {
	t.Run("it should create a new base aggregate", func(t *testing.T) {
		base := aggregate.New("test-id", "test-name")
		if base == nil {
			t.Fatal("expected base to not be nil")
		}

		if base.AggregateID().String() != "test-id" {
			t.Errorf("expected ID to be %s, got %s", "test-id", base.AggregateID())
		}

		if base.AggregateName() != "test-name" {
			t.Errorf("expected Name to be %s, got %s", "test-name", base.AggregateName())
		}

		if len(base.AggregateChanges()) != 0 {
			t.Errorf("expected Events to be empty, got %d", len(base.AggregateChanges()))
		}

		if base.AggregateVersion() != 0 {
			t.Errorf("expected Version to be 0, got %d", base.AggregateVersion())
		}
	})

	t.Run("it should record a event", func(t *testing.T) {
		base := aggregate.New("test-id", "test-name")
		evt := event.New("test-id", "test.reason", &struct{}{})

		base.RecordChange(evt.Any())
		if len(base.AggregateChanges()) != 1 {
			t.Errorf("expected Events to have 1 event, got %d", len(base.AggregateChanges()))
		}
	})

	t.Run("it should commit events", func(t *testing.T) {
		base := aggregate.New("test-id", "test-name")
		evt := event.New("test-id", "test.reason", &struct{}{})

		base.RecordChange(evt.Any())
		if len(base.AggregateChanges()) != 1 {
			t.Errorf("expected Events to have 1 event, got %d", len(base.AggregateChanges()))
		}

		base.CommitChanges()
		if len(base.AggregateChanges()) != 0 {
			t.Errorf("expected Events to be empty, got %d", len(base.AggregateChanges()))
		}

		if base.AggregateVersion() != aggregate.Version(evt.Aggregate().Version) {
			t.Errorf("expected Version to be %d, got %d", evt.Aggregate().Version, base.AggregateVersion())
		}

		base.CommitChanges()
		if len(base.AggregateChanges()) != 0 {
			t.Errorf("expected Events to be empty, got %d", len(base.AggregateChanges()))
		}

		if base.AggregateVersion() != aggregate.Version(evt.Aggregate().Version) {
			t.Errorf("expected Version to be %d, got %d", evt.Aggregate().Version, base.AggregateVersion())
		}
	})

	t.Run("it should apply events", func(t *testing.T) {
		base := aggregate.New("test-id", "test-name")
		evt := event.New("test-id", "test.reason", &struct{}{})

		called := false
		base.When("test.reason", func(evt aggregate.Change) error {
			called = true
			return nil
		})

		base.ApplyChange(evt.Any())
		if !called {
			t.Error("expected handler to be called")
		}
	})

	t.Run("it should return the aggregate's id", func(t *testing.T) {
		base := aggregate.New("test-id", "test-name")
		if base.AggregateID().String() != "test-id" {
			t.Errorf("expected ID to be %s, got %s", "test-id", base.AggregateID())
		}
	})

	t.Run("it should return the aggregate's name", func(t *testing.T) {
		base := aggregate.New("test-id", "test-name")
		if base.AggregateName() != "test-name" {
			t.Errorf("expected Name to be %s, got %s", "test-name", base.AggregateName())
		}
	})

	t.Run("it should return the aggregate's version", func(t *testing.T) {
		base := aggregate.New("test-id", "test-name")
		if base.AggregateVersion() != 0 {
			t.Errorf("expected Version to be 0, got %d", base.AggregateVersion())
		}
	})
}
