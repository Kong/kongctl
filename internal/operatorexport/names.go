package operatorexport

import (
	"regexp"
	"strings"
)

// maxNameLength keeps generated names well inside the 253 character limit of a
// DNS-1123 subdomain while leaving room for a disambiguating suffix.
const maxNameLength = 200

var (
	reInvalidNameChars = regexp.MustCompile(`[^a-z0-9.-]+`)
	reRepeatedDashes   = regexp.MustCompile(`-{2,}`)
)

// sanitizeName converts value into a valid Kubernetes object name (a DNS-1123
// subdomain). It returns "" when nothing usable remains.
func sanitizeName(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	name = strings.ReplaceAll(name, "*", "wildcard")
	name = strings.ReplaceAll(name, "_", "-")
	name = reInvalidNameChars.ReplaceAllString(name, "-")
	name = reRepeatedDashes.ReplaceAllString(name, "-")
	if len(name) > maxNameLength {
		name = name[:maxNameLength]
	}
	return strings.Trim(name, "-.")
}

// shortID returns the leading segment of a Konnect UUID for use in names.
func shortID(id string) string {
	short, _, _ := strings.Cut(id, "-")
	if short == "" {
		return sanitizeName(id)
	}
	return sanitizeName(short)
}

// nameAllocator assigns unique object names per kind. Entities whose preferred
// name collides with another entity of the same kind, or that have no usable
// name, get their short Konnect ID appended so the output is deterministic
// regardless of the order Konnect returns entities in.
type nameAllocator struct {
	candidates map[string]map[string]candidate // kind -> id -> base name
	counts     map[string]map[string]int       // kind -> base name -> uses
	assigned   map[string]map[string]string    // kind -> id -> final name
}

type candidate struct {
	base string
	// fallback marks a generic base used because the entity has no name of
	// its own; such names always carry the ID so they stay stable.
	fallback bool
}

func newNameAllocator() *nameAllocator {
	return &nameAllocator{
		candidates: map[string]map[string]candidate{},
		counts:     map[string]map[string]int{},
		assigned:   map[string]map[string]string{},
	}
}

// propose registers the preferred name for an entity. fallback is used as the
// base when preferred sanitizes to nothing.
func (a *nameAllocator) propose(kind, id, preferred, fallback string) {
	c := candidate{base: sanitizeName(preferred)}
	if c.base == "" {
		c.base = sanitizeName(fallback)
		c.fallback = true
	}
	if a.candidates[kind] == nil {
		a.candidates[kind] = map[string]candidate{}
		a.counts[kind] = map[string]int{}
	}
	a.candidates[kind][id] = c
	a.counts[kind][c.base]++
}

// name returns the final name for a proposed entity, or "" if it was never
// proposed.
func (a *nameAllocator) name(kind, id string) string {
	if name, ok := a.assigned[kind][id]; ok {
		return name
	}
	c, ok := a.candidates[kind][id]
	if !ok {
		return ""
	}
	name := c.base
	if c.fallback || name == "" || a.counts[kind][c.base] > 1 {
		name = strings.Trim(c.base+"-"+shortID(id), "-")
	}
	if a.assigned[kind] == nil {
		a.assigned[kind] = map[string]string{}
	}
	a.assigned[kind][id] = name
	return name
}
