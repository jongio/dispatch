//go:build screenshots

// Command screenshots generates ANSI text captures of every TUI state
// used on the Dispatch website and writes them as self-contained HTML
// files ready for Playwright screenshot capture.
//
// Usage:
//
//	go run ./cmd/screenshots [--out DIR]
//
// The companion render.mjs script converts the HTML files to PNG:
//
//	node cmd/screenshots/render.mjs [DIR]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jongio/dispatch/internal/tui"
)

const (
	demoDBRel     = "internal/data/testdata/fake_sessions.db"
	defaultOutDir = "web/public/screenshots"
	termWidth     = 120
	termHeight    = 40
)

func main() {
	outDir := defaultOutDir

	for i, arg := range os.Args[1:] {
		if arg == "--out" && i+2 < len(os.Args) {
			outDir = os.Args[i+2]
		}
	}

	// Force terminal color environment.
	os.Setenv("CLICOLOR_FORCE", "1")
	os.Setenv("COLORTERM", "truecolor")

	dbPath := findDemoDB()
	if dbPath == "" {
		fmt.Fprintln(os.Stderr, "demo db not found; run from the repo root")
		os.Exit(1)
	}

	fmt.Printf("Capturing %dx%d screenshots from %s\n", termWidth, termHeight, dbPath)

	shots, err := tui.CaptureScreenshots(dbPath, termWidth, termHeight)
	if err != nil {
		fmt.Fprintf(os.Stderr, "capture failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "creating output dir: %v\n", err)
		os.Exit(1)
	}

	for _, s := range shots {
		dir := outDir
		if s.SubDir != "" {
			dir = filepath.Join(outDir, s.SubDir)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "creating dir %s: %v\n", dir, err)
			os.Exit(1)
		}
		html := ansiToHTML(s.ANSI, s.FG, s.BG, s.Palette, s.Highlights)
		htmlPath := filepath.Join(dir, s.Name+".html")
		if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "writing %s: %v\n", htmlPath, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Wrote %d HTML files to %s\n", len(shots), outDir)
	fmt.Println("Run: node cmd/screenshots/render.mjs")
}

// ansiToHTML converts ANSI-escaped terminal output to a self-contained
// HTML document styled to look like a terminal window.
func ansiToHTML(ansi, fg, bg string, palette [16]string, highlights []tui.Highlight) string {
	body := convertANSI(ansi, palette)

	hasHL := len(highlights) > 0
	// When highlights are present, .terminal gets padding so the callout
	// border isn't clipped at the element edge by Playwright's screenshot.
	const hlPad = 6.0 // px added on each side when highlighted

	var hlHTML string
	for _, h := range highlights {
		// Convert cell coordinates to pixel positions.
		// At 14px font with ~8.41px per character and 1.2 line-height:
		const charW = 8.41  // approximate character width
		const lineH = 16.80 // 14px × 1.2
		y1 := float64(h.Row)*lineH + hlPad
		y2 := y1 + float64(h.Rows)*lineH
		x1 := float64(h.Col)*charW + hlPad
		x2 := x1 + float64(h.Cols)*charW

		// Create 4 overlay divs around the cutout to form a spotlight.
		// Top bar (full width, from top to cutout top).
		if y1 > 0 {
			hlHTML += fmt.Sprintf(
				`<div class="spotlight" style="top:0;left:0;right:0;height:%.1fpx"></div>`+"\n", y1)
		}
		// Bottom bar (full width, from cutout bottom to container bottom).
		hlHTML += fmt.Sprintf(
			`<div class="spotlight" style="top:%.1fpx;left:0;right:0;bottom:0"></div>`+"\n", y2)
		// Left bar (cutout row height, from left edge to cutout left).
		if x1 > 0 {
			hlHTML += fmt.Sprintf(
				`<div class="spotlight" style="top:%.1fpx;left:0;width:%.1fpx;height:%.1fpx"></div>`+"\n",
				y1, x1, y2-y1)
		}
		// Right bar (cutout row height, from cutout right to container right).
		hlHTML += fmt.Sprintf(
			`<div class="spotlight" style="top:%.1fpx;left:%.1fpx;right:0;height:%.1fpx"></div>`+"\n",
			y1, x2, y2-y1)

		// Accent-ring callout div around the cutout.
		// Slight expansion (pad) so the border doesn't touch the text.
		// With hlPad offsetting coordinates, there's always room at edges.
		const pad = 3.0
		cx := x1 - pad
		cy := y1 - pad
		cw := (x2 - cx) + pad
		ch := (y2 - cy) + pad
		hlHTML += fmt.Sprintf(
			`<div class="callout" style="top:%.1fpx;left:%.1fpx;width:%.1fpx;height:%.1fpx"></div>`+"\n",
			cy, cx, cw, ch)
	}

	termClass := "terminal"
	if hasHL {
		termClass = "terminal highlighted"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
%s
</style>
</head>
<body>
<div class="%s">
<pre>%s</pre>
%s</div>
</body>
</html>`, cssBase(fg, bg), termClass, body, hlHTML)
}

func cssBase(fg, bg string) string {
	return fmt.Sprintf(`* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  background: %s;
  color: %s;
  display: inline-block;
  padding: 16px;
}
.terminal {
  position: relative;
  display: inline-block;
}
.terminal.highlighted {
  padding: 6px;
}
pre {
  font-family: 'CaskaydiaCove NF', 'CaskaydiaCove Nerd Font', 'Cascadia Code', 'JetBrains Mono', 'Fira Code', 'Consolas', 'Courier New', monospace;
  font-size: 14px;
  line-height: 1.2;
  white-space: pre;
  letter-spacing: 0;
  tab-size: 8;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  font-feature-settings: normal;
}
.spotlight {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.50);
  pointer-events: none;
  z-index: 10;
}
.callout {
  position: absolute;
  pointer-events: none;
  z-index: 11;
  border: 3px solid #E8375A;
  border-radius: 4px;
  box-shadow: inset 0 0 8px rgba(232, 55, 90, 0.35);
}`, bg, fg)
}

// sgrRe matches ANSI CSI SGR sequences: ESC [ <params> m
var sgrRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// convertANSI replaces ANSI SGR escape codes with HTML span tags.
// The palette maps ANSI indices 0–15 to theme-specific hex colors.
func convertANSI(s string, palette [16]string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)

	type state struct {
		fg   string // CSS color value or ""
		bg   string
		bold bool
		dim  bool
	}

	var cur state
	spanOpen := false

	writeSpan := func(st state) {
		if spanOpen {
			b.WriteString("</span>")
			spanOpen = false
		}
		if st.fg == "" && st.bg == "" && !st.bold && !st.dim {
			return
		}
		b.WriteString(`<span style="`)
		if st.fg != "" {
			b.WriteString("color:")
			b.WriteString(st.fg)
			b.WriteByte(';')
		}
		if st.bg != "" {
			b.WriteString("background:")
			b.WriteString(st.bg)
			b.WriteByte(';')
		}
		if st.bold {
			b.WriteString("font-weight:bold;")
		}
		if st.dim {
			b.WriteString("opacity:0.6;")
		}
		b.WriteString(`">`)
		spanOpen = true
	}

	last := 0
	for _, loc := range sgrRe.FindAllStringIndex(s, -1) {
		// Write text before this escape sequence.
		if loc[0] > last {
			b.WriteString(escapeHTML(s[last:loc[0]]))
		}
		last = loc[1]

		// Parse the SGR parameters.
		inner := s[loc[0]+2 : loc[1]-1] // strip ESC[ and m
		params := parseSGR(inner)

		for i := 0; i < len(params); i++ {
			p := params[i]
			switch {
			case p == 0:
				cur = state{}
			case p == 1:
				cur.bold = true
			case p == 2:
				cur.dim = true
			case p == 22:
				cur.bold = false
				cur.dim = false
			case p == 38 && i+4 < len(params) && params[i+1] == 2:
				cur.fg = rgbCSS(params[i+2], params[i+3], params[i+4])
				i += 4
			case p == 48 && i+4 < len(params) && params[i+1] == 2:
				cur.bg = rgbCSS(params[i+2], params[i+3], params[i+4])
				i += 4
			case p == 38 && i+2 < len(params) && params[i+1] == 5:
				cur.fg = ansi256CSS(params[i+2], palette)
				i += 2
			case p == 48 && i+2 < len(params) && params[i+1] == 5:
				cur.bg = ansi256CSS(params[i+2], palette)
				i += 2
			case p >= 30 && p <= 37:
				cur.fg = palette[p-30]
			case p >= 40 && p <= 47:
				cur.bg = palette[p-40]
			case p >= 90 && p <= 97:
				cur.fg = palette[p-90+8]
			case p >= 100 && p <= 107:
				cur.bg = palette[p-100+8]
			case p == 39:
				cur.fg = ""
			case p == 49:
				cur.bg = ""
			}
		}

		writeSpan(cur)
	}

	// Trailing text.
	if last < len(s) {
		b.WriteString(escapeHTML(s[last:]))
	}
	if spanOpen {
		b.WriteString("</span>")
	}

	return b.String()
}

func parseSGR(s string) []int {
	if s == "" {
		return []int{0}
	}
	parts := strings.Split(s, ";")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		nums = append(nums, n)
	}
	return nums
}

func rgbCSS(r, g, b int) string {
	return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ansi256CSS converts a 256-color index to a CSS color.
// Indices 0–15 use the theme palette; 16–255 use the standard xterm table.
func ansi256CSS(n int, palette [16]string) string {
	if n < 16 {
		return palette[n]
	}
	if n >= 232 {
		v := 8 + (n-232)*10
		return fmt.Sprintf("rgb(%d,%d,%d)", v, v, v)
	}
	n -= 16
	r := (n / 36) * 51
	g := ((n % 36) / 6) * 51
	b := (n % 6) * 51
	return fmt.Sprintf("rgb(%d,%d,%d)", r, g, b)
}

func findDemoDB() string {
	if _, err := os.Stat(demoDBRel); err == nil {
		abs, _ := filepath.Abs(demoDBRel)
		return abs
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), demoDBRel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
