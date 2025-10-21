package kai

import (
	"errors"
	"io"
	"net"
	"net/url"
	"syscall"
)

// TransientError wraps failures that are likely caused by temporary transport issues.
type TransientError struct {
	Err error
}

// Error implements error.
func (e *TransientError) Error() string {
	if e == nil || e.Err == nil {
		return "transient error"
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *TransientError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsTransientError reports whether the provided error (or any wrapped error) is transient.
func IsTransientError(err error) bool {
	var terr *TransientError
	return errors.As(err, &terr)
}

func wrapIfTransient(err error) error {
	if err == nil {
		return nil
	}
	if IsTransientError(err) {
		return err
	}
	if isLikelyTransient(err) {
		return &TransientError{Err: err}
	}
	return err
}

func isLikelyTransient(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, io.EOF),
		errors.Is(err, io.ErrUnexpectedEOF),
		errors.Is(err, io.ErrClosedPipe),
		errors.Is(err, syscall.ECONNRESET),
		errors.Is(err, syscall.EPIPE):
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		// url.Error may wrap another transient failure.
		if urlErr.Timeout() {
			return true
		}
		return isLikelyTransient(urlErr.Err)
	}
	return false
}
