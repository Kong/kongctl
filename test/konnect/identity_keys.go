package konnect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/google/uuid"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// APIKeyHTTP exercises the native SDK while keeping key fixtures scoped by the
// complete directory/parent route. No request reaches a network transport.
type APIKeyHTTP struct {
	Keys     map[string][]helpers.IdentityAPIKey
	Writes   []APIKeyWrite
	PageSize int
	Failure  string
	Before   func(*http.Request)
}

type APIKeyWrite struct {
	Method, Path string
	Body         map[string]any
}

func (a *APIKeyHTTP) SDK() *helpers.KonnectSDK {
	return &helpers.KonnectSDK{SDK: kk.New(kk.WithClient(a))}
}

func (a *APIKeyHTTP) response(request *http.Request, status int, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(bytes.NewReader(data)), Request: request,
	}, nil
}

func (a *APIKeyHTTP) Do(request *http.Request) (*http.Response, error) {
	if a.Before != nil {
		a.Before(request)
	}
	path := request.URL.Path
	position := strings.Index(path, "/api-keys")
	if position < 0 {
		return nil, fmt.Errorf("unexpected non-key fixture route")
	}
	collection := path[:position+len("/api-keys")]
	id := strings.TrimPrefix(strings.TrimPrefix(path, collection), "/")
	values := a.Keys[collection]
	if request.Method == http.MethodGet {
		if a.Failure == "read" || a.Failure == "list" && id == "" {
			return a.response(request, http.StatusBadRequest, map[string]any{"detail": "fixture-private-material"})
		}
		if id != "" {
			for _, value := range values {
				if value.ID == id {
					return a.response(request, http.StatusOK, value)
				}
			}
			return a.response(request, http.StatusNotFound, map[string]any{"detail": "missing"})
		}
		start := 0
		if after := request.URL.Query().Get("page[after]"); after != "" {
			var err error
			start, err = strconv.Atoi(after)
			if err != nil || start < 0 || start > len(values) {
				return nil, fmt.Errorf("invalid key fixture cursor")
			}
		}
		end := len(values)
		var next any
		if a.PageSize > 0 && start+a.PageSize < end {
			end = start + a.PageSize
			next = strconv.Itoa(end)
		}
		data := append([]helpers.IdentityAPIKey{}, values[start:end]...)
		return a.response(request, http.StatusOK, map[string]any{
			"data": data, "meta": map[string]any{"page": map[string]any{"size": 100, "next": next}},
		})
	}
	var body map[string]any
	if request.Body != nil && request.Method != http.MethodDelete {
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			return nil, err
		}
	}
	a.Writes = append(a.Writes, APIKeyWrite{Method: request.Method, Path: path, Body: body})
	if request.Method == http.MethodPost {
		if a.Failure == "post" {
			return a.response(request, http.StatusBadRequest, map[string]any{"detail": "fixture-private-material"})
		}
		var value helpers.IdentityAPIKey
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		value.ID = uuid.NewString()
		value.CreatedAt, value.UpdatedAt = time.Now().UTC(), time.Now().UTC()
		if value.ManagedBy == nil {
			value.ManagedBy = map[string]string{}
		}
		if a.Keys == nil {
			a.Keys = make(map[string][]helpers.IdentityAPIKey)
		}
		a.Keys[collection] = append(values, value)
		data, err = json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var issued map[string]any
		if err := json.Unmarshal(data, &issued); err != nil {
			return nil, err
		}
		issued["secret"] = "fixture-private-material"
		return a.response(request, http.StatusCreated, issued)
	}
	for i := range values {
		if values[i].ID != id {
			continue
		}
		if request.Method == http.MethodPatch {
			if name, ok := body["name"].(string); ok && strings.Contains(collection, "/principals/") {
				values[i].Name = new(name)
			}
			// This reproduces the deployed omission-clears-expiration behavior.
			values[i].ExpiresAt = nil
			if expiration, ok := body["expires_at"].(string); ok {
				parsed, err := time.Parse(time.RFC3339Nano, expiration)
				if err != nil {
					return nil, err
				}
				values[i].ExpiresAt = &parsed
			}
			return a.response(request, http.StatusOK, values[i])
		}
		if request.Method == http.MethodDelete {
			a.Keys[collection] = append(values[:i], values[i+1:]...)
			return a.response(request, http.StatusNoContent, nil)
		}
	}
	return a.response(request, http.StatusNotFound, map[string]any{"detail": "missing"})
}
