// Package styles defines the lipgloss colour palette and reusable styles
// for the Copilot CLI Session Browser TUI.
//
// The package exposes the same set of style variables as before, but they
// are now backed by a Theme instance.  Call SetTheme() to swap the active
// palette; all exported style variables are updated atomically.
//
// When no explicit SetTheme() call is made the package initialises with
// the legacy adaptive-color palette so existing code keeps working.
package styles

import "github.com/charmbracelet/lipgloss"

// currentTheme holds the active theme.  Access via CurrentTheme().
var currentTheme *Theme

func init() {
	// Build the legacy default theme so every exported variable is
	// populated before any consumer reads them.
	applyLegacyDefaults()
}

// SetTheme swaps the active theme and updates every exported style
// variable in one shot.
func SetTheme(t *Theme) {
	if t == nil {
		return
	}
	currentTheme = t

	// Semantic colour aliases (kept for helpers / overlay code that
	// reads colour tokens directly, e.g. help.go).
	ColorPrimary = lipgloss.Color(t.Primary)
	ColorText = lipgloss.Color(t.Text)
	ColorDimmed = lipgloss.Color(t.Dimmed)

	// Styles.
	TitleStyle = t.TitleStyle
	SubtitleStyle = t.SubtitleStyle
	HeaderStyle = t.HeaderStyle

	SelectedStyle = t.SelectedStyle
	NormalStyle = t.NormalStyle
	DimmedStyle = t.DimmedStyle
	HiddenStyle = t.HiddenStyle
	GroupHeaderStyle = t.GroupHeaderStyle

	BadgeStyle = t.BadgeStyle
	ActiveBadgeStyle = t.ActiveBadgeStyle

	PreviewBorderStyle = t.PreviewBorder
	PreviewTitleStyle = t.PreviewTitle
	PreviewLabelStyle = t.PreviewLabel
	PreviewValueStyle = t.PreviewValue

	OverlayStyle = t.OverlayStyle
	OverlayTitleStyle = t.OverlayTitle

	StatusBarStyle = t.StatusBar
	SearchPromptStyle = t.SearchPrompt
	ErrorStyle = t.ErrorStyle
	SuccessStyle = t.SuccessStyle
	DimStyle = t.DimStyle
	KeyStyle = t.KeyStyle
	SpinnerStyle = t.SpinnerStyle
	SeparatorStyle = t.SeparatorStyle
	ConfigLabelStyle = t.ConfigLabel
	ConfigValueStyle = t.ConfigValue
	ConfigDimmedValue = t.ConfigDimmedValue

	ChatUserBubbleStyle = t.ChatUserBubble
	ChatAssistantBubbleStyle = t.ChatAssistantBubble
	ChatUserLabelStyle = t.ChatUserLabel
	ChatAssistantLabelStyle = t.ChatAssistantLabel

	AttentionWaitingStyle = t.AttentionWaitingStyle
	AttentionActiveStyle = t.AttentionActiveStyle
	AttentionStaleStyle = t.AttentionStaleStyle
	AttentionIdleStyle = t.AttentionIdleStyle
}

// CurrentTheme returns the active Theme (never nil after init).
func CurrentTheme() *Theme {
	return currentTheme
}

// ---------------------------------------------------------------------------
// Exported colour tokens (updated by SetTheme, consumed by help.go etc.)
// ---------------------------------------------------------------------------

var (
	// ColorPrimary is the accent color used for highlights, links, and
	// interactive elements. Updated by SetTheme.
	ColorPrimary lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7C6FF4"}

	// ColorText is the primary foreground color for body text.
	// Updated by SetTheme.
	ColorText lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "#1A1A2E", Dark: "#E4E4E7"}

	// ColorDimmed is a muted foreground color for secondary or
	// de-emphasised text. Updated by SetTheme.
	ColorDimmed lipgloss.TerminalColor = lipgloss.AdaptiveColor{Light: "#71717A", Dark: "#71717A"}
)

// ---------------------------------------------------------------------------
// Exported style variables — same names as before, updated by SetTheme.
// ---------------------------------------------------------------------------

