// Package data — workstatus.go provides functions to parse plan.md files and
// determine whether a session's planned work has been completed.
package data

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Section name constants for plan.md header-based task parsing.
const (
	sectionTodo       = "todo"
	sectionDone       = "done"
	sectionInProgress = "inprogress"
)

// ParsePlanTasks parses plan.md content and extracts task items.
//
// It recognises two formats:
//
//  1. Markdown checkboxes: "- [ ] task" (incomplete) / "- [x] task" (complete).
//  2. Section-based headers: "## TODO:", "## DONE:", "## IN PROGRESS:" with
//     list items underneath.
//
// If both checkboxes and section headers are present the checkbox counts take
// precedence. Returns (0, 0) for empty or unparseable content.
func ParsePlanTasks(content string) (total int, done int) {
	if content == "" {
		return 0, 0
	}

	lines := strings.Split(content, "\n")

	var cbTotal, cbDone int   // checkbox-based counts
	var secTotal, secDone int // section-based counts
	var hasCheckboxes bool    // at least one checkbox found
	var hasSections bool      // at least one recognised section found
	currentSection := ""      // "todo", "done", "inprogress", or ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// ----- checkbox detection -----
		if isIncompleteCheckbox(trimmed) {
			hasCheckboxes = true
			cbTotal++
		} else if isCompleteCheckbox(trimmed) {
			hasCheckboxes = true
			cbTotal++
			cbDone++
		}

		// ----- section header detection -----
		lower := strings.ToLower(trimmed)
		if isSectionHeader(trimmed) {
			switch {
			case strings.Contains(lower, "todo") || strings.Contains(lower, "to do"):
				hasSections = true
				currentSection = sectionTodo
				continue
			case strings.Contains(lower, "done") || strings.Contains(lower, "completed"):
				hasSections = true
				currentSection = sectionDone
				continue
			case strings.Contains(lower, "in progress") || strings.Contains(lower, "in-progress"):
				hasSections = true
				currentSection = sectionInProgress
				continue
			default:
				// Unrecognised heading — leave current section.
				currentSection = ""
				continue
			}
		}

		// ----- section item counting -----
		if currentSection != "" && isListItem(trimmed) {
			secTotal++
			if currentSection == sectionDone {
				secDone++
			}
		}
	}

	// Prefer checkbox counting when checkboxes are present.
	if hasCheckboxes {
		return cbTotal, cbDone
	}
	if hasSections {
		return secTotal, secDone
	}
	return 0, 0
}

// ScanWorkStatusQuick performs a fast classification pass over a plan map.
// Sessions with plans are marked WorkStatusUnknown (pending full analysis);
// sessions without plans are marked WorkStatusNoPlan. This is O(1) per
// session since it requires no file I/O.
func ScanWorkStatusQuick(planMap map[string]bool) map[string]WorkStatusResult {
	result := make(map[string]WorkStatusResult, len(planMap))
	for id, hasPlan := range planMap {
		if hasPlan {
			result[id] = WorkStatusResult{Status: WorkStatusUnknown}
		} else {
			result[id] = WorkStatusResult{Status: WorkStatusNoPlan}
		}
	}
	return result
}

// ScanWorkStatus performs a full analysis of plan.md files for the given
// sessions. It reads each plan, parses its tasks, and returns a
// WorkStatusResult per session.
//
// The optional progressFn callback is invoked after each session is analysed
// with (completed count, total count) for progress reporting.
func ScanWorkStatus(sessionIDs []string, progressFn func(completed, total int)) map[string]WorkStatusResult {
	n := len(sessionIDs)
	result := make(map[string]WorkStatusResult, n)

	for i, id := range sessionIDs {
		result[id] = analyseSession(id)

		if progressFn != nil {
			progressFn(i+1, n)
		}
	}
	return result
}

// analyseSession reads and classifies a single session's plan.
func analyseSession(sessionID string) WorkStatusResult {
	content, err := ReadPlanContent(sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return WorkStatusResult{Status: WorkStatusNoPlan}
		}
		return WorkStatusResult{Status: WorkStatusError, Error: err}
	}
	if content == "" {
		return WorkStatusResult{Status: WorkStatusNoPlan}
	}

	taskTotal, taskDone := ParsePlanTasks(content)
	if taskTotal == 0 {
		return WorkStatusResult{Status: WorkStatusUnknown}
	}
	if taskDone == taskTotal {
		return WorkStatusResult{
			Status:     WorkStatusComplete,
			TotalTasks: taskTotal,
			DoneTasks:  taskDone,
			Detail:     fmt.Sprintf("%d/%d tasks complete", taskDone, taskTotal),
		}
	}
	return WorkStatusResult{
		Status:         WorkStatusIncomplete,
		TotalTasks:     taskTotal,
		DoneTasks:      taskDone,
		Detail:         fmt.Sprintf("%d/%d tasks complete", taskDone, taskTotal),
		RemainingItems: ParsePlanRemainingItems(content),
	}
}

