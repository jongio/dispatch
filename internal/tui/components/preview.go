package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/markdown"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// PreviewPanel renders a detail panel for the selected session.
type PreviewPanel struct {
	detail          *data.SessionDetail
	attentionStatus data.AttentionStatus
	width           int
	height          int
	scroll          int
	totalLines      int    // cached line count for scroll clamping
	newestFirst     bool   // conversation turn display order
	convHeaderLine  int    // content line where "Conversation" label is rendered (-1 = none)
	planContent     string // plan.md content (empty when no plan)
	planViewMode    bool   // when true, render plan instead of session detail
}

// NewPreviewPanel returns an empty PreviewPanel.
func NewPreviewPanel() PreviewPanel {
	return PreviewPanel{convHeaderLine: -1}
}

// SetConversationSort sets the conversation turn display order.
// When newestFirst is true, turns are shown in descending order (newest
// at the top). When false, turns are shown in ascending order (oldest first).
func (p *PreviewPanel) SetConversationSort(newestFirst bool) {
	p.newestFirst = newestFirst
	p.updateTotalLines()
}

// ToggleConversationSort flips the conversation sort order and returns the
// new value.
func (p *PreviewPanel) ToggleConversationSort() bool {
	p.newestFirst = !p.newestFirst
	p.updateTotalLines()
	return p.newestFirst
}

// SetDetail updates the displayed session detail.
func (p *PreviewPanel) SetDetail(d *data.SessionDetail) {
	p.detail = d
	p.scroll = 0
	p.updateTotalLines()
}

// SetAttentionStatus updates the attention status shown in the preview.
func (p *PreviewPanel) SetAttentionStatus(status data.AttentionStatus) {
	p.attentionStatus = status
	p.updateTotalLines()
}

// SetPlanContent stores the plan.md content for the current session.
// Pass an empty string to clear the plan content.
func (p *PreviewPanel) SetPlanContent(content string) {
	p.planContent = content
	p.updateTotalLines()
}

// TogglePlanView switches between plan view and normal session detail view.
// Returns the new plan view mode state. Does nothing if no plan content is set.
func (p *PreviewPanel) TogglePlanView() bool {
	if p.planContent == "" {
		return false
	}
	p.planViewMode = !p.planViewMode
	p.scroll = 0
	p.updateTotalLines()
	return p.planViewMode
}

// PlanViewMode returns true when the preview is showing plan content.
func (p *PreviewPanel) PlanViewMode() bool {
	return p.planViewMode
}

// HasPlanContent returns true when plan.md content is available.
func (p *PreviewPanel) HasPlanContent() bool {
	return p.planContent != ""
}

// ExitPlanView switches back to normal session detail view.
func (p *PreviewPanel) ExitPlanView() {
	if p.planViewMode {
		p.planViewMode = false
		p.scroll = 0
		p.updateTotalLines()
	}
}

// SetSize updates the panel dimensions.
func (p *PreviewPanel) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.updateTotalLines()
}

// ScrollUp scrolls the preview content up by n lines.
func (p *PreviewPanel) ScrollUp(n int) {
	p.scroll -= n
	if p.scroll < 0 {
		p.scroll = 0
	}
}

// ScrollDown scrolls the preview content down by n lines.
func (p *PreviewPanel) ScrollDown(n int) {
	p.scroll += n
	viewportH := max(1, p.height-2)
	maxScroll := max(0, p.totalLines-viewportH)
	if p.scroll > maxScroll {
		p.scroll = maxScroll
	}
}

// PageUp scrolls up by half the viewport height.
func (p *PreviewPanel) PageUp() {
	p.ScrollUp(max(1, (p.height-2)/2))
}

// PageDown scrolls down by half the viewport height.
func (p *PreviewPanel) PageDown() {
	p.ScrollDown(max(1, (p.height-2)/2))
}

// ScrollOffset returns the current scroll position.
func (p *PreviewPanel) ScrollOffset() int {
	return p.scroll
}

// HitConversationSort reports whether contentRow (a 0-indexed line in the
// full rendered content) falls on the "Conversation" header label. This is
// used by the mouse handler to detect clicks on the sort toggle.
func (p *PreviewPanel) HitConversationSort(contentRow int) bool {
	return p.convHeaderLine >= 0 && contentRow == p.convHeaderLine
}

// updateTotalLines recomputes the cached total line count for scroll clamping.
func (p *PreviewPanel) updateTotalLines() {
	if p.width <= 0 || p.height <= 0 {
		p.totalLines = 0
		p.convHeaderLine = -1
		return
	}
	if p.planViewMode && p.planContent != "" {
		content := p.renderPlanContent()
		p.totalLines = strings.Count(content, "\n") + 1
		p.convHeaderLine = -1
		return
	}
	if p.detail == nil {
		p.totalLines = 0
		p.convHeaderLine = -1
		return
	}
	content, convLine := p.renderContent()
	p.totalLines = strings.Count(content, "\n") + 1
	p.convHeaderLine = convLine
}

