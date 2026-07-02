// Package styles — icons.go defines Nerd Font icon constants and a fallback
// mechanism for terminals without a Nerd Font installed.
package styles

import "sync/atomic"

// ---------------------------------------------------------------------------
// Nerd Font availability flag
// ---------------------------------------------------------------------------

// nerdFontEnabled is atomically set to 1 when a Nerd Font is detected or
// installed, enabling rich icon rendering across the TUI.
var nerdFontEnabled int32

// SetNerdFontEnabled updates the global Nerd Font availability flag.
func SetNerdFontEnabled(enabled bool) {
	if enabled {
		atomic.StoreInt32(&nerdFontEnabled, 1)
	} else {
		atomic.StoreInt32(&nerdFontEnabled, 0)
	}
}

// NerdFontEnabled returns true when Nerd Font icons should be rendered.
func NerdFontEnabled() bool {
	return atomic.LoadInt32(&nerdFontEnabled) == 1
}

// icon returns nf when Nerd Font is enabled, otherwise fb (fallback).
func icon(nf, fb string) string {
	if NerdFontEnabled() {
		return nf
	}
	return fb
}

// ---------------------------------------------------------------------------
// Nerd Font icon codepoints (Font Awesome / Devicons subset in Nerd Fonts)
// ---------------------------------------------------------------------------

const (
	nfTerminal   = "\uf489" //  nf-oct-terminal
	nfFolder     = "\uf07b" //  nf-fa-folder
	nfFolderOpen = "\uf07c" //  nf-fa-folder_open
	nfSearch     = "\uf002" //  nf-fa-search
	nfClock      = "\uf017" //  nf-fa-clock_o
	nfBullet     = "\uf111" //  nf-fa-circle
	nfSortUp     = "▲"
	nfSortDown   = "▼"
	nfGear       = "\uf013" //  nf-fa-gear
	nfKeyboard   = "\uf11c" //  nf-fa-keyboard_o
	nfPointer    = "\uf0da" //  nf-fa-caret_right
	nfSession    = "\uf120" //  nf-fa-terminal
	nfFilter     = "\uf0b0" //  nf-fa-filter
	nfGitBranch  = "\uf418" //  nf-oct-git_branch
	nfCheck      = "\uf00c" //  nf-fa-check
	nfEyeSlash   = "\uf070" //  nf-fa-eye_slash
	nfRepo       = "\uea62" //  nf-cod-repo
	nfCalendar   = "\uf073" //  nf-fa-calendar
	nfList       = "\uf03a" //  nf-fa-list

	// Host type icons — distinguish session origin (GitHub, Azure DevOps).
	nfGitHub = "\uf09b" //  nf-fa-github
	nfADO    = "\uf0c2" //  nf-fa-cloud (Azure DevOps)
	nfHost   = "\uf233" //  nf-fa-server (host type pivot group)
	nfPencil = "\uf040" //  nf-fa-pencil
)

// ---------------------------------------------------------------------------
// Unicode / ASCII fallback characters (current app defaults)
// ---------------------------------------------------------------------------

const (
	fbTerminal   = "⚡"
	fbFolder     = "▸"
	fbFolderOpen = "▾"
	fbSearch     = "/"
	fbClock      = ""
	fbBullet     = "•"
	fbSortUp     = "▲"
	fbSortDown   = "▼"
	fbGear       = "⚙"
	fbKeyboard   = "⌨"
	fbPointer    = "▸"
	fbSession    = ""
	fbFilter     = "📁"
	fbGitBranch  = ""
	fbCheck      = "✓"
	fbEyeSlash   = "⊘"
	fbRepo       = "◆"
	fbCalendar   = "◇"
	fbList       = "≡"
	fbBranch     = "⎇" // branch pivot fallback (distinct from fbGitBranch)

	fbGitHub = "⊙"
	fbADO    = "☁"
	fbHost   = "⛁"
	fbPencil = "✎"
)

