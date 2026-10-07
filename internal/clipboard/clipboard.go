// Package clipboard provides macOS clipboard access: reading text (via pbpaste)
// and writing a PNG image (via osascript).
package clipboard

import (
	"bytes"
	"fmt"
	"os/exec"
)

// ReadText returns the current clipboard contents as text.
func ReadText() (string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return "", fmt.Errorf("reading clipboard: %w", err)
	}
	return string(out), nil
}

// WritePNG copies the PNG file at absPath onto the clipboard as an image, so it
// can be pasted directly into Slack, Messages, etc.
func WritePNG(absPath string) error {
	// AppleScript reads the file as a PNG image («class PNGf») and sets it as
	// the clipboard contents.
	script := fmt.Sprintf(
		`set the clipboard to (read (POSIX file %q) as «class PNGf»)`, absPath)
	cmd := exec.Command("osascript", "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("copying image to clipboard: %s", stderr.String())
		}
		return fmt.Errorf("copying image to clipboard: %w", err)
	}
	return nil
}