// View renders the preview panel content.
func (p PreviewPanel) View() string {
	if p.width <= 0 || p.height <= 0 {
		return ""
	}

	var content string
	if p.planViewMode && p.planContent != "" {
		content = p.renderPlanContent()
	} else {
		content, _ = p.renderContent()
	}

	// innerW = content+padding width passed to lipgloss Width() (excludes border).
	// lipgloss Width() includes padding; text wraps at innerW - hPadding(2) = p.width-4.
	innerW := max(1, p.width-2)
	innerH := max(1, p.height-2)

	// Apply scroll viewport.
	lines := strings.Split(content, "\n")
	scroll := p.scroll
	totalLines := len(lines)
	maxScroll := max(0, totalLines-innerH)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > 0 && scroll < len(lines) {
		lines = lines[scroll:]
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}

	// Scroll indicators — replace first/last visible line when content extends.
	if scroll > 0 && len(lines) > 1 {
		lines[0] = styles.DimmedStyle.Render(fmt.Sprintf("  ▲ %d lines above", scroll))
	}
	if moreBelow := totalLines - scroll - innerH; moreBelow > 0 && len(lines) > 1 {
		lines[len(lines)-1] = styles.DimmedStyle.Render(fmt.Sprintf("  ▼ %d lines below", moreBelow))
	}

	viewportContent := strings.Join(lines, "\n")

	rendered := styles.PreviewBorderStyle.
		Width(innerW).
		Height(innerH).
		Render(viewportContent)

	// Clamp output to exactly p.height lines. lipgloss Width() can
	// word-wrap content producing more lines than Height() requests
	// (Height only pads, it does not truncate).
	outLines := strings.Split(rendered, "\n")
	if len(outLines) > p.height {
		outLines = outLines[:p.height]
	} else {
		for len(outLines) < p.height {
			outLines = append(outLines, strings.Repeat(" ", p.width))
		}
	}
	return strings.Join(outLines, "\n")
}

// maxPreviewItems is the maximum number of refs, checkpoints, or files
// shown before truncation with a "… N more" indicator.
const maxPreviewItems = 5

