package copilot

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sdk "github.com/github/copilot-sdk/go"
	"github.com/jongio/dispatch/internal/data"
)

// toolResultLimit is the maximum number of results returned by Copilot SDK
// tool handlers.  Keeping the limit modest avoids bloating the conversation
// context with too many results.
const toolResultLimit = 20

// ---------------------------------------------------------------------------
// Tool parameter / result types
// ---------------------------------------------------------------------------

// SearchSessionsParams are the arguments accepted by the search_sessions tool.
type SearchSessionsParams struct {
	Query  string `json:"query,omitempty"  jsonschema:"Keyword or phrase to search for"`
	Repo   string `json:"repo,omitempty"   jsonschema:"Filter by repository name"`
	Branch string `json:"branch,omitempty" jsonschema:"Filter by git branch"`
	Since  string `json:"since,omitempty"  jsonschema:"Only sessions updated after this ISO-8601 date"`
	Until  string `json:"until,omitempty"  jsonschema:"Only sessions updated before this ISO-8601 date"`
}

// GetSessionDetailParams are the arguments for get_session_detail.
type GetSessionDetailParams struct {
	ID string `json:"id" jsonschema:"The session ID to retrieve"`
}

// SearchDeepParams are the arguments for the deep FTS5 search tool.
type SearchDeepParams struct {
	Query string `json:"query" jsonschema:"Full-text search query across all session content"`
}

// ListReposParams is used for the list_repositories tool.
// The unused field ensures the generated JSON schema includes a "properties"
// key — the API rejects object schemas without one.
type ListReposParams struct {
	Filter string `json:"filter,omitempty" jsonschema:"Optional substring to filter repository names"`
}

// AnalyzeCompletionParams are the arguments for the analyze_completion tool.
type AnalyzeCompletionParams struct {
	SessionID   string `json:"session_id"    jsonschema:"The session ID to analyze"`
	PlanContent string `json:"plan_content"  jsonschema:"The plan.md content for this session"`
}

// analysisFilesLimit caps the number of file paths included in the
// completion analysis context to keep the AI prompt reasonably sized.
const analysisFilesLimit = 50

// analysisContext is the structured data returned by the analyze_completion
// tool for the AI to reason about.
type analysisContext struct {
	SessionID    string              `json:"session_id"`
	Summary      string              `json:"summary"`
	Repository   string              `json:"repository"`
	Branch       string              `json:"branch"`
	TurnCount    int                 `json:"turn_count"`
	FilesTouched []string            `json:"files_touched"`
	Checkpoints  []checkpointSummary `json:"checkpoints"`
	PlanContent  string              `json:"plan_content"`
}

// checkpointSummary contains the checkpoint fields relevant to
// completion analysis — title, what was done, and what remains.
type checkpointSummary struct {
	Title     string `json:"title"`
	WorkDone  string `json:"work_done"`
	NextSteps string `json:"next_steps"`
}

// ---------------------------------------------------------------------------
// Tool definitions
// ---------------------------------------------------------------------------

// defineTools returns the set of Copilot tools wired to the given data.Store.
func defineTools(store *data.Store) []sdk.Tool {
	return []sdk.Tool{
		defineSearchSessionsTool(store),
		defineGetSessionDetailTool(store),
		defineListRepositoriesTool(store),
		defineSearchDeepTool(store),
		defineAnalyzeCompletionTool(store),
	}
}

// parseFlexibleDate parses a date string in either RFC3339 or "2006-01-02" format.
func parseFlexibleDate(s, fieldName string) (*time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
		if err != nil {
			return nil, fmt.Errorf("invalid %s date %q: %w", fieldName, s, err)
		}
	}
	return &t, nil
}

func defineSearchSessionsTool(store *data.Store) sdk.Tool {
	return sdk.DefineTool(
		"search_sessions",
		"Search Copilot CLI sessions by keyword, repository, branch, or date range. "+
			"Returns matching sessions with ID, summary, repository, branch, timestamps, and counts.",
		func(params SearchSessionsParams, _ sdk.ToolInvocation) (string, error) {
			filter := data.FilterOptions{
				Query:      params.Query,
				Repository: params.Repo,
				Branch:     params.Branch,
				DeepSearch: params.Query != "",
			}
			if params.Since != "" {
				t, err := parseFlexibleDate(params.Since, "since")
				if err != nil {
					return "", err
				}
				filter.Since = t
			}
			if params.Until != "" {
				t, err := parseFlexibleDate(params.Until, "until")
				if err != nil {
					return "", err
				}
				filter.Until = t
			}
			sort := data.SortOptions{Field: data.SortByUpdated, Order: data.Descending}
			sessions, err := store.ListSessions(filter, sort, toolResultLimit)
			if err != nil {
				return "", fmt.Errorf("searching sessions: %w", err)
			}
			if len(sessions) == 0 {
				return "No sessions found matching the criteria.", nil
			}
			raw, err := marshalJSON(sessions)
			if err != nil {
				return "", err
			}
			return SanitizeExternalContent(raw), nil
		},
	)
}

