package fsm

import (
	"testing"
)

func TestInvalidTransitionError(t *testing.T) {
	err := &InvalidTransitionError{
		From: "A",
		To:   "C",
	}

	expectedMsg := "invalid transition from 'A' to 'C'"
	if err.Error() != expectedMsg {
		t.Errorf("InvalidTransitionError.Error() = %v, want %v", err.Error(), expectedMsg)
	}
}

func TestUnknownStateError(t *testing.T) {
	err := &UnknownStateError{
		State: "UNKNOWN",
	}

	expectedMsg := "unknown state: 'UNKNOWN'"
	if err.Error() != expectedMsg {
		t.Errorf("UnknownStateError.Error() = %v, want %v", err.Error(), expectedMsg)
	}
}