func (p PreviewPanel) renderContent() (string, int) {
	if p.detail == nil {
		// Use height-2 to account for the border added by View().
		return lipgloss.Place(
			max(1, p.width-4), max(1, p.height-2),
			lipgloss.Center, lipgloss.Center,
			styles.DimmedStyle.Render("Select a session"),
		), -1
	}

	s := p.detail.Session
	contentW := max(1, p.width-4) // text area = total - border(2) - padding(2)

	var b strings.Builder
	convLine := -1

	// ── Title ──
	b.WriteString(styles.PreviewTitleStyle.Render(styles.IconSession()+"Session Detail") + "\n")

	// ── Summary ──
	if s.Summary != "" {
		summary := CleanSummary(s.Summary)
		wrapped := wordWrap(summary, contentW)
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(wrapped) + "\n\n")
	}

	// ── Identity fields ──
	field := func(label, value string) {
		l := styles.PreviewLabelStyle.Render(label + ": ")
		v := styles.PreviewValueStyle.Render(Truncate(value, max(1, contentW-lipgloss.Width(l))))
		b.WriteString(l + v + "\n")
	}

	field("ID", s.ID)
	field("Folder", AbbrevPath(s.Cwd))

	if s.Repository != "" {
		field("Repo", s.Repository)
	}
	if s.Branch != "" {
		field(styles.IconGitBranch()+"Branch", s.Branch)
	}

	// ── Timing & stats ──
	b.WriteString("\n")
	field(styles.IconClock()+"Created", FormatTimestamp(s.CreatedAt))
	field(styles.IconClock()+"Active", FormatTimestamp(s.LastActiveAt))

	// ── Attention status ──
	statusIcon, statusLabel, statusStyle := attentionStatusDisplay(p.attentionStatus)
	l := styles.PreviewLabelStyle.Render("Status: ")
	b.WriteString(l + statusStyle.Render(statusIcon+" "+statusLabel) + "\n")

	field("Turns", FormatInt(s.TurnCount))
	field("Files", FormatInt(s.FileCount))

	// ── References ──
	if len(p.detail.Refs) > 0 {
		b.WriteString("\n")
		b.WriteString(styles.PreviewLabelStyle.Render("References") + "\n")
		seen := make(map[string]struct{})
		count := 0
		for _, ref := range p.detail.Refs {
			key := ref.RefType + ":" + ref.RefValue
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			count++
			if count > maxPreviewItems {
				b.WriteString(styles.DimmedStyle.Render(
					fmt.Sprintf("  … and %d more", countUniqueRefs(p.detail.Refs)-maxPreviewItems)) + "\n")
				break
			}
			val := Truncate(ref.RefValue, max(1, contentW-len(ref.RefType)-6))
			b.WriteString(styles.DimmedStyle.Render(
				"  "+styles.IconBullet()+" "+ref.RefType+": "+val) + "\n")
		}
	}

	// ── Conversation ──
	if len(p.detail.Turns) > 0 {
		b.WriteString("\n")
		sep := styles.SeparatorStyle.Render(strings.Repeat("─", max(1, contentW)))
		b.WriteString(sep + "\n")

		// Record the content line where the "Conversation" label appears.
		convLine = strings.Count(b.String(), "\n")

		sortArrow := styles.IconSortUp()
		if p.newestFirst {
			sortArrow = styles.IconSortDown()
		}
		b.WriteString(styles.PreviewLabelStyle.Render("Conversation "+sortArrow) + "\n\n")

		turns := p.detail.Turns
		if p.newestFirst {
			// Reverse a copy — never mutate the original slice.
			turns = make([]data.Turn, len(p.detail.Turns))
			copy(turns, p.detail.Turns)
			for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
				turns[i], turns[j] = turns[j], turns[i]
			}
		}

		b.WriteString(RenderConversation(turns, contentW))
	}

	// ── Checkpoints ──
	if len(p.detail.Checkpoints) > 0 {
		b.WriteString("\n")
		sep := styles.SeparatorStyle.Render(strings.Repeat("─", max(1, contentW)))
		b.WriteString(sep + "\n")
		b.WriteString(styles.PreviewLabelStyle.Render(
			fmt.Sprintf("Checkpoints (%d)", len(p.detail.Checkpoints))) + "\n")
		for i, cp := range p.detail.Checkpoints {
			if i >= maxPreviewItems {
				b.WriteString(styles.DimmedStyle.Render(
					fmt.Sprintf("  … and %d more", len(p.detail.Checkpoints)-maxPreviewItems)) + "\n")
				break
			}
			b.WriteString(styles.DimmedStyle.Render(
				"  "+styles.IconBullet()+" "+Truncate(cp.Title, contentW-4)) + "\n")
		}
	}

	// ── Files ──
	if len(p.detail.Files) > 0 {
		unique := uniqueFilePaths(p.detail.Files)
		b.WriteString("\n")
		sep := styles.SeparatorStyle.Render(strings.Repeat("─", max(1, contentW)))
		b.WriteString(sep + "\n")
		b.WriteString(styles.PreviewLabelStyle.Render(
			fmt.Sprintf("Files (%d)", len(unique))) + "\n")
		for i, fp := range unique {
			if i >= maxPreviewItems {
				b.WriteString(styles.DimmedStyle.Render(
					fmt.Sprintf("  … and %d more", len(unique)-maxPreviewItems)) + "\n")
				break
			}
			b.WriteString(styles.DimmedStyle.Render(
				"  "+styles.IconBullet()+" "+Truncate(AbbrevPath(fp), contentW-4)) + "\n")
		}
	}

	return b.String(), convLine
}

// renderPlanContent renders the plan.md content as styled markdown using
// Glamour, with a cyan "Plan" header and a hint to return.
func (p PreviewPanel) renderPlanContent() string {
	contentW := max(1, p.width-4) // text area = total - border(2) - padding(2)

	var b strings.Builder

	// ── Title ──
	b.WriteString(styles.PlanIndicatorStyle.Render(styles.IconPlan()+" Plan") + "\n")
	b.WriteString(styles.DimmedStyle.Render("Press v or Esc to return") + "\n\n")

	// ── Plan body — rendered as markdown ──
	lines := markdown.RenderStatic(p.planContent, contentW)
	b.WriteString(strings.Join(lines, "\n"))

	return b.String()
}

// uniqueFilePaths returns deduplicated file paths preserving order.
func uniqueFilePaths(files []data.SessionFile) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, f := range files {
		if _, ok := seen[f.FilePath]; !ok {
			seen[f.FilePath] = struct{}{}
			result = append(result, f.FilePath)
		}
	}
	return result
}

// countUniqueRefs returns the number of unique type:value ref pairs.
func countUniqueRefs(refs []data.SessionRef) int {
	seen := make(map[string]struct{})
	for _, r := range refs {
		seen[r.RefType+":"+r.RefValue] = struct{}{}
	}
	return len(seen)
}

