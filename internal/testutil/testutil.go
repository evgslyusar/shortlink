package testutil

import (
	"errors"
	"testing"

	"go.uber.org/zap"
)

// NewTestLogger returns a no-op logger suitable for unit tests.
func NewTestLogger(t *testing.T) *zap.Logger {
	t.Helper()
	return zap.NewNop()
}

// AssertErrorIs checks that err matches target using errors.Is.
func AssertErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Errorf("expected error %v, got %v", target, err)
	}
}

// AssertErrorAs checks that err can be unwrapped to type T using errors.As
// and returns the typed error.
func AssertErrorAs[T any](t *testing.T, err error) T {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("expected error of type %T, got %v", target, err)
	}
	return target
}