var (
	// TitleStyle renders the main application title.
	TitleStyle lipgloss.Style

	// SubtitleStyle renders secondary header text.
	SubtitleStyle lipgloss.Style

	// HeaderStyle renders the header bar background.
	HeaderStyle lipgloss.Style

	// SelectedStyle renders the currently highlighted session or list item.
	SelectedStyle lipgloss.Style

	// NormalStyle renders unselected session rows.
	NormalStyle lipgloss.Style

	// DimmedStyle renders muted or secondary text.
	DimmedStyle lipgloss.Style

	// HiddenStyle renders sessions the user has marked as hidden.
	HiddenStyle lipgloss.Style

	// GroupHeaderStyle renders collapsible folder/group headers in tree view.
	GroupHeaderStyle lipgloss.Style

	// BadgeStyle renders inactive filter/status badges.
	BadgeStyle lipgloss.Style

	// ActiveBadgeStyle renders currently active filter badges.
	ActiveBadgeStyle lipgloss.Style

	// PreviewBorderStyle renders the preview panel border and padding.
	PreviewBorderStyle lipgloss.Style

	// PreviewTitleStyle renders the title inside the preview panel.
	PreviewTitleStyle lipgloss.Style

	// PreviewLabelStyle renders field labels in the preview panel.
	PreviewLabelStyle lipgloss.Style

	// PreviewValueStyle renders field values in the preview panel.
	PreviewValueStyle lipgloss.Style

	// OverlayStyle renders the outer frame of modal overlays.
	OverlayStyle lipgloss.Style

	// OverlayTitleStyle renders the title inside modal overlays.
	OverlayTitleStyle lipgloss.Style

	// StatusBarStyle renders the bottom status bar text.
	StatusBarStyle lipgloss.Style

	// SearchPromptStyle renders the search input prompt icon.
	SearchPromptStyle lipgloss.Style

	// ErrorStyle renders error messages.
	ErrorStyle lipgloss.Style

	// SuccessStyle renders success messages.
	SuccessStyle lipgloss.Style

	// DimStyle renders de-emphasised inline text.
	DimStyle lipgloss.Style

	// KeyStyle renders keyboard shortcut key labels.
	KeyStyle lipgloss.Style

	// SpinnerStyle renders the loading spinner.
	SpinnerStyle lipgloss.Style

	// SeparatorStyle renders horizontal separator lines.
	SeparatorStyle lipgloss.Style

	// ConfigLabelStyle renders field labels in the config panel.
	ConfigLabelStyle lipgloss.Style

	// ConfigValueStyle renders field values in the config panel.
	ConfigValueStyle lipgloss.Style

	// ConfigDimmedValue renders inactive field values in the config panel.
	ConfigDimmedValue lipgloss.Style

	// ChatUserBubbleStyle renders user message bubbles in the preview.
	ChatUserBubbleStyle lipgloss.Style

	// ChatAssistantBubbleStyle renders assistant message bubbles in the preview.
	ChatAssistantBubbleStyle lipgloss.Style

	// ChatUserLabelStyle renders the "You" label above user messages.
	ChatUserLabelStyle lipgloss.Style

	// ChatAssistantLabelStyle renders the "Copilot" label above assistant messages.
	ChatAssistantLabelStyle lipgloss.Style

	// AttentionWaitingStyle renders the dot for sessions waiting for user input.
	AttentionWaitingStyle lipgloss.Style

	// AttentionActiveStyle renders the dot for sessions where AI is working.
	AttentionActiveStyle lipgloss.Style

	// AttentionStaleStyle renders the dot for running but quiet sessions.
	AttentionStaleStyle lipgloss.Style

	// AttentionIdleStyle renders the dot for sessions that are not running.
	AttentionIdleStyle lipgloss.Style
)

