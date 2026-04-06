package tui

import (
	"github.com/jongio/dispatch/internal/config"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Layout — computed once per resize, consumed by all rendering functions.
// ---------------------------------------------------------------------------

// layout holds the pre-computed panel dimensions produced by recalcLayout.
// Rendering and hit-testing read these fields instead of recomputing them,
// guaranteeing a single source of truth for every frame.
type layout struct {
	totalWidth      int
	totalHeight     int
	headerHeight    int
	footerHeight    int
	contentHeight   int
	listWidth       int
	previewWidth    int
	listHeight      int // for vertical splits: height allocated to the session list
	previewHeight   int // for vertical splits: height allocated to the preview panel
	previewPosition string
}

// recalcLayout recomputes all panel dimensions based on the current terminal
// size, preview visibility, and preview position.  It stores the result in
// m.layout and propagates sizes to every sub-component.
func (m *Model) recalcLayout() {
	contentH := m.height - styles.HeaderLines - styles.FooterLines
	if contentH < 1 {
		contentH = 1
	}

	pos := m.previewPosition
	previewW := 0
	previewH := 0
	listW := m.width
	listH := contentH

	isHorizontal := pos == config.PreviewPositionRight || pos == config.PreviewPositionLeft
	isVertical := pos == config.PreviewPositionTop || pos == config.PreviewPositionBottom

	if m.showPreview {
		if isHorizontal && m.width >= styles.PreviewMinWidth {
			previewW = int(float64(m.width) * styles.PreviewWidthRatio)
			previewH = contentH
			listW = m.width - previewW - gapWidth
			listH = contentH
		} else if isVertical && m.height >= styles.PreviewMinHeight {
			previewH = int(float64(contentH) * styles.PreviewHeightRatio)
			previewW = m.width
			listW = m.width
			listH = contentH - previewH - 1 // 1-line gap
		}
	}

	m.layout = layout{
		totalWidth:      m.width,
		totalHeight:     m.height,
		headerHeight:    styles.HeaderLines,
		footerHeight:    styles.FooterLines,
		contentHeight:   contentH,
		listWidth:       listW,
		previewWidth:    previewW,
		listHeight:      listH,
		previewHeight:   previewH,
		previewPosition: pos,
	}

	m.sessionList.SetSize(listW, listH)
	m.preview.SetSize(previewW, previewH)
	m.help.SetSize(m.width, m.height)
	m.shellPicker.SetSize(m.width, m.height)
	m.filterPanel.SetSize(m.width, m.height)
	m.configPanel.SetSize(m.width, m.height)
	m.attentionPicker.SetSize(m.width, m.height)
}

// isOverPreview returns true when the mouse coordinates fall within the
// preview panel area, accounting for the current preview position.
func (m *Model) isOverPreview(x, y int) bool {
	if m.layout.previewWidth == 0 || m.layout.previewHeight == 0 {
		return false
	}
	contentY := y - styles.HeaderLines
	if contentY < 0 {
		return false
	}
	switch m.layout.previewPosition {
	case config.PreviewPositionLeft:
		return x < m.layout.previewWidth
	case config.PreviewPositionTop:
		return contentY < m.layout.previewHeight
	case config.PreviewPositionBottom:
		return contentY >= m.layout.listHeight+1 // +1 for gap line
	default: // right
		return x >= m.layout.listWidth+gapWidth
	}
}

// cyclePreviewPosition advances the preview position: right → bottom → left → top → right.
func (m *Model) cyclePreviewPosition() {
	switch m.previewPosition {
	case config.PreviewPositionRight:
		m.previewPosition = config.PreviewPositionBottom
	case config.PreviewPositionBottom:
		m.previewPosition = config.PreviewPositionLeft
	case config.PreviewPositionLeft:
		m.previewPosition = config.PreviewPositionTop
	case config.PreviewPositionTop:
		m.previewPosition = config.PreviewPositionRight
	default:
		m.previewPosition = config.PreviewPositionBottom
	}
}

// previewContentCoords maps absolute mouse coordinates (x, y) to a content
// line index and column offset within the preview panel. Returns (-1, 0)
// when the coordinates fall outside the renderable content area (e.g.
// on the border or padding).
func (m *Model) previewContentCoords(x, y int) (contentLine, col int) {
	var previewRow int
	switch m.layout.previewPosition {
	case config.PreviewPositionTop:
		previewRow = y - styles.HeaderLines - 1 // -1 for top border
	case config.PreviewPositionBottom:
		previewRow = y - styles.HeaderLines - m.layout.listHeight - 1 - 1 // gap + top border
	case config.PreviewPositionLeft:
		previewRow = y - styles.HeaderLines - 1
	default: // right
		previewRow = y - styles.HeaderLines - 1
	}
	if previewRow < 0 {
		return -1, 0
	}

	contentLine = previewRow + m.preview.ScrollOffset()

	// Compute column relative to the preview panel's content area.
	// The preview border adds 1 char on each side and padding adds 1 char on each side.
	// Horizontal offset depends on preview position.
	var previewStartX int
	switch m.layout.previewPosition {
	case config.PreviewPositionLeft:
		previewStartX = 0
	case config.PreviewPositionTop, config.PreviewPositionBottom:
		previewStartX = 0
	default: // right
		previewStartX = m.layout.listWidth + gapWidth
	}
	// Border(1) + padding(1) = 2 chars inset from preview left edge.
	col = x - previewStartX - 2
	if col < 0 {
		col = 0
	}
	return contentLine, col
}
