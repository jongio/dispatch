package copilot

import "errors"

// Sentinel errors returned by copilot operations.
var (
	ErrInvalidDate         = errors.New("invalid date")
	ErrSessionIDRequired   = errors.New("session ID is required")
	ErrQueryRequired       = errors.New("query is required")
	ErrSessionNotAvailable = errors.New("copilot session not available")
	ErrSearchUnavailable   = errors.New("search unavailable")
	ErrAnalyzeUnavailable  = errors.New("analyze_completion unavailable")
	ErrSearchingSessions   = errors.New("searching sessions")
	ErrLoadingSession      = errors.New("loading session")
	ErrListingRepos        = errors.New("listing repositories")
	ErrDeepSearch          = errors.New("deep search")
	ErrStartingSDK         = errors.New("starting Copilot SDK")
)
