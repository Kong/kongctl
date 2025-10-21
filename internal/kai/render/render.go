package render

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/muesli/termenv"
)

var (
	defaultRenderer *glamour.TermRenderer
	defaultMu       sync.RWMutex
)

// Options controls markdown rendering behaviour.
type Options struct {
	NoColor bool
	Width   int
}

// Markdown renders the provided Markdown string tailored for terminal output.
func Markdown(markdown string, opts Options) string {
	var (
		r   *glamour.TermRenderer
		err error
	)

	if opts.Width > 0 || opts.NoColor {
		r, err = newRenderer(opts)
	} else {
		r, err = getDefaultRenderer()
	}
	if err != nil {
		return markdown
	}

	str, err := r.Render(markdown)
	if err != nil {
		return markdown
	}
	return normalizeSpacing(str)
}

func normalizeSpacing(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
		if i == 0 {
			lines[i] = strings.TrimLeft(lines[i], " ")
		}
	}
	return strings.Join(lines, "\n")
}

func newRenderer(opts Options) (*glamour.TermRenderer, error) {
	options := []glamour.TermRendererOption{}
	if opts.NoColor {
		options = append(options,
			glamour.WithStandardStyle("noColor"),
			glamour.WithColorProfile(termenv.Ascii),
		)
	} else {
		options = append(options,
			glamour.WithAutoStyle(),
			glamour.WithColorProfile(termenv.TrueColor),
		)
	}
	if opts.Width > 0 {
		options = append(options, glamour.WithWordWrap(opts.Width))
	}
	return glamour.NewTermRenderer(options...)
}

func getDefaultRenderer() (*glamour.TermRenderer, error) {
	defaultMu.RLock()
	if defaultRenderer != nil {
		r := defaultRenderer
		defaultMu.RUnlock()
		return r, nil
	}
	defaultMu.RUnlock()

	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultRenderer == nil {
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithColorProfile(termenv.TrueColor),
		)
		if err != nil {
			return nil, err
		}
		defaultRenderer = r
	}

	return defaultRenderer, nil
}
