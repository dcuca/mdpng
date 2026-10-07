// Package theme loads bundled, template-driven visual themes. Each theme is a
// directory under assets/themes/<name>/ containing a theme.json manifest, a
// theme.css (which owns ALL visual layout), an HTML template, and a fonts
// folder. Adding a theme requires no Go changes.
package theme

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/dcuca/mdpng/assets"
)

const themesRoot = "themes"

// DefaultName is the theme used when none is specified.
const DefaultName = "dark"

// Manifest is the parsed theme.json.
type Manifest struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	ChromaStyle string `json:"chromaStyle"`
	WindowTitle string `json:"windowTitle"`
}

// Theme is a fully loaded, ready-to-render theme.
type Theme struct {
	Manifest Manifest
	CSS      string // theme.css with font url()s inlined as data: URIs
	tmpl     *template.Template
}

// TemplateData is passed to the theme's HTML template.
type TemplateData struct {
	CSS     template.CSS
	Content template.HTML
	Title   string
}

// Render executes the theme's HTML template into a full, self-contained page.
func (t *Theme) Render(contentHTML, title string) (string, error) {
	var sb strings.Builder
	data := TemplateData{
		CSS:     template.CSS(t.CSS),
		Content: template.HTML(contentHTML), //nolint:gosec // content is rendered by goldmark
		Title:   title,
	}
	if err := t.tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return sb.String(), nil
}

var (
	fontURLRe    = regexp.MustCompile(`url\(\s*fonts/([^)\s"']+)\s*\)`)
	cssCommentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)
)

// Load reads and prepares the named theme from the embedded filesystem.
func Load(name string) (*Theme, error) {
	if name == "" {
		name = DefaultName
	}
	dir := path.Join(themesRoot, name)
	if _, err := fs.Stat(assets.FS, dir); err != nil {
		avail := strings.Join(List(), ", ")
		return nil, fmt.Errorf("theme %q does not exist (available: %s)", name, avail)
	}

	manifestBytes, err := fs.ReadFile(assets.FS, path.Join(dir, "theme.json"))
	if err != nil {
		return nil, fmt.Errorf("reading theme.json for %q: %w", name, err)
	}
	var m Manifest
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		return nil, fmt.Errorf("parsing theme.json for %q: %w", name, err)
	}
	if m.ChromaStyle == "" {
		m.ChromaStyle = "catppuccin-mocha"
	}

	cssBytes, err := fs.ReadFile(assets.FS, path.Join(dir, "theme.css"))
	if err != nil {
		return nil, fmt.Errorf("reading theme.css for %q: %w", name, err)
	}
	css, err := inlineFonts(string(cssBytes), dir)
	if err != nil {
		return nil, err
	}

	tmplBytes, err := fs.ReadFile(assets.FS, path.Join(dir, "template.html.tmpl"))
	if err != nil {
		return nil, fmt.Errorf("reading template for %q: %w", name, err)
	}
	tmpl, err := template.New(name).Parse(string(tmplBytes))
	if err != nil {
		return nil, fmt.Errorf("parsing template for %q: %w", name, err)
	}

	return &Theme{Manifest: m, CSS: css, tmpl: tmpl}, nil
}

// inlineFonts rewrites url(fonts/x.woff2) references into base64 data: URIs so
// the output HTML needs no external files. This is a mechanical embedding step,
// not a visual decision — all layout/typography choices stay in the CSS.
func inlineFonts(css, dir string) (string, error) {
	// Drop comments first so illustrative url(fonts/...) text in comments is
	// never mistaken for a real font reference.
	css = cssCommentRe.ReplaceAllString(css, "")
	var outerErr error
	out := fontURLRe.ReplaceAllStringFunc(css, func(match string) string {
		sub := fontURLRe.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		data, err := fs.ReadFile(assets.FS, path.Join(dir, "fonts", sub[1]))
		if err != nil {
			outerErr = fmt.Errorf("reading font %q: %w", sub[1], err)
			return match
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		return fmt.Sprintf("url(data:font/woff2;base64,%s)", b64)
	})
	if outerErr != nil {
		return "", outerErr
	}
	return out, nil
}

// List returns the names of all bundled themes, sorted.
func List() []string {
	entries, err := fs.ReadDir(assets.FS, themesRoot)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}