// ---------------------------------------------------------------------------
// Public icon accessors — each returns the appropriate glyph based on
// whether a Nerd Font is available.
// ---------------------------------------------------------------------------

// IconTitle returns the header title icon ("" or "⚡").
func IconTitle() string { return icon(nfTerminal, fbTerminal) }

// IconFolder returns the collapsed-folder icon ("" or "▸").
func IconFolder() string { return icon(nfFolder+" ", fbFolder) }

// IconFolderOpen returns the expanded-folder icon ("" or "▾").
func IconFolderOpen() string { return icon(nfFolderOpen+" ", fbFolderOpen) }

// IconSearch returns the search prompt icon ("" or "/").
func IconSearch() string { return icon(nfSearch, fbSearch) }

// IconPointer returns the cursor/selection indicator ("" or "▸").
func IconPointer() string { return icon(nfPointer, fbPointer) }

// IconBullet returns the bullet point icon ("" or "•").
func IconBullet() string { return icon(nfBullet, fbBullet) }

// IconSortUp returns the ascending sort arrow ("" or "↑").
func IconSortUp() string { return icon(nfSortUp, fbSortUp) }

// IconSortDown returns the descending sort arrow ("" or "↓").
func IconSortDown() string { return icon(nfSortDown, fbSortDown) }

// IconGear returns the settings/gear icon ("" or "⚙").
func IconGear() string { return icon(nfGear, fbGear) }

// IconKeyboard returns the keyboard icon ("" or "⌨").
func IconKeyboard() string { return icon(nfKeyboard, fbKeyboard) }

// IconSession returns the session/terminal icon ("" or "").
func IconSession() string { return icon(nfSession+" ", fbSession) }

// IconClock returns the clock icon ("" or "").
func IconClock() string { return icon(nfClock+" ", fbClock) }

// IconFilter returns the filter icon ("" or "📁").
func IconFilter() string { return icon(nfFilter, fbFilter) }

// IconGitBranch returns the git branch icon ("" or "").
func IconGitBranch() string { return icon(nfGitBranch+" ", fbGitBranch) }

// IconCheck returns the check/success icon ("" or "✓").
func IconCheck() string { return icon(nfCheck, fbCheck) }

// IconRepo returns the repository icon ("" or "◆").
func IconRepo() string { return icon(nfRepo+" ", fbRepo) }

// IconRepoOpen returns the expanded repository icon ("" or "◆").
func IconRepoOpen() string { return icon(nfRepo+" ", fbRepo) }

// IconCalendar returns the date/calendar icon ("" or "◇").
func IconCalendar() string { return icon(nfCalendar+" ", fbCalendar) }

// IconCalendarOpen returns the expanded date/calendar icon ("" or "◇").
func IconCalendarOpen() string { return icon(nfCalendar+" ", fbCalendar) }

// IconBranch returns the collapsed git branch icon ("" or "⎇").
func IconBranch() string { return icon(nfGitBranch+" ", fbBranch) }

// IconBranchOpen returns the expanded git branch icon ("" or "⎇").
func IconBranchOpen() string { return icon(nfGitBranch+" ", fbBranch) }

// IconHost returns the collapsed host type icon ("" or "⛁").
func IconHost() string { return icon(nfHost+" ", fbHost) }

// IconHostOpen returns the expanded host type icon ("" or "⛁").
func IconHostOpen() string { return icon(nfHost+" ", fbHost) }

// ---------------------------------------------------------------------------
// Attention status dot icons
// ---------------------------------------------------------------------------

const (
	fbAttentionDot  = "●"
	fbAttentionIdle = "○"
)

// IconAttentionWaiting returns a filled dot for "waiting for user" status.
func IconAttentionWaiting() string { return icon(nfBullet, fbAttentionDot) }

// IconAttentionActive returns a filled dot for "AI working" status.
func IconAttentionActive() string { return icon(nfBullet, fbAttentionDot) }

