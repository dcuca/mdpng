// Command mdpng turns Markdown (from the clipboard or stdin) into a polished,
// ray.so-inspired PNG, saves it to the current directory, and copies it back to
// the clipboard — a pbpaste-style terminal convenience for sharing in Slack.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dcuca/mdpng/internal/clipboard"
	"github.com/dcuca/mdpng/internal/input"
	"github.com/dcuca/mdpng/internal/output"
	"github.com/dcuca/mdpng/internal/render"
	"github.com/dcuca/mdpng/internal/theme"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mdpng: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		outFlag    string
		themeFlag  string
		titleFlag  string
		pdfFlag    bool
		listThemes bool
	)
	flag.StringVar(&outFlag, "o", "", "output filename (default: ~/Downloads/md-outN.png)")
	flag.StringVar(&themeFlag, "theme", theme.DefaultName, "theme name")
	flag.StringVar(&titleFlag, "title", "", "optional window title bar text")
	flag.BoolVar(&pdfFlag, "pdf", false, "export a PDF instead of a PNG")
	flag.BoolVar(&listThemes, "list-themes", false, "list available themes and exit")
	flag.Usage = usage
	flag.Parse()

	if listThemes {
		fmt.Println("Available themes:")
		for _, n := range theme.List() {
			fmt.Printf("  %s\n", n)
		}
		return nil
	}

	// Load the theme early so an unknown name fails before we read input.
	th, err := theme.Load(themeFlag)
	if err != nil {
		return err
	}

	format := resolveFormat(outFlag, pdfFlag)

	markdown, src, err := input.Read()
	if err != nil {
		return err
	}

	var data []byte
	if format == "pdf" {
		data, err = render.ToPDF(markdown, th, titleFlag)
	} else {
		data, err = render.ToPNG(markdown, th, titleFlag)
	}
	if err != nil {
		return err
	}

	outPath, err := output.Resolve(outFlag, format)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	absPath, err := filepath.Abs(outPath)
	if err != nil {
		absPath = outPath
	}

	fmt.Printf("✓ rendered %s input → %s\n", src, displayPath(outPath))
	if format == "pdf" {
		// A PDF on the clipboard isn't useful for pasting into chat, so we only
		// copy images. The PDF file is saved and ready to attach.
		return nil
	}
	if clipErr := clipboard.WritePNG(absPath); clipErr != nil {
		// The file is already saved; a clipboard failure is a soft warning.
		fmt.Printf("⚠ saved the PNG, but could not copy it to the clipboard: %s\n", clipErr)
	} else {
		fmt.Println("✓ copied image to clipboard")
	}
	return nil
}

// resolveFormat picks the output format. An explicit .pdf/.png extension on -o
// wins; otherwise the --pdf flag decides; the default is PNG.
func resolveFormat(outFlag string, pdfFlag bool) string {
	lower := strings.ToLower(outFlag)
	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return "pdf"
	case strings.HasSuffix(lower, ".png"):
		return "png"
	case pdfFlag:
		return "pdf"
	default:
		return "png"
	}
}

// displayPath prefixes a bare filename with ./ for clarity.
func displayPath(p string) string {
	if !strings.ContainsRune(p, filepath.Separator) {
		return "./" + p
	}
	return p
}

func usage() {
	fmt.Fprint(os.Stderr, `mdpng — Markdown to a polished PNG

Usage:
  mdpng [flags]                 read Markdown from the clipboard
  cat notes.md | mdpng          read Markdown from stdin
  echo "# Hi" | mdpng

Flags:
  -o <file>        output filename (default: md-outN.png in ~/Downloads);
                   a .pdf extension here exports a PDF
  --pdf            export a PDF instead of a PNG
  --theme <name>   theme to use (default: `+theme.DefaultName+`)
  --title <text>   optional window title bar text
  --list-themes    list available themes and exit
  -h, --help       show this help

Output is saved to ~/Downloads (a bare -o name goes there too; an -o path like
./x.png or /abs/x.pdf is honored as given). A PNG is also copied to the
clipboard; a PDF is just saved, ready to attach.
`)
}
