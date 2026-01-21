package deck

import "fmt"

// ErrDeckNotFound indicates the deck executable could not be located.
type ErrDeckNotFound struct{}

func (e ErrDeckNotFound) Error() string {
	return "deck executable not found in PATH"
}

// ErrInvalidArgs reports invalid deck arguments or missing execution context.
type ErrInvalidArgs struct {
	Reason string
}

func (e ErrInvalidArgs) Error() string {
	if e.Reason == "" {
		return "invalid deck args"
	}
	return fmt.Sprintf("invalid deck args: %s", e.Reason)
}

// ErrConflictingFlag indicates a deck flag was provided when kongctl injects it.
type ErrConflictingFlag struct {
	Flag string
}

func (e ErrConflictingFlag) Error() string {
	if e.Flag == "" {
		return "deck args already include a kongctl-managed flag"
	}
	return fmt.Sprintf("deck args already include %s", e.Flag)
}