// applyLegacyDefaults initialises the exported variables with the same
// adaptive-color defaults that existed before theming was introduced.
// This ensures backward compatibility when SetTheme() is never called.
func applyLegacyDefaults() {
	lp := lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7C6FF4"}
	lt := lipgloss.AdaptiveColor{Light: "#1A1A2E", Dark: "#E4E4E7"}
	ld := lipgloss.AdaptiveColor{Light: "#71717A", Dark: "#71717A"}
	lb := lipgloss.AdaptiveColor{Light: "#D4D4D8", Dark: "#3F3F46"}
	ls := lipgloss.AdaptiveColor{Light: "#EEE8FF", Dark: "#2D2250"}
	le := lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	lk := lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#4ADE80"}
	lbdg := lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A78BFA"}
	lbbg := lipgloss.AdaptiveColor{Light: "#F5F3FF", Dark: "#1E1538"}

	ColorPrimary = lp
	ColorText = lt
	ColorDimmed = ld

	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lp)
	SubtitleStyle = lipgloss.NewStyle().Foreground(ld)
	HeaderStyle = lipgloss.NewStyle().Foreground(lt).Background(lipgloss.AdaptiveColor{Light: "#F3F3F3", Dark: "#111111"})

	SelectedStyle = lipgloss.NewStyle().Bold(true).Background(ls).Foreground(lt)
	NormalStyle = lipgloss.NewStyle().Foreground(lt)
	DimmedStyle = lipgloss.NewStyle().Foreground(ld)
	HiddenStyle = lipgloss.NewStyle().Foreground(ld).Faint(true)
	GroupHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lp)

	BadgeStyle = lipgloss.NewStyle().Foreground(lbdg).Background(lbbg).Padding(0, 1)
	ActiveBadgeStyle = lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}).
		Background(lp).Padding(0, 1)

	PreviewBorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lb).Padding(0, 1)
	PreviewTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lp).Padding(0, 0, 1, 0)
	PreviewLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lt)
	PreviewValueStyle = lipgloss.NewStyle().Foreground(ld)

	OverlayStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lp).
		Foreground(lt).Padding(1, 2)
	OverlayTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lp).Padding(0, 0, 1, 0)

	StatusBarStyle = lipgloss.NewStyle().Foreground(ld)
	SearchPromptStyle = lipgloss.NewStyle().Foreground(lp).Bold(true)
	ErrorStyle = lipgloss.NewStyle().Foreground(le)
	SuccessStyle = lipgloss.NewStyle().Foreground(lk)
	DimStyle = lipgloss.NewStyle().Foreground(ld)
	KeyStyle = lipgloss.NewStyle().Foreground(lp)
	SpinnerStyle = lipgloss.NewStyle().Foreground(lp)
	SeparatorStyle = lipgloss.NewStyle().Foreground(lb)
	ConfigLabelStyle = lipgloss.NewStyle().Foreground(lt).Width(20)
	ConfigValueStyle = lipgloss.NewStyle().Foreground(lp)
	ConfigDimmedValue = lipgloss.NewStyle().Foreground(ld)

	// Chat bubble styles — thick side-bar borders only.
	ChatUserBubbleStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.ThickBorder()).
		BorderRight(true).BorderLeft(false).BorderTop(false).BorderBottom(false).
		BorderForeground(lp).
		Foreground(lt).
		PaddingLeft(1).PaddingRight(1)
	ChatAssistantBubbleStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).BorderRight(false).BorderTop(false).BorderBottom(false).
		BorderForeground(lb).
		Foreground(lt).
		PaddingLeft(1).PaddingRight(1)
	ChatUserLabelStyle = lipgloss.NewStyle().Foreground(lp).Bold(true)
	ChatAssistantLabelStyle = lipgloss.NewStyle().Foreground(ld).Bold(true)

	// Attention dot styles — legacy adaptive defaults.
	AttentionWaitingStyle = lipgloss.NewStyle().Foreground(le).Bold(true)
	AttentionActiveStyle = lipgloss.NewStyle().Foreground(lk)
	AttentionStaleStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#C19C00", Dark: "#C19C00"})
	AttentionIdleStyle = lipgloss.NewStyle().Foreground(ld).Faint(true)

	// Build a Theme struct so CurrentTheme() is never nil.
	currentTheme = &Theme{
		SchemeName: "Default",
		Primary:    "#7C6FF4",
		Text:       "#E4E4E7",
		Dimmed:     "#71717A",
		Border:     "#3F3F46",
		Selected:   "#2D2250",
		Error:      "#F87171",
		Success:    "#4ADE80",
		Badge:      "#A78BFA",
		BadgeBg:    "#1E1538",
		StatusBg:   "#18181B",
		HeaderBg:   "#111111",
		IsDark:     true,
	}
}

// ---------------------------------------------------------------------------
// Layout constants
// ---------------------------------------------------------------------------

const (
	// MinTermWidth is the minimum terminal width in columns required for
	// the TUI to render correctly.
	MinTermWidth = 60

	// PreviewMinWidth is the minimum terminal width at which the preview
	// panel becomes visible.
	PreviewMinWidth = 80

	// PreviewWidthRatio is the fraction of the total width allocated to
	// the preview panel when it is visible.
	PreviewWidthRatio = 0.38

	// HeaderLines is the number of lines reserved for the header area
	// (title + badges + separator).
	HeaderLines = 3 // header + badges + separator

	// FooterLines is the number of lines reserved for the footer status bar.
	FooterLines = 1
)
