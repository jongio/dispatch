package components

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/jongio/dispatch/internal/tui/styles"
)

// ---------------------------------------------------------------------------
// Filter panel — directory exclusion tree with multi-select.
// ---------------------------------------------------------------------------

// FilterCategory identifies the kind of filter (kept for badge compatibility).
type FilterCategory int

const (
	// FilterFolder represents a directory-based filter category.
	FilterFolder FilterCategory = iota
)

// dirGroup is a parent directory containing one or more child folders.
type dirGroup struct {
	parentPath  string
	displayName string
	children    []dirChild
	expanded    bool
}

// dirChild is a single selectable directory within a parent group.
type dirChild struct {
	fullPath    string
	displayName string
}

// filterNavItem maps a cursor position to a group or child within the tree.
type filterNavItem struct {
	groupIdx int
	childIdx int // -1 for group header
}

// FilterPanel is an overlay showing a hierarchical directory tree for
// multi-select exclusion. Users navigate with arrows, toggle with Space,
// apply with Enter, and cancel with Esc.
type FilterPanel struct {
	groups   []dirGroup
	navItems []filterNavItem
	cursor   int
	offset   int

	// Exclusion state — pending is the working copy, applied is committed.
	pending map[string]struct{}
	applied []string

	width  int
	height int
}

// NewFilterPanel returns a FilterPanel.
func NewFilterPanel() FilterPanel {
	return FilterPanel{
		pending: make(map[string]struct{}),
	}
}

// SetFolders populates the filter panel with folder paths and initialises
// the pending exclusion set from the given excluded dirs.
func (f *FilterPanel) SetFolders(folders []string, excluded []string) {
	// Build groups by grouping folders under their parent directory.
	groupMap := make(map[string][]dirChild)
	var parentOrder []string

	for _, folder := range folders {
		parent, leaf := SplitDirLeaf(folder)
		if leaf == "" {
			continue
		}
		if _, ok := groupMap[parent]; !ok {
			parentOrder = append(parentOrder, parent)
		}
		groupMap[parent] = append(groupMap[parent], dirChild{
			fullPath:    folder,
			displayName: leaf,
		})
	}

	slices.Sort(parentOrder)

	f.groups = nil
	for _, parent := range parentOrder {
		children := groupMap[parent]
		slices.SortFunc(children, func(a, b dirChild) int {
			return strings.Compare(a.displayName, b.displayName)
		})
		f.groups = append(f.groups, dirGroup{
			parentPath:  parent,
			displayName: AbbrevHome(parent),
			children:    children,
			expanded:    true,
		})
	}

	// Build navigation items.
	f.rebuildNav()

	// Initialise pending exclusions.
	f.pending = make(map[string]struct{})
	for _, dir := range excluded {
		f.pending[dir] = struct{}{}
	}
	f.applied = append([]string{}, excluded...)

	f.cursor = 0
	f.offset = 0
}

// SetOptions is a backward-compatible shim (deprecated — use SetFolders).
func (f *FilterPanel) SetOptions(folders, repos, branches []string) {
	f.SetFolders(folders, f.applied)
}

// SetActive is a no-op kept for backward compatibility with time-range
// quick-key calls (time ranges are now handled outside the filter panel).
func (f *FilterPanel) SetActive(_ FilterCategory, _ string) {}

// HasActive returns true if any exclusions are applied.
func (f *FilterPanel) HasActive() bool {
	return len(f.applied) > 0
}

// SetSize updates the overlay dimensions.
func (f *FilterPanel) SetSize(w, h int) {
	f.width = w
	f.height = h
}

// MoveUp moves the cursor up.
func (f *FilterPanel) MoveUp() {
	if f.cursor > 0 {
		f.cursor--
	}
	if f.cursor < f.offset {
		f.offset = f.cursor
	}
}

// MoveDown moves the cursor down.
func (f *FilterPanel) MoveDown() {
	if f.cursor < len(f.navItems)-1 {
		f.cursor++
	}
	maxVis := f.visibleRows()
	if f.cursor >= f.offset+maxVis {
		f.offset = f.cursor - maxVis + 1
	}
}

// ToggleExclusion toggles the exclusion state of the item under the cursor.
// On a child: toggles that directory. On a parent: toggles all children.
func (f *FilterPanel) ToggleExclusion() {
	if f.cursor < 0 || f.cursor >= len(f.navItems) {
		return
	}
	nav := f.navItems[f.cursor]
	g := &f.groups[nav.groupIdx]

	if nav.childIdx < 0 {
		// Parent: toggle all children.
		anyUnchecked := false
		for _, ch := range g.children {
			if _, ok := f.pending[ch.fullPath]; !ok {
				anyUnchecked = true
				break
			}
		}
		for _, ch := range g.children {
			if anyUnchecked {
				f.pending[ch.fullPath] = struct{}{}
			} else {
				delete(f.pending, ch.fullPath)
			}
		}
	} else {
		// Child: toggle individual directory.
		ch := g.children[nav.childIdx]
		if _, ok := f.pending[ch.fullPath]; ok {
			delete(f.pending, ch.fullPath)
		} else {
			f.pending[ch.fullPath] = struct{}{}
		}
	}
}

