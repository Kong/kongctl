package iostreams

import (
	"bytes"
	"io"
	"os"
	"strings"
)

var osStreams *IOStreams

type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// Empty type to represent the _type_ IOStreams . Genesis is to support a key in a Context
type Key struct{}

// StreamsKey is a global instance of the Key type
var StreamsKey = Key{}

// Get a singleton instance of the OS IOStreams
func GetOSIOStreams() *IOStreams {
	if osStreams == nil {
		osStreams = &IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		}
	}
	return osStreams
}

// Build a new instance of the OS IOStreams
func NewOSIOStreams() *IOStreams {
	return &IOStreams{
		In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr,
	}
}

func NewTestIOStreamsOnly() *IOStreams {
	return &IOStreams{
		In:     &bytes.Buffer{},
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
}

// HasTrueColorEnv reports whether the COLORTERM environment variable
// indicates TrueColor support.  The charmbracelet/colorprofile detector
// deliberately ignores COLORTERM inside tmux, but tmux does support
// TrueColor when configured — this helper lets callers restore the
// behaviour of the previous termenv-based detection.
func HasTrueColorEnv() bool {
	ct := strings.ToLower(strings.TrimSpace(os.Getenv("COLORTERM")))
	return ct == "truecolor" || ct == "24bit"
}

func NewTestIOStreams() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &IOStreams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}, in, out, errOut
}
