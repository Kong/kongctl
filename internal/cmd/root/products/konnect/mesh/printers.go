package mesh

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"
)

// Table printers reproducing the columns kumactl prints on Kong Mesh 3.
//
// kumactl's registry is three printers: a bespoke one for Dataplane and a
// generic pair keyed on scope. Everything else — every policy, every
// enterprise type, every type added by a newer release — falls through to the
// generic pair, which is why no per-type code is needed here.
//
// The Mesh printer kumactl carried on 2.x (NAME, mTLS, AGE) is deliberately
// absent: Mesh.mtls was removed from the API in Kong Mesh 3, so the column
// cannot be populated. Do not reintroduce it in any derived form.

// resourceRow is one rendered row. Columns not used by the resolved printer
// are left empty.
type resourceRow struct {
	Mesh    string
	Name    string
	Tags    string
	Address string
	Age     string
}

// headersFor returns the column set for a resource type, matching kumactl.
func headersFor(d ResourceDescriptor) []string {
	switch {
	case d.Name == "Dataplane":
		return []string{"MESH", "NAME", "TAGS", "ADDRESS", "AGE"}
	case d.IsMeshScoped():
		return []string{"MESH", "NAME", "AGE"}
	default:
		return []string{"NAME", "AGE"}
	}
}

// cellsFor projects a row onto the resolved column set.
func cellsFor(d ResourceDescriptor, row resourceRow) []string {
	switch {
	case d.Name == "Dataplane":
		return []string{row.Mesh, row.Name, row.Tags, row.Address, row.Age}
	case d.IsMeshScoped():
		return []string{row.Mesh, row.Name, row.Age}
	default:
		return []string{row.Name, row.Age}
	}
}

// buildRows projects control plane items into rows.
//
// Every column is filled whatever the resource type; which of them are printed
// is decided by headersFor and cellsFor, so no descriptor is needed here.
func buildRows(items []map[string]any, now time.Time) []resourceRow {
	rows := make([]resourceRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, resourceRow{
			Mesh:    stringField(item, "mesh"),
			Name:    stringField(item, "name"),
			Tags:    displayTags(item),
			Address: dataplaneAddress(item),
			Age:     age(item, now),
		})
	}
	return rows
}

// age renders the time since the resource was last modified, in kumactl's
// format, so that existing eyes and awk scripts read it the same way.
func age(item map[string]any, now time.Time) string {
	raw := stringField(item, "modificationTime")
	if raw == "" {
		raw = stringField(item, "creationTime")
	}
	if raw == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return "-"
	}
	return duration(now.Sub(t))
}

// duration mirrors kumactl's compact age rendering.
func duration(d time.Duration) string {
	switch seconds := int(d.Seconds()); {
	case seconds < -1:
		return "never"
	case seconds < 0:
		return "0s"
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	}
	if minutes := int(d.Minutes()); minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	if hours < 24*365 {
		return fmt.Sprintf("%dd", hours/24)
	}
	return fmt.Sprintf("%dy", hours/24/365)
}

// displayTags renders the TAGS column for a Dataplane.
//
// On Kong Mesh 3 this is the resource's labels merged with its gateway tags,
// not the inbound tags an older Kuma displayed. Labels win on conflict, which
// is what the control plane itself does.
func displayTags(item map[string]any) string {
	tags := map[string][]string{}

	for key, value := range mapField(item, "labels") {
		if s, ok := value.(string); ok {
			tags[key] = []string{s}
		}
	}

	gateway := mapField(mapField(item, "networking"), "gateway")
	for key, value := range mapField(gateway, "tags") {
		if _, taken := tags[key]; taken {
			continue
		}
		if s, ok := value.(string); ok {
			tags[key] = []string{s}
		}
	}

	rendered := make([]string, 0, len(tags))
	for _, key := range slices.Sorted(maps.Keys(tags)) {
		values := tags[key]
		sort.Strings(values)
		rendered = append(rendered, fmt.Sprintf("%s=%s", key, strings.Join(values, ",")))
	}
	sort.Strings(rendered)
	return strings.Join(rendered, " ")
}

// dataplaneAddress reads the ADDRESS column for a Dataplane.
func dataplaneAddress(item map[string]any) string {
	return stringField(mapField(item, "networking"), "address")
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

func mapField(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	if nested, ok := m[key].(map[string]any); ok {
		return nested
	}
	return nil
}