// ExpandGroup expands the group under the cursor.
func (f *FilterPanel) ExpandGroup() {
	if f.cursor < 0 || f.cursor >= len(f.navItems) {
		return
	}
	nav := f.navItems[f.cursor]
	if nav.childIdx < 0 && !f.groups[nav.groupIdx].expanded {
		f.groups[nav.groupIdx].expanded = true
		f.rebuildNav()
	}
}

// CollapseGroup collapses the group under the cursor.
func (f *FilterPanel) CollapseGroup() {
	if f.cursor < 0 || f.cursor >= len(f.navItems) {
		return
	}
	nav := f.navItems[f.cursor]
	if nav.childIdx < 0 && f.groups[nav.groupIdx].expanded {
		f.groups[nav.groupIdx].expanded = false
		f.rebuildNav()
	}
}

// Toggle is provided for backward compatibility. It calls ToggleExclusion
// and returns placeholder values.
func (f *FilterPanel) Toggle() (FilterCategory, string, bool) {
	f.ToggleExclusion()
	return FilterFolder, "", false
}

// Apply commits the pending exclusions and returns them as a string slice.
func (f *FilterPanel) Apply() []string {
	f.applied = nil
	for dir := range f.pending {
		f.applied = append(f.applied, dir)
	}
	slices.Sort(f.applied)
	return f.applied
}

// Cancel discards the pending exclusions (reverts to last applied state).
func (f *FilterPanel) Cancel() {
	f.pending = make(map[string]struct{})
	for _, dir := range f.applied {
		f.pending[dir] = struct{}{}
	}
}

// ClearAll removes all active filters.
func (f *FilterPanel) ClearAll() {
	f.pending = make(map[string]struct{})
	f.applied = nil
}

// ActiveBadges returns a slice of short badge strings representing the
// currently applied exclusions.
func (f *FilterPanel) ActiveBadges() []string {
	return nil
}

// View renders the filter panel as a centred overlay.
func (f FilterPanel) View() string {
	title := styles.OverlayTitleStyle.Render(styles.IconFilter() + " Filter: Exclude Directories")

	var body strings.Builder
	body.WriteString(title + "\n")

	if len(f.navItems) == 0 {
		body.WriteString("\n" + styles.DimmedStyle.Render("  No directories found") + "\n")
	} else {
		maxVis := f.visibleRows()
		end := min(f.offset+maxVis, len(f.navItems))

		for ni := f.offset; ni < end; ni++ {
			nav := f.navItems[ni]
			g := f.groups[nav.groupIdx]
			selected := ni == f.cursor

			if nav.childIdx < 0 {
				// Parent group row.
				arrow := styles.IconFolderOpen()
				if !g.expanded {
					arrow = styles.IconFolder()
				}
				indicator := "  "
				if selected {
					indicator = styles.IconPointer() + " "
				}
				line := indicator + arrow + " " + g.displayName
				if selected {
					line = styles.SelectedStyle.Render(line)
				} else {
					line = styles.GroupHeaderStyle.Render(line)
				}
				body.WriteString(line + "\n")
			} else {
				// Child directory row.
				ch := g.children[nav.childIdx]
				checkbox := "[ ]"
				if _, ok := f.pending[ch.fullPath]; ok {
					checkbox = "[x]"
				}
				indicator := "  "
				if selected {
					indicator = styles.IconPointer() + " "
				}
				line := indicator + "    " + checkbox + " " + ch.displayName
				if selected {
					line = styles.SelectedStyle.Render(line)
				}
				body.WriteString(line + "\n")
			}
		}
	}

	body.WriteString("\n" + styles.DimmedStyle.Render("Space toggle · Enter apply · Esc cancel"))

	maxW := min(60, f.width-4)
	maxW = max(maxW, 25)

	overlay := styles.OverlayStyle.
		Width(maxW).
		Render(body.String())

	return lipgloss.Place(f.width, f.height, lipgloss.Center, lipgloss.Center, overlay)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (f *FilterPanel) rebuildNav() {
	f.navItems = nil
	for gi, g := range f.groups {
		f.navItems = append(f.navItems, filterNavItem{groupIdx: gi, childIdx: -1})
		if g.expanded {
			for ci := range g.children {
				f.navItems = append(f.navItems, filterNavItem{groupIdx: gi, childIdx: ci})
			}
		}
	}
	if f.cursor >= len(f.navItems) {
		f.cursor = max(0, len(f.navItems)-1)
	}
}

func (f FilterPanel) visibleRows() int {
	v := f.height - 10
	if v < 5 {
		v = 5
	}
	return v
}
