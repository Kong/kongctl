package tableview

import (
	"context"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/kong/kongctl/internal/cmd"
)

// ChildLoader knows how to fetch and render nested resources for a specific parent field.
type ChildLoader func(ctx context.Context, helper cmd.Helper, parent any) (ChildView, error)

type ChildView struct {
	Headers        []string
	Rows           []table.Row
	DetailRenderer DetailRenderer
	Title          string
	ParentType     string
	DetailContext  DetailContextProvider
	Mode           ChildViewMode
}

type resourceKey struct {
	parentType string
	field      string
}

type childRegistration struct {
	field  string
	loader ChildLoader
}

var childLoaders = map[resourceKey]childRegistration{}

// RegisterChildLoader associates a loader with a parent type and field label.
// Typically invoked from resource-specific packages during init.
func RegisterChildLoader(parentType, field string, loader ChildLoader) {
	key := resourceKey{
		parentType: strings.ToLower(strings.TrimSpace(parentType)),
		field:      normalizeHeaderKey(field),
	}
	childLoaders[key] = childRegistration{
		field:  strings.TrimSpace(field),
		loader: loader,
	}
}

// getChildLoader retrieves a loader for the given parent type and detail label.
func getChildLoader(parentType, fieldLabel string) ChildLoader {
	reg, ok := childLoaders[resourceKey{
		parentType: strings.ToLower(strings.TrimSpace(parentType)),
		field:      normalizeHeaderKey(fieldLabel),
	}]
	if !ok {
		return nil
	}
	return reg.loader
}

func childLoaderFields(parentType string) []childRegistration {
	parentType = strings.ToLower(strings.TrimSpace(parentType))
	if parentType == "" {
		return nil
	}
	seen := make(map[string]struct{})
	fields := make([]childRegistration, 0)
	for key, reg := range childLoaders {
		if key.parentType != parentType {
			continue
		}
		if _, exists := seen[key.field]; exists {
			continue
		}
		seen[key.field] = struct{}{}
		fields = append(fields, childRegistration{
			field:  reg.field,
			loader: reg.loader,
		})
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].field < fields[j].field
	})
	return fields
}

// HelperLoader is a convenience for loaders that already produce matrix data.
func HelperLoader(headers []string, rows [][]string, detail DetailRenderer, title string) ChildView {
	return ChildView{
		Headers:        headers,
		Rows:           convertRows(rows, len(headers)),
		DetailRenderer: detail,
		Title:          title,
		Mode:           ChildViewModeCollection,
	}
}

// ChildViewMode determines how a child view should be presented.
type ChildViewMode int

const (
	// ChildViewModeCollection renders the child as a tabular collection with selectable rows.
	ChildViewModeCollection ChildViewMode = iota
	// ChildViewModeDetail renders the child as another detail card view.
	ChildViewModeDetail
)
