// Package styles defines the lipgloss colour palette and reusable styles
// for the Copilot CLI Session Browser TUI.
//
// scheme.go holds the ColorScheme (raw ANSI palette) and Theme (derived
// semantic styles) types plus color-blending helpers.
package styles

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
)

// hexPattern validates a #RRGGBB hex color string.
var hexPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// ---------------------------------------------------------------------------
// ColorScheme — raw 16-color ANSI palette (Windows Terminal format)
// ---------------------------------------------------------------------------

// ColorScheme mirrors the Windows Terminal color scheme JSON object.
// All color values must be #RRGGBB hex strings.
type ColorScheme struct {
	Name                string `json:"name"`
	Foreground          string `json:"foreground"`
	Background          string `json:"background"`
	CursorColor         string `json:"cursorColor,omitempty"`
	SelectionBackground string `json:"selectionBackground,omitempty"`

	// Standard 8 ANSI colors.
	Black  string `json:"black"`
	Red    string `json:"red"`
	Green  string `json:"green"`
	Yellow string `json:"yellow"`
	Blue   string `json:"blue"`
	Purple string `json:"purple"`
	Cyan   string `json:"cyan"`
	White  string `json:"white"`

	// Bright variants.
	BrightBlack  string `json:"brightBlack"`
	BrightRed    string `json:"brightRed"`
	BrightGreen  string `json:"brightGreen"`
	BrightYellow string `json:"brightYellow"`
	BrightBlue   string `json:"brightBlue"`
	BrightPurple string `json:"brightPurple"`
	BrightCyan   string `json:"brightCyan"`
	BrightWhite  string `json:"brightWhite"`
}

// Palette returns the 16 ANSI colors in index order:
// [0]=Black, [1]=Red, …, [7]=White, [8]=BrightBlack, …, [15]=BrightWhite.
func (cs *ColorScheme) Palette() [16]string {
	return [16]string{
		cs.Black, cs.Red, cs.Green, cs.Yellow,
		cs.Blue, cs.Purple, cs.Cyan, cs.White,
		cs.BrightBlack, cs.BrightRed, cs.BrightGreen, cs.BrightYellow,
		cs.BrightBlue, cs.BrightPurple, cs.BrightCyan, cs.BrightWhite,
	}
}

