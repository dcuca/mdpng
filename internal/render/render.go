// Package render turns Markdown into a polished image: Markdown -> HTML
// (goldmark, GitHub-flavored, syntax-highlighted) -> themed HTML page ->
// headless-Chrome capture. It can emit a PNG (screenshot clipped to the theme's
// frame) or a single-page PDF (vector, text-selectable) sized to the frame.
package render

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	ghtml "github.com/yuin/goldmark/renderer/html"

	"github.com/dcuca/mdpng/internal/theme"
)

// rect holds a measured bounding box in CSS pixels.
type rect struct {
	X, Y, W, H float64
}

// MarkdownToHTML renders GitHub-flavored Markdown to an HTML fragment with
// syntax-highlighted code blocks using the given chroma style.
func MarkdownToHTML(src, chromaStyle string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // tables, strikethrough, task lists, autolinks
			highlighting.NewHighlighting(
				highlighting.WithStyle(chromaStyle),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			ghtml.WithUnsafe(), // allow inline HTML in markdown (GFM task-list markup etc.)
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", fmt.Errorf("rendering markdown: %w", err)
	}
	return buf.String(), nil
}

// buildPage renders the Markdown into a full, self-contained themed HTML page.
func buildPage(markdown string, th *theme.Theme, title string) (string, error) {
	contentHTML, err := MarkdownToHTML(markdown, th.Manifest.ChromaStyle)
	if err != nil {
		return "", err
	}
	if title == "" {
		title = th.Manifest.WindowTitle
	}
	return th.Render(contentHTML, title)
}

// ToPNG renders Markdown to PNG bytes using the given theme.
func ToPNG(markdown string, th *theme.Theme, title string) ([]byte, error) {
	pageHTML, err := buildPage(markdown, th, title)
	if err != nil {
		return nil, err
	}
	return screenshot(pageHTML)
}

// ToPDF renders Markdown to a single-page PDF sized exactly to the theme frame.
func ToPDF(markdown string, th *theme.Theme, title string) ([]byte, error) {
	pageHTML, err := buildPage(markdown, th, title)
	if err != nil {
		return nil, err
	}
	return printPDF(pageHTML)
}

// withChrome launches headless Chrome (using the installed browser) and runs fn
// with a ready, time-bounded context.
func withChrome(fn func(ctx context.Context) error) error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("hide-scrollbars", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()
	ctx, cancelTimeout := context.WithTimeout(ctx, 45*time.Second)
	defer cancelTimeout()
	return fn(ctx)
}

func dataURL(html string) string {
	return "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))
}

// loadAndMeasure returns the chromedp actions that load the page, wait for the
// bundled fonts to apply, and measure the #frame element. The frame's size is
// defined entirely by the theme CSS; the renderer only reads it back.
func loadAndMeasure(html string, out *rect, fontsReady *bool) []chromedp.Action {
	return []chromedp.Action{
		// Generous initial viewport at 2x device scale for crisp PNG output.
		emulation.SetDeviceMetricsOverride(1400, 1400, 2, false),
		chromedp.Navigate(dataURL(html)),
		chromedp.Evaluate(`document.fonts.ready.then(() => true)`, fontsReady,
			func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
				return p.WithAwaitPromise(true)
			}),
		chromedp.Sleep(120 * time.Millisecond),
		chromedp.Evaluate(`(() => {
			const r = document.getElementById('frame').getBoundingClientRect();
			return {X: r.x, Y: r.y, W: r.width, H: r.height};
		})()`, out),
	}
}

// screenshot loads the HTML and captures the #frame element at 2x, clipping to
// its measured bounds so width and auto-expanding height come from the theme.
func screenshot(html string) ([]byte, error) {
	var r rect
	var fontsReady bool
	var buf []byte

	err := withChrome(func(ctx context.Context) error {
		actions := append(loadAndMeasure(html, &r, &fontsReady),
			chromedp.ActionFunc(func(ctx context.Context) error {
				if r.W <= 0 || r.H <= 0 {
					return fmt.Errorf("could not measure rendered frame (empty content?)")
				}
				var err error
				buf, err = page.CaptureScreenshot().
					WithFormat(page.CaptureScreenshotFormatPng).
					WithCaptureBeyondViewport(true).
					WithClip(&page.Viewport{
						X:      r.X,
						Y:      r.Y,
						Width:  r.W,
						Height: r.H,
						Scale:  1,
					}).Do(ctx)
				return err
			}),
		)
		return chromedp.Run(ctx, actions...)
	})
	if err != nil {
		return nil, fmt.Errorf("rendering PNG with headless Chrome: %w", err)
	}
	return buf, nil
}

// printPDF loads the HTML and prints a single page sized exactly to the frame.
// It injects an @page rule with the measured dimensions so there is no extra
// paper margin or pagination, and prints backgrounds so gradients/colors show.
func printPDF(html string) ([]byte, error) {
	var r rect
	var fontsReady bool
	var pageSet bool
	var buf []byte

	err := withChrome(func(ctx context.Context) error {
		actions := append(loadAndMeasure(html, &r, &fontsReady),
			chromedp.ActionFunc(func(ctx context.Context) error {
				if r.W <= 0 || r.H <= 0 {
					return fmt.Errorf("could not measure rendered frame (empty content?)")
				}
				// Size the printed page to the frame so the PDF is one page with
				// no margins and nothing clipped.
				setPage := fmt.Sprintf(`(() => {
					const s = document.createElement('style');
					s.textContent = '@page { size: %.3fpx %.3fpx; margin: 0; }';
					document.head.appendChild(s);
					return true;
				})()`, r.W, r.H)
				if err := chromedp.Evaluate(setPage, &pageSet).Do(ctx); err != nil {
					return err
				}
				data, _, err := page.PrintToPDF().
					WithPrintBackground(true).
					WithPreferCSSPageSize(true).
					WithMarginTop(0).WithMarginBottom(0).
					WithMarginLeft(0).WithMarginRight(0).
					Do(ctx)
				if err != nil {
					return err
				}
				buf = data
				return nil
			}),
		)
		return chromedp.Run(ctx, actions...)
	})
	if err != nil {
		return nil, fmt.Errorf("rendering PDF with headless Chrome: %w", err)
	}
	return buf, nil
}
