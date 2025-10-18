package fsm

import "fmt"

// InvalidTransitionError is returned when a transition is not allowed
type InvalidTransitionError struct {
	From string
	To   string
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid transition from '%s' to '%s'", e.From, e.To)
}

// UnknownStateError is returned when a state is not defined in the FSM
type UnknownStateError struct {
	State string
}

func (e *UnknownStateError) Error() string {
	return fmt.Sprintf("unknown state: '%s'", e.State)
}
