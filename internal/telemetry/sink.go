package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
)

// Sink is the replaceable transport for telemetry events. Implementations
// must be safe to call from a single dispatcher goroutine; concurrent Emit
// calls are not required.
//
// Implementations MUST honor ctx cancellation and deadlines. The dispatcher
// passes a bounded context (see flushTimeout) into every Emit and Close call
// so that a network-backed sink cannot wedge the dispatcher goroutine past
// process exit. Long-running I/O (HTTP POSTs, file syncs) must abort when ctx
// is done.
type Sink interface {
	Emit(ctx context.Context, e Event) error
	Close(ctx context.Context) error
}

// NoopSink discards events. Used when telemetry is disabled so the hot path
// stays allocation-free.
type NoopSink struct{}

func (NoopSink) Emit(_ context.Context, _ Event) error { return nil }
func (NoopSink) Close(_ context.Context) error         { return nil }

// fileSink appends one JSONL line per event to a fixed path. This is a debug-aid
// for developers not for end-users.
type fileSink struct {
	mu   sync.Mutex
	path string
}

// NewFileSink returns a sink that appends JSONL events to path.
// The file is opened lazily on first Emit so a disabled sink imposes no IO.
func NewFileSink(path string) Sink {
	return &fileSink{path: path}
}

func (s *fileSink) Emit(_ context.Context, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	writeErr := writeJSONLine(f, e)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (s *fileSink) Close(_ context.Context) error { return nil }

// multiSink fans out to all child sinks. It calls every child even when one
// errors and returns the first non-nil error so that, e.g., a failing file
// sink does not silence a working stderr sink.
type multiSink struct {
	children []Sink
}

// NewMultiSink composes sinks. Nil children are filtered.
func NewMultiSink(sinks ...Sink) Sink {
	cleaned := make([]Sink, 0, len(sinks))
	for _, s := range sinks {
		if s != nil {
			cleaned = append(cleaned, s)
		}
	}
	if len(cleaned) == 0 {
		return NoopSink{}
	}
	if len(cleaned) == 1 {
		return cleaned[0]
	}
	return &multiSink{children: cleaned}
}

func (m *multiSink) Emit(ctx context.Context, e Event) error {
	var firstErr error
	for _, s := range m.children {
		if err := s.Emit(ctx, e); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiSink) Close(ctx context.Context) error {
	var firstErr error
	for _, s := range m.children {
		if err := s.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func writeJSONLine(w io.Writer, e Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}