func defineGetSessionDetailTool(store *data.Store) sdk.Tool {
	return sdk.DefineTool(
		"get_session_detail",
		"Get full details for a specific session including all turns, checkpoints, files, and refs.",
		func(params GetSessionDetailParams, _ sdk.ToolInvocation) (string, error) {
			if params.ID == "" {
				return "", errors.New("session ID is required")
			}
			detail, err := store.GetSession(params.ID)
			if err != nil {
				return "", fmt.Errorf("loading session %s: %w", params.ID, err)
			}
			raw, err := marshalJSON(detail)
			if err != nil {
				return "", err
			}
			return SanitizeExternalContent(raw), nil
		},
	)
}

func defineListRepositoriesTool(store *data.Store) sdk.Tool {
	return sdk.DefineTool(
		"list_repositories",
		"List all distinct repository names across all sessions.",
		func(params ListReposParams, _ sdk.ToolInvocation) (string, error) {
			repos, err := store.ListRepositories()
			if err != nil {
				return "", fmt.Errorf("listing repositories: %w", err)
			}
			if len(repos) == 0 {
				return "No repositories found.", nil
			}
			// Apply optional filter.
			if params.Filter != "" {
				f := strings.ToLower(params.Filter)
				var filtered []string
				for _, r := range repos {
					if strings.Contains(strings.ToLower(r), f) {
						filtered = append(filtered, r)
					}
				}
				repos = filtered
			}
			if len(repos) == 0 {
				return "No repositories match the filter.", nil
			}
			return SanitizeExternalContent(strings.Join(repos, "\n")), nil
		},
	)
}

func defineSearchDeepTool(store *data.Store) sdk.Tool {
	return sdk.DefineTool(
		"search_deep",
		"Perform a deep full-text search across all session content including turns, "+
			"checkpoints, files, and refs. Returns matching content snippets with session IDs.",
		func(params SearchDeepParams, _ sdk.ToolInvocation) (string, error) {
			if params.Query == "" {
				return "", errors.New("query is required")
			}
			results, err := store.SearchSessions(params.Query, toolResultLimit)
			if err != nil {
				return "", fmt.Errorf("deep search: %w", err)
			}
			if len(results) == 0 {
				return "No results found.", nil
			}
			raw, err := marshalJSON(results)
			if err != nil {
				return "", err
			}
			return SanitizeExternalContent(raw), nil
		},
	)
}

func defineAnalyzeCompletionTool(store *data.Store) sdk.Tool {
	return sdk.DefineTool(
		"analyze_completion",
		"Analyze whether planned work in a session has been completed by examining "+
			"the plan against session activity (turns, files changed, checkpoints). "+
			"Returns a JSON assessment with completion status, task counts, and remaining items.",
		func(params AnalyzeCompletionParams, _ sdk.ToolInvocation) (string, error) {
			if params.SessionID == "" {
				return "", errors.New("session_id is required")
			}
			detail, err := store.GetSession(params.SessionID)
			if err != nil {
				return "", fmt.Errorf("loading session %s: %w", params.SessionID, err)
			}

			// Collect unique file paths, capped to keep context manageable.
			seen := make(map[string]struct{}, len(detail.Files))
			files := make([]string, 0, min(len(detail.Files), analysisFilesLimit))
			for _, f := range detail.Files {
				if _, ok := seen[f.FilePath]; !ok {
					seen[f.FilePath] = struct{}{}
					files = append(files, f.FilePath)
					if len(files) >= analysisFilesLimit {
						break
					}
				}
			}

			// Build checkpoint summaries with only the fields relevant to
			// completion analysis.
			cps := make([]checkpointSummary, 0, len(detail.Checkpoints))
			for _, cp := range detail.Checkpoints {
				cps = append(cps, checkpointSummary{
					Title:     cp.Title,
					WorkDone:  cp.WorkDone,
					NextSteps: cp.NextSteps,
				})
			}

			actx := analysisContext{
				SessionID:    detail.Session.ID,
				Summary:      detail.Session.Summary,
				Repository:   detail.Session.Repository,
				Branch:       detail.Session.Branch,
				TurnCount:    detail.Session.TurnCount,
				FilesTouched: files,
				Checkpoints:  cps,
				PlanContent:  params.PlanContent,
			}

			raw, err := marshalJSON(actx)
			if err != nil {
				return "", err
			}
			return SanitizeExternalContent(raw), nil
		},
	)
}

// marshalJSON serialises v as indented JSON for tool results.
func marshalJSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling result: %w", err)
	}
	return string(b), nil
}