// Validate checks that all required color fields are valid #RRGGBB values.
func (cs *ColorScheme) Validate() error {
	fields := []struct {
		name  string
		value string
	}{
		{"foreground", cs.Foreground},
		{"background", cs.Background},
		{"black", cs.Black},
		{"red", cs.Red},
		{"green", cs.Green},
		{"yellow", cs.Yellow},
		{"blue", cs.Blue},
		{"purple", cs.Purple},
		{"cyan", cs.Cyan},
		{"white", cs.White},
		{"brightBlack", cs.BrightBlack},
		{"brightRed", cs.BrightRed},
		{"brightGreen", cs.BrightGreen},
		{"brightYellow", cs.BrightYellow},
		{"brightBlue", cs.BrightBlue},
		{"brightPurple", cs.BrightPurple},
		{"brightCyan", cs.BrightCyan},
		{"brightWhite", cs.BrightWhite},
	}
	for _, f := range fields {
		if !hexPattern.MatchString(f.value) {
			return fmt.Errorf("color scheme %q: field %q has invalid hex color %q", cs.Name, f.name, f.value)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Theme — semantic colors + pre-computed lipgloss styles
// ---------------------------------------------------------------------------

// Theme holds the resolved semantic colors derived from a ColorScheme and
// all pre-computed lipgloss styles used throughout the TUI.
type Theme struct {
	// Name of the source color scheme.
	SchemeName string

	// Semantic colors (hex strings).
	Primary    string
	Text       string
	Background string
	Dimmed     string
	Border     string
	Selected   string
	Error      string
	Success    string
	Badge      string
	BadgeBg    string
	StatusBg   string
	HeaderBg   string

	// Whether the scheme is dark-background.
	IsDark bool

	// ANSIPalette holds the 16 ANSI colors from the source scheme.
	ANSIPalette [16]string

	// Pre-computed styles — match the exported var names in theme.go.
	TitleStyle        lipgloss.Style
	SubtitleStyle     lipgloss.Style
	HeaderStyle       lipgloss.Style
	SelectedStyle     lipgloss.Style
	NormalStyle       lipgloss.Style
	DimmedStyle       lipgloss.Style
	HiddenStyle       lipgloss.Style
	FavoritedStyle    lipgloss.Style
	GroupHeaderStyle  lipgloss.Style
	BadgeStyle        lipgloss.Style
	ActiveBadgeStyle  lipgloss.Style
	PreviewBorder     lipgloss.Style
	PreviewTitle      lipgloss.Style
	PreviewLabel      lipgloss.Style
	PreviewValue      lipgloss.Style
	OverlayStyle      lipgloss.Style
	OverlayTitle      lipgloss.Style
	StatusBar         lipgloss.Style
	SearchPrompt      lipgloss.Style
	ErrorStyle        lipgloss.Style
	SuccessStyle      lipgloss.Style
	DimStyle          lipgloss.Style
	KeyStyle          lipgloss.Style
	SpinnerStyle      lipgloss.Style
	SeparatorStyle    lipgloss.Style
	ConfigLabel       lipgloss.Style
	ConfigValue       lipgloss.Style
	ConfigDimmedValue lipgloss.Style

	// Chat message bubble styles.
	ChatUserBubble      lipgloss.Style
	ChatAssistantBubble lipgloss.Style
	ChatUserLabel       lipgloss.Style
	ChatAssistantLabel  lipgloss.Style

	// Attention dot styles.
	AttentionWaitingStyle     lipgloss.Style
	AttentionActiveStyle      lipgloss.Style
	AttentionStaleStyle       lipgloss.Style
	AttentionIdleStyle        lipgloss.Style
	AttentionInterruptedStyle lipgloss.Style

	// Plan indicator style.
	PlanIndicatorStyle lipgloss.Style
}

// DeriveTheme produces a complete Theme from a raw ColorScheme.
// It computes semantic colors by blending palette entries and then
// builds every lipgloss.Style used in the TUI.
func DeriveTheme(cs ColorScheme) *Theme {
	dark := isDarkHex(cs.Background)

	primary := cs.Blue
	if dark {
		primary = cs.BrightBlue
	}

	errColor := cs.Red
	if dark {
		errColor = cs.BrightRed
	}

	successColor := cs.Green
	if dark {
		successColor = cs.BrightGreen
	}

	badge := cs.Purple
	if dark {
		badge = cs.BrightPurple
	}

	// On light backgrounds the 50% fg/bg blend produces colors that are
	// too faint to meet WCAG AA contrast (e.g. One Half Light → ~2.69:1).
	// BrightBlack is the ANSI palette's canonical "dim/comment" color and
	// is specifically chosen by scheme designers for readable muted text.
	dimmed := blendHex(cs.Foreground, cs.Background, 0.45)
	if !dark {
		dimmed = cs.BrightBlack
	}
	border := blendHex(cs.Foreground, cs.Background, 0.25)
	selected := blendHex(cs.Background, cs.Blue, 0.20)
	badgeBg := blendHex(cs.Background, cs.Purple, 0.10)
	statusBg := blendHex(cs.Background, cs.Foreground, 0.05)
	headerBg := blendHex(cs.Background, cs.Foreground, 0.03)

	t := &Theme{
		SchemeName:  cs.Name,
		Primary:     primary,
		Text:        cs.Foreground,
		Background:  cs.Background,
		Dimmed:      dimmed,
		Border:      border,
		Selected:    selected,
		Error:       errColor,
		Success:     successColor,
		Badge:       badge,
		BadgeBg:     badgeBg,
		StatusBg:    statusBg,
		HeaderBg:    headerBg,
		IsDark:      dark,
		ANSIPalette: cs.Palette(),
	}

	// Build all lipgloss styles from the semantic colors.
	t.buildStyles()
	return t
}

// buildStyles populates every lipgloss.Style field on t.
func (t *Theme) buildStyles() {
	c := lipgloss.Color

	// Title / header.
	t.TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(c(t.Primary))
	t.SubtitleStyle = lipgloss.NewStyle().Foreground(c(t.Dimmed))
	t.HeaderStyle = lipgloss.NewStyle().Foreground(c(t.Text))

	// Session list.
	t.SelectedStyle = lipgloss.NewStyle().Bold(true).Background(c(t.Selected)).Foreground(c(t.Text))
	t.NormalStyle = lipgloss.NewStyle().Foreground(c(t.Text))
	t.DimmedStyle = lipgloss.NewStyle().Foreground(c(t.Dimmed))
	t.HiddenStyle = lipgloss.NewStyle().Foreground(c(t.Dimmed)).Faint(true)
	t.FavoritedStyle = lipgloss.NewStyle().Foreground(c(t.Primary)).Bold(true)
	t.GroupHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(c(t.Primary))

	// Filter badges.
	t.BadgeStyle = lipgloss.NewStyle().Foreground(c(t.Badge)).Background(c(t.BadgeBg)).Padding(0, 1)
	t.ActiveBadgeStyle = lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(contrastText(t.Primary))).
		Background(c(t.Primary)).
		Padding(0, 1)

	// Preview panel.
	t.PreviewBorder = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(c(t.Border)).
		Padding(0, 1)
	t.PreviewTitle = lipgloss.NewStyle().Bold(true).Foreground(c(t.Primary)).Padding(0, 0, 1, 0)
	t.PreviewLabel = lipgloss.NewStyle().Bold(true).Foreground(c(t.Text))
	t.PreviewValue = lipgloss.NewStyle().Foreground(c(t.Dimmed))

	// Overlay.
	t.OverlayStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(c(t.Primary)).
		Foreground(c(t.Text)).
		Background(c(t.Background)).
		Padding(1, 2)
	t.OverlayTitle = lipgloss.NewStyle().Bold(true).Foreground(c(t.Primary)).Padding(0, 0, 1, 0)

	// Status / footer / misc.
	t.StatusBar = lipgloss.NewStyle().Foreground(c(t.Dimmed))
	t.SearchPrompt = lipgloss.NewStyle().Foreground(c(t.Primary)).Bold(true)
	t.ErrorStyle = lipgloss.NewStyle().Foreground(c(t.Error))
	t.SuccessStyle = lipgloss.NewStyle().Foreground(c(t.Success))
	t.DimStyle = lipgloss.NewStyle().Foreground(c(t.Dimmed))
	t.KeyStyle = lipgloss.NewStyle().Foreground(c(t.Primary))
	t.SpinnerStyle = lipgloss.NewStyle().Foreground(c(t.Primary))
	t.SeparatorStyle = lipgloss.NewStyle().Foreground(c(t.Border))
	t.ConfigLabel = lipgloss.NewStyle().Foreground(c(t.Text)).Width(20)
	t.ConfigValue = lipgloss.NewStyle().Foreground(c(t.Primary))
	t.ConfigDimmedValue = lipgloss.NewStyle().Foreground(c(t.Dimmed))

	// Chat message bubbles — thick side-bar borders only.
	t.ChatUserBubble = lipgloss.NewStyle().
		BorderStyle(lipgloss.ThickBorder()).
		BorderRight(true).BorderLeft(false).BorderTop(false).BorderBottom(false).
		BorderForeground(c(t.Primary)).
		Foreground(c(t.Text)).
		PaddingLeft(1).PaddingRight(1)
	t.ChatAssistantBubble = lipgloss.NewStyle().
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).BorderRight(false).BorderTop(false).BorderBottom(false).
		BorderForeground(c(t.Border)).
		Foreground(c(t.Text)).
		PaddingLeft(1).PaddingRight(1)
	t.ChatUserLabel = lipgloss.NewStyle().
		Foreground(c(t.Primary)).Bold(true)
	t.ChatAssistantLabel = lipgloss.NewStyle().
		Foreground(c(t.Dimmed)).Bold(true)

	// Attention dot styles — colored dots for session attention status.
	t.AttentionWaitingStyle = lipgloss.NewStyle().Foreground(c(t.Primary)).Bold(true)
	t.AttentionActiveStyle = lipgloss.NewStyle().Foreground(c(t.Success))
	t.AttentionStaleStyle = lipgloss.NewStyle().Foreground(c(t.ANSIPalette[3])) // Yellow
	t.AttentionIdleStyle = lipgloss.NewStyle().Foreground(c(t.Dimmed)).Faint(true)
	t.AttentionInterruptedStyle = lipgloss.NewStyle().Foreground(c(t.ANSIPalette[1])).Bold(true) // Red — interrupted/crashed

	// Plan indicator — BrightCyan from the ANSI palette.
	t.PlanIndicatorStyle = lipgloss.NewStyle().Foreground(c(t.ANSIPalette[14])).Bold(true)
}

// ---------------------------------------------------------------------------
// Color helpers
// ---------------------------------------------------------------------------

// isDarkHex returns true if the given #RRGGBB color is considered dark
// (luminance < 0.5).
func isDarkHex(hex string) bool {
	c, err := colorful.Hex(strings.ToLower(hex))
	if err != nil {
		// If parsing fails, assume dark.
		return true
	}
	_, _, l := c.Hsl()
	return l < 0.5
}

// blendHex linearly blends two #RRGGBB colors. t=0 returns a, t=1 returns b.
func blendHex(a, b string, t float64) string {
	ca, errA := colorful.Hex(strings.ToLower(a))
	cb, errB := colorful.Hex(strings.ToLower(b))
	if errA != nil || errB != nil {
		return a // fallback to first color
	}
	blended := ca.BlendLab(cb, t)
	return blended.Hex()
}

// wcagLuminance returns the WCAG 2.1 relative luminance of a #RRGGBB color.
// See https://www.w3.org/TR/WCAG21/#dfn-relative-luminance.
func wcagLuminance(hex string) float64 {
	c, err := colorful.Hex(strings.ToLower(hex))
	if err != nil {
		return 0
	}
	r, g, b := c.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// contrastText returns "#FFFFFF" or "#000000" — whichever provides a higher
// WCAG contrast ratio against the given background hex color.
func contrastText(bgHex string) string {
	bgLum := wcagLuminance(bgHex)
	// Contrast with white (luminance 1.0).
	whiteRatio := (1.0 + 0.05) / (bgLum + 0.05)
	// Contrast with black (luminance 0.0).
	blackRatio := (bgLum + 0.05) / (0.0 + 0.05)
	if whiteRatio >= blackRatio {
		return "#FFFFFF"
	}
	return "#000000"
}
