// Package output resolves the destination PNG path.
package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultPrefix = "md-out"

// Resolve returns the path to write to, for the given format ("png" or "pdf").
//
// The default output directory is ~/Downloads:
//   - With no explicit name, the next free md-outN.<ext> in ~/Downloads is used.
//   - An explicit bare filename (e.g. "team-update") is placed in ~/Downloads.
//   - An explicit path (absolute, or containing a separator like "./x.png")
//     is honored as given, relative to the current directory.
//
// A ".<format>" extension is ensured when the explicit name lacks one.
func Resolve(explicit, format string) (string, error) {
	ext := "." + format
	baseDir, err := defaultDir()
	if err != nil {
		return "", err
	}

	if explicit != "" {
		if !hasImageExt(explicit) {
			explicit += ext
		}
		// An explicit path (absolute or containing a separator) is honored as
		// typed; a bare filename goes into the default directory.
		if filepath.IsAbs(explicit) || strings.ContainsRune(explicit, filepath.Separator) {
			return explicit, nil
		}
		return filepath.Join(baseDir, explicit), nil
	}

	for n := 1; n < 100000; n++ {
		name := fmt.Sprintf("%s%d%s", defaultPrefix, n, ext)
		path := filepath.Join(baseDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		}
	}
	return "", fmt.Errorf("could not find a free %sN%s filename in %s", defaultPrefix, ext, baseDir)
}

// hasImageExt reports whether name already ends in a recognized output
// extension, so we don't double-append one.
func hasImageExt(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".pdf")
}

// defaultDir returns ~/Downloads, creating it if necessary. If the home
// directory cannot be determined, it falls back to the current directory.
func defaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".", nil //nolint:nilerr // fall back to cwd rather than failing
	}
	dir := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("preparing output directory %s: %w", dir, err)
	}
	return dir, nil
}
