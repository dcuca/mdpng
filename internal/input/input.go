// Package input obtains the Markdown source: from stdin when piped, otherwise
// from the clipboard (mirroring pbpaste-style terminal convenience).
package input

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dcuca/mdpng/internal/clipboard"
)

// Source describes where the Markdown came from.
type Source int

const (
	SourceStdin Source = iota
	SourceClipboard
)

func (s Source) String() string {
	if s == SourceStdin {
		return "stdin"
	}
	return "clipboard"
}

// Read returns the Markdown text and the source it came from. If stdin is piped
// (not a terminal) it reads stdin; otherwise it reads the clipboard. It returns
// a clear error when no usable input is found.
func Read() (string, Source, error) {
	if piped(os.Stdin) {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", SourceStdin, fmt.Errorf("reading stdin: %w", err)
		}
		if strings.TrimSpace(string(data)) == "" {
			return "", SourceStdin, fmt.Errorf("no input provided on stdin")
		}
		return string(data), SourceStdin, nil
	}

	text, err := clipboard.ReadText()
	if err != nil {
		return "", SourceClipboard, err
	}
	if strings.TrimSpace(text) == "" {
		return "", SourceClipboard, fmt.Errorf("clipboard is empty — copy some Markdown first, or pipe it in")
	}
	return text, SourceClipboard, nil
}

// piped reports whether f is connected to a pipe/file rather than a terminal.
func piped(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}