// IconAttentionStale returns a filled dot for "running but quiet" status.
func IconAttentionStale() string { return icon(nfBullet, fbAttentionDot) }

// IconAttentionIdle returns an open dot for "not running" status.
func IconAttentionIdle() string { return icon(nfBullet, fbAttentionIdle) }

// IconAttentionInterrupted returns a lightning bolt for "crashed/interrupted" status.
func IconAttentionInterrupted() string { return icon("\uf0e7", "⚡") }

// IconAttentionWorking returns a filled dot for "actively executing tools" status.
func IconAttentionWorking() string { return icon(nfBullet, fbAttentionDot) }

// IconAttentionThinking returns a filled dot for "generating response" status.
func IconAttentionThinking() string { return icon(nfBullet, fbAttentionDot) }

// IconAttentionCompacting returns a filled dot for "context compaction" status.
func IconAttentionCompacting() string { return icon(nfBullet, fbAttentionDot) }

// IconExpandAll returns the "expand all groups" icon (⊞).
func IconExpandAll() string { return "⊞" }

// IconCollapseAll returns the "collapse all groups" icon (⊟).
func IconCollapseAll() string { return "⊟" }

// IconPlan returns a filled dot for sessions that have a plan.md file.
func IconPlan() string { return icon(nfBullet, fbAttentionDot) }

// IconNote returns a pencil icon for sessions that have a user note.
func IconNote() string { return icon(nfPencil, fbPencil) }

// IconWorkComplete returns a check icon for sessions with all planned work complete.
func IconWorkComplete() string { return icon(nfCheck, fbCheck) }

// IconWorkIncomplete returns a warning icon for sessions with remaining planned work.
func IconWorkIncomplete() string { return icon("\uf071", "!") } // nf-fa-warning

// IconWorkAnalyzing returns a spinner icon for sessions being analyzed.
func IconWorkAnalyzing() string { return icon("\uf110", "~") } // nf-fa-spinner

// IconHostType returns an icon representing the session's host type.
// Returns empty string for unknown/empty host types (no icon shown).
func IconHostType(hostType string) string {
	switch hostType {
	case "github":
		return icon(nfGitHub, fbGitHub)
	case "ado":
		return icon(nfADO, fbADO)
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Git workspace state icons
// ---------------------------------------------------------------------------

// IconGitDirty returns a modified icon for dirty git workspaces.
func IconGitDirty() string { return icon("\uf069", "✱") } // nf-fa-asterisk

// IconGitUntracked returns a plus icon for workspaces with untracked files.
func IconGitUntracked() string { return icon("\uf067", "+") } // nf-fa-plus

// IconGitAhead returns an up-arrow icon for branches ahead of upstream.
func IconGitAhead() string { return icon("\uf062", "↑") } // nf-fa-arrow_up

// IconGitBehind returns a down-arrow icon for branches behind upstream.
func IconGitBehind() string { return icon("\uf063", "↓") } // nf-fa-arrow_down

// IconGitMissing returns a cross icon for missing session directories.
func IconGitMissing() string { return icon("\uf00d", "✗") } // nf-fa-times

// PivotGroupIcons returns the (collapsed, expanded) icons for a pivot field.
// The pivot string matches data.PivotField values ("cwd", "repository",
// "branch", "date", "host_type") or the TUI pivot mode constants ("folder",
// "repo", "host", etc).
func PivotGroupIcons(pivot string) (collapsed, expanded string) {
	switch pivot {
	case "repository", "repo":
		return IconRepo(), IconRepoOpen()
	case "branch":
		return IconBranch(), IconBranchOpen()
	case "date":
		return IconCalendar(), IconCalendarOpen()
	case "host", "host_type":
		return IconHost(), IconHostOpen()
	default: // "cwd", "folder", or anything else
		return IconFolder(), IconFolderOpen()
	}
}
