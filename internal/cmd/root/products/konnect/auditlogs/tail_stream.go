package auditlogs

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/itchyny/gojq"
	"github.com/kong/kongctl/internal/iostreams"
)

type tailEventEmitter struct {
	out    io.Writer
	errOut io.Writer

	mu   sync.Mutex
	jq   *gojq.Code
	expr string
}

func newTailEventEmitter(streams *iostreams.IOStreams, jqExpr string) (*tailEventEmitter, error) {
	if streams == nil || streams.Out == nil {
		return nil, fmt.Errorf("output stream is unavailable")
	}

	emitter := &tailEventEmitter{
		out:    streams.Out,
		errOut: streams.ErrOut,
	}

	jqExpr = strings.TrimSpace(jqExpr)
	if jqExpr == "" {
		return emitter, nil
	}

	parsed, err := gojq.Parse(jqExpr)
	if err != nil {
		return nil, fmt.Errorf("invalid jq expression: %w", err)
	}
	code, err := gojq.Compile(parsed)
	if err != nil {
		return nil, fmt.Errorf("failed to compile jq expression: %w", err)
	}
	emitter.expr = jqExpr
	emitter.jq = code

	return emitter, nil
}

func (e *tailEventEmitter) EmitRecords(records [][]byte) error {
	if e == nil || len(records) == 0 {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, record := range records {
		outputRecord := record
		if e.jq != nil {
			filtered, err := applyJQCodeToJSONRecord(outputRecord, e.jq)
			if err != nil {
				e.warn("skipping non-JSON or invalid jq record for filter %q: %v", e.expr, err)
				continue
			}
			outputRecord = filtered
		}

		if _, err := fmt.Fprintln(e.out, strings.TrimRight(string(outputRecord), "\n")); err != nil {
			return err
		}
	}

	return nil
}

func (e *tailEventEmitter) warn(format string, args ...any) {
	if e == nil || e.errOut == nil {
		return
	}
	_, _ = fmt.Fprintf(e.errOut, format+"\n", args...)
}

func applyJQCodeToJSONRecord(record []byte, code *gojq.Code) ([]byte, error) {
	if code == nil {
		return nil, fmt.Errorf("jq code is required")
	}

	var payload any
	if err := json.Unmarshal(record, &payload); err != nil {
		return nil, fmt.Errorf("record is not valid JSON: %w", err)
	}

	iter := code.Run(payload)
	results := make([]any, 0, 1)
	for {
		value, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := value.(error); isErr {
			return nil, fmt.Errorf("jq filter failed: %w", err)
		}
		results = append(results, normalizeGoJQValue(value))
	}

	switch len(results) {
	case 0:
		return []byte("null"), nil
	case 1:
		return json.Marshal(results[0])
	default:
		return json.Marshal(results)
	}
}

func normalizeGoJQValue(v any) any {
	switch typed := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprint(key)] = normalizeGoJQValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = normalizeGoJQValue(typed[i])
		}
		return out
	default:
		return typed
	}
}
