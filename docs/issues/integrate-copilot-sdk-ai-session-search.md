# Integrate GitHub Copilot SDK for AI-Powered Session Search

## Summary

Add a conversational AI chat interface to Dispatch that lets users find sessions using natural language queries, powered by the GitHub Copilot SDK and the existing session SQLite database.

## Description

Dispatch currently supports two search modes: quick search (LIKE on session metadata) and deep search (LIKE across turns, checkpoints, files, and refs). Both are keyword-based and require users to know exact terms to match against.

Users often want to find sessions by intent or concept — "the session where I fixed the auth bug last week" or "when did I work on the database migration?" — which keyword search handles poorly.

By integrating the GitHub Copilot SDK, Dispatch can offer a chat-based interface where users describe what they're looking for in natural language. The SDK's AI model translates the user's intent into effective queries against the session store, synthesizes results, and presents relevant sessions conversationally.

### Why the Copilot SDK

- **Official GitHub integration** — aligns with Dispatch being a Copilot CLI companion tool
- **Handles query expansion** — AI understands synonyms, intent, and context (e.g., "auth bug" → searches for authentication, login, token, JWT)
- **Conversational refinement** — users can follow up ("no, the one from last Tuesday") without re-typing
- **Result synthesis** — AI can summarize what a session was about, not just return raw rows

## Technical Details

### Current Architecture

- **Language**: Go 1.26.1
- **TUI Framework**: Bubble Tea (charmbracelet/bubbletea)
- **Session DB**: SQLite at `~/.copilot/session-store.db` (read-only)
- **Driver**: `modernc.org/sqlite` v1.46.1
- **Search**: `internal/data/store.go` — `ListSessions()`, `SearchSessions()`, `GroupSessions()` with `FilterOptions`
- **Deep search**: Queries `turns.user_message`, `checkpoints.*`, `session_files.file_path`, `session_refs.ref_value`
- **FTS5**: `search_index` virtual table exists but `SearchSessions()` currently uses LIKE, not FTS5 MATCH

### Key Files

| File | Role |
|------|------|
| `internal/data/store.go` | Session query API (ListSessions, SearchSessions, GroupSessions) |
| `internal/data/models.go` | Session, Turn, Checkpoint, SessionFile, SessionRef structs |
| `internal/tui/model.go` | Main TUI model, state machine, deep search orchestration |
| `internal/tui/components/searchbar.go` | Search input component |
| `internal/tui/components/preview.go` | Session detail preview panel |
| `internal/tui/keys.go` | Key bindings |
| `cmd/dispatch/main.go` | Entry point |

### Database Schema (relevant tables)

```sql
sessions       (id, cwd, repository, branch, summary, created_at, updated_at)
turns          (session_id, turn_index, user_message, assistant_response, timestamp)
checkpoints    (session_id, checkpoint_number, title, overview, history, work_done,
                technical_details, important_files, next_steps)
session_files  (session_id, file_path, tool_name, turn_index, first_seen_at)
session_refs   (session_id, ref_type, ref_value, turn_index, created_at)
search_index   (FTS5: content, session_id, source_type, source_id)
```

### Proposed Integration Approach

1. **Add GitHub Copilot SDK dependency** — Go SDK from `github.com/copilot/...` (or the appropriate official package)
2. **New chat view in TUI** — a Bubble Tea component that renders a chat conversation (user messages + AI responses), accessible via a keybinding (e.g., `?` or `/ai`)
3. **Session context provider** — a function that, given the AI's generated query parameters, executes them against `store.go` methods and returns structured results for the AI to reason about
4. **Tool/function calling** — expose the session DB query capabilities as tools the Copilot SDK model can call:
   - `search_sessions(query, since, until, repo, branch)` — maps to `ListSessions` with `FilterOptions`
   - `get_session_detail(id)` — maps to `GetSession`
   - `list_repositories()` — maps to `ListRepositories`
   - `search_deep(query)` — maps to deep search with FTS5
5. **Result rendering** — AI responses rendered in the preview panel or a dedicated chat panel, with session links the user can select to navigate to
6. **Authentication** — use GitHub token from environment (`GITHUB_TOKEN` or `gh auth token`) for Copilot SDK auth

### UX Considerations

- Chat should feel snappy — stream responses if the SDK supports it
- Allow navigating from AI-suggested sessions directly to the session detail view
- Keep the existing keyword search as-is — AI chat is additive, not a replacement
- Consider a split view: chat panel on one side, session list on the other
- Conversation history within a Dispatch session (not persisted across restarts initially)

## Acceptance Criteria

- [ ] GitHub Copilot SDK integrated as a Go dependency
- [ ] New TUI view/panel for AI chat accessible via keybinding
- [ ] AI can search sessions by natural language query (translates intent to DB queries)
- [ ] AI can answer follow-up questions with conversational context
- [ ] Session results from AI are navigable (user can jump to a session)
- [ ] Authentication uses existing GitHub token (no separate login)
- [ ] Existing keyword search remains fully functional
- [ ] Graceful degradation if Copilot SDK is unavailable (show error, fall back to keyword search)

## Related

- Current search implementation: `internal/data/store.go` (`SearchSessions`, `ListSessions`)
- Deep search orchestration: `internal/tui/model.go` (deepSearchVersion pattern)
- FTS5 search_index table exists but is underutilized — could be leveraged by AI tool calls
- GitHub Copilot SDK documentation: https://github.com/features/copilot