// ParsePlanRemainingItems extracts the text of unchecked/incomplete tasks
// from plan.md content. It recognises the same two formats as ParsePlanTasks:
//
//  1. Checkbox-based: text from "- [ ] task" lines.
//  2. Section-based: list items under TODO / IN PROGRESS headers.
//
// Checkbox items take precedence when present, matching ParsePlanTasks
// behaviour. Returns nil when no incomplete items are found.
func ParsePlanRemainingItems(content string) []string {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")

	var cbRemaining []string  // checkbox-based remaining items
	var secRemaining []string // section-based remaining items
	var hasCheckboxes bool
	var hasSections bool
	currentSection := "" // "todo", "done", "inprogress", or ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// ----- checkbox detection -----
		if isIncompleteCheckbox(trimmed) {
			hasCheckboxes = true
			// Extract task text after "- [ ] " prefix.
			if strings.HasPrefix(trimmed, "- [ ] ") {
				if text := trimmed[len("- [ ] "):]; text != "" {
					cbRemaining = append(cbRemaining, text)
				}
			}
		} else if isCompleteCheckbox(trimmed) {
			hasCheckboxes = true
		}

		// ----- section header detection -----
		lower := strings.ToLower(trimmed)
		if isSectionHeader(trimmed) {
			switch {
			case strings.Contains(lower, "todo") || strings.Contains(lower, "to do"):
				hasSections = true
				currentSection = sectionTodo
				continue
			case strings.Contains(lower, "done") || strings.Contains(lower, "completed"):
				hasSections = true
				currentSection = sectionDone
				continue
			case strings.Contains(lower, "in progress") || strings.Contains(lower, "in-progress"):
				hasSections = true
				currentSection = sectionInProgress
				continue
			default:
				currentSection = ""
				continue
			}
		}

		// ----- section item extraction (todo + inprogress only) -----
		if currentSection != "" && currentSection != sectionDone && isListItem(trimmed) {
			if text := extractListItemText(trimmed); text != "" {
				secRemaining = append(secRemaining, text)
			}
		}
	}

	// Prefer checkbox items when checkboxes are present.
	if hasCheckboxes {
		if len(cbRemaining) == 0 {
			return nil
		}
		return cbRemaining
	}
	if hasSections {
		if len(secRemaining) == 0 {
			return nil
		}
		return secRemaining
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// isIncompleteCheckbox reports whether the trimmed line is an unchecked
// Markdown checkbox (e.g. "- [ ] task").
func isIncompleteCheckbox(line string) bool {
	return strings.HasPrefix(line, "- [ ] ") || line == "- [ ]"
}

// isCompleteCheckbox reports whether the trimmed line is a checked
// Markdown checkbox (e.g. "- [x] task" or "- [X] task").
func isCompleteCheckbox(line string) bool {
	return strings.HasPrefix(line, "- [x] ") || strings.HasPrefix(line, "- [X] ") ||
		line == "- [x]" || line == "- [X]"
}

// isSectionHeader reports whether the trimmed line is a Markdown heading
// at level 2 or deeper (i.e. starts with "## ").
func isSectionHeader(line string) bool {
	return strings.HasPrefix(line, "## ")
}

// isListItem reports whether the trimmed line starts with a Markdown list
// marker: "- ", "* ", or a numbered prefix like "1. ".
func isListItem(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return true
	}
	// Numbered list: one or more digits followed by ". ".
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && i+1 < len(line) && line[i] == '.' && line[i+1] == ' '
}

// extractListItemText strips the leading list marker ("- ", "* ", "1. ")
// from a trimmed line and returns the remaining text.
func extractListItemText(trimmed string) string {
	if strings.HasPrefix(trimmed, "- ") {
		return trimmed[2:]
	}
	if strings.HasPrefix(trimmed, "* ") {
		return trimmed[2:]
	}
	// Numbered list: one or more digits followed by ". ".
	i := 0
	for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	if i > 0 && i+1 < len(trimmed) && trimmed[i] == '.' && trimmed[i+1] == ' ' {
		return trimmed[i+2:]
	}
	return ""
}
