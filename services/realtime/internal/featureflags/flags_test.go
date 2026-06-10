package featureflags

import (
	"testing"
)

func TestFeatureFlagsDefaults(t *testing.T) {
	store := NewStore()

	if store.IsEnabled("nonexistent") {
		t.Error("nonexistent feature should be disabled")
	}
}

func TestFeatureFlagsEnable(t *testing.T) {
	store := NewStore()

	store.Set("new_ui", true)

	if !store.IsEnabled("new_ui") {
		t.Error("new_ui should be enabled")
	}
}

func TestFeatureFlagsDisable(t *testing.T) {
	store := NewStore()

	store.Set("new_ui", true)
	store.Set("new_ui", false)

	if store.IsEnabled("new_ui") {
		t.Error("new_ui should be disabled")
	}
}

func TestFeatureFlagsRollout(t *testing.T) {
	store := NewStore()

	store.Set("slow_rollout", true)
	store.SetRollout("slow_rollout", 50)

	enabled := 0
	for i := 0; i < 1000; i++ {
		if store.Evaluate("slow_rollout", "user-"+string(rune('A'+i%26))) {
			enabled++
		}
	}

	if enabled < 400 || enabled > 600 {
		t.Errorf("expected ~50%% rollout, got %d/1000", enabled)
	}
}

func TestFeatureFlagsList(t *testing.T) {
	store := NewStore()

	store.Set("feature_a", true)
	store.Set("feature_b", false)

	features := store.List()
	if len(features) != 2 {
		t.Errorf("expected 2 features, got %d", len(features))
	}
}

func TestFeatureFlagsConsistency(t *testing.T) {
	store := NewStore()

	store.Set("stable", true)

	for i := 0; i < 100; i++ {
		if !store.Evaluate("stable", "user-123") {
			t.Fatal("feature should be consistently enabled")
		}
	}
}

func TestFeatureFlagsUnknownRollout(t *testing.T) {
	store := NewStore()

	rollout := store.GetRollout("nonexistent")
	if rollout != 0 {
		t.Errorf("expected 0 rollout for unknown feature, got %d", rollout)
	}
}