// wordWrap wraps text to a maximum line width, breaking on spaces.
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		lineLen := 0
		for i, word := range words {
			wLen := len([]rune(word))
			if i > 0 && lineLen+1+wLen > width {
				result.WriteString("\n")
				lineLen = 0
			} else if i > 0 {
				result.WriteString(" ")
				lineLen++
			}
			result.WriteString(word)
			lineLen += wLen
		}
		result.WriteString("\n")
	}
	return strings.TrimRight(result.String(), "\n")
}

// ---------------------------------------------------------------------------
// Chat-style conversation rendering
// ---------------------------------------------------------------------------

// RenderConversation renders all turns as a chat-style conversation with
// right-aligned user messages and left-aligned assistant messages.
func RenderConversation(turns []data.Turn, contentWidth int) string {
	if len(turns) == 0 {
		return ""
	}

	var b strings.Builder
	for i, turn := range turns {
		if i > 0 {
			// Subtle turn separator.
			sep := styles.DimmedStyle.Render("· · ·")
			b.WriteString(lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, sep) + "\n")
		}

		// User message (right-aligned).
		if turn.UserMessage != "" {
			b.WriteString(RenderChatBubble(turn.UserMessage, "You", contentWidth, true))
		}

		// Assistant response (left-aligned).
		if turn.AssistantResponse != "" {
			if turn.UserMessage != "" {
				b.WriteString("\n")
			}
			b.WriteString(RenderChatBubble(turn.AssistantResponse, "Copilot", contentWidth, false))
		}

		b.WriteString("\n")
	}
	return b.String()
}

// maxBubbleLines is the maximum number of wrapped lines shown per chat
// message before truncation with a "… N more lines" indicator.
const maxBubbleLines = 8

// bubbleInset is the number of characters to inset chat bubbles from the
// preview panel edges to prevent a double-border appearance.
const bubbleInset = 2

// RenderChatBubble renders a single chat message as a styled bubble with a
// role label. When isUser is true, both the label and bubble are
// right-aligned; otherwise they are left-aligned.
func RenderChatBubble(msg, label string, contentWidth int, isUser bool) string {
	if msg == "" {
		return ""
	}

	// Max bubble width is 75% of available width minus inset, minimum 10.
	maxBubbleW := max(10, (contentWidth-bubbleInset)*3/4)
	// Text width accounts for side border (1 char) + padding (1 char each side).
	textWidth := max(1, maxBubbleW-3)
	wrapped := wordWrap(msg, textWidth)

	// Truncate long messages.
	wrapLines := strings.Split(wrapped, "\n")
	if len(wrapLines) > maxBubbleLines {
		total := len(wrapLines)
		wrapLines = wrapLines[:maxBubbleLines-1]
		wrapLines = append(wrapLines, fmt.Sprintf("… %d more lines", total-maxBubbleLines+1))
		wrapped = strings.Join(wrapLines, "\n")
	}

	var bubbleStyle, labelStyle lipgloss.Style
	if isUser {
		bubbleStyle = styles.ChatUserBubbleStyle
		labelStyle = styles.ChatUserLabelStyle
	} else {
		bubbleStyle = styles.ChatAssistantBubbleStyle
		labelStyle = styles.ChatAssistantLabelStyle
	}

	bubble := bubbleStyle.Render(wrapped)
	styledLabel := labelStyle.Render(label)

	if isUser {
		// Right-align with inset from right edge.
		effectiveW := contentWidth - bubbleInset
		alignedLabel := lipgloss.PlaceHorizontal(effectiveW, lipgloss.Right, styledLabel)
		alignedBubble := lipgloss.PlaceHorizontal(effectiveW, lipgloss.Right, bubble)
		return alignedLabel + "\n" + alignedBubble
	}

	// Left-aligned with inset from left edge.
	pad := strings.Repeat(" ", bubbleInset)
	bubbleLines := strings.Split(bubble, "\n")
	for i, line := range bubbleLines {
		bubbleLines[i] = pad + line
	}
	return pad + styledLabel + "\n" + strings.Join(bubbleLines, "\n")
}

// attentionStatusDisplay returns the icon, label, and style for an attention
// status, matching the colors used by attentionDot in the session list.
// For idle status, PreviewValueStyle is used instead of AttentionIdleStyle
// (which uses Faint) so the text remains readable in the preview pane.
func attentionStatusDisplay(status data.AttentionStatus) (icon, label string, style lipgloss.Style) {
	switch status {
	case data.AttentionWaiting:
		return styles.IconAttentionWaiting(), "Waiting", styles.AttentionWaitingStyle
	case data.AttentionActive:
		return styles.IconAttentionActive(), "Active", styles.AttentionActiveStyle
	case data.AttentionStale:
		return styles.IconAttentionStale(), "Stale", styles.AttentionStaleStyle
	default:
		return styles.IconAttentionIdle(), "Idle", styles.PreviewValueStyle
	}
}
