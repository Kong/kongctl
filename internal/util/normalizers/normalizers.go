package normalizers

import (
	"strings"
)

const Indentation = `  `

type source struct {
	string
}

// LongDesc normalizes a command's long description following
// a convention
func LongDesc(s string) string {
	return source{s}.trim().string
}

// Examples normalizes a command's examples following
// a convention
func Examples(s string) string {
	if len(s) == 0 {
		return s
	}
	return source{s}.trim().indent().string
}

func (s source) trim() source {
	s.string = strings.TrimSpace(s.string)
	return s
}

func (s source) indent() source {
	indentedLines := []string{}
	for _, line := range strings.Split(s.string, "\n") {
		trimmed := strings.TrimSpace(line)
		indented := Indentation + trimmed
		indentedLines = append(indentedLines, indented)
	}
	s.string = strings.Join(indentedLines, "\n")
	return s
}
