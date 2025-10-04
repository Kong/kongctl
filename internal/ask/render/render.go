package render

import (
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
}

// Markdown renders the provided Markdown string tailored for terminal output.
func Markdown(markdown string, opts Options) string {
	r, err := getRenderer(opts)
	if err != nil {
		return markdown
	}

	str, err := r.Render(markdown)
	if err != nil {
		return markdown
	}
	return str
}

func getRenderer(opts Options) (*glamour.TermRenderer, error) {
	if opts.NoColor {
		return glamour.NewTermRenderer(
			glamour.WithStandardStyle("noColor"),
			glamour.WithColorProfile(termenv.Ascii),
		)
	}

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
