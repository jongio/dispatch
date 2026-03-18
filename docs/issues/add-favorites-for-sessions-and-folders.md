# Add Favorites for Sessions and Folders

## Summary

Add a favorites system that lets users star individual sessions and folder paths so they can quickly access frequently-used items. Favorited items display a ★ indicator and can be filtered with `F`. All group-header favorites resolve to folder paths as the canonical storage type.

## Description

Power users accumulate hundreds of Copilot sessions across many repositories and branches. Currently there's no way to pin or bookmark the items you care about most — you have to search, scroll, or remember pivot/filter combos every time.

A favorites feature would let users mark sessions and folders as favorites with a single keypress (`*`). Favorites persist across restarts and surface visually with a star indicator (★). Users can toggle favorites on and off, and filter to show only favorites.

This mirrors the existing **hidden sessions** pattern (`h`/`H` keys, `HiddenSessions` config field, `hiddenSet` map) but inverts the intent — instead of hiding items, it highlights them.

### Why It Matters

- **Speed**: Jump to your most important sessions without searching
- **Context switching**: When working across multiple repos/branches, favorites act as a personal "pinned" workspace
- **Discoverability**: With hundreds of sessions, visual markers reduce cognitive load

## Technical Details

### Architecture: Follow the Hidden Sessions Pattern

The hidden sessions feature is the closest analog and should serve as the implementation blueprint:

| Aspect | Hidden Sessions (existing) | Favorites (proposed) |
|--------|---------------------------|---------------------|
| Config field | `HiddenSessions []string` | `FavoriteSessions []string`, `FavoriteFolders []string` |
| Runtime set | `hiddenSet map[string]struct{}` | `favoriteSessions map[string]struct{}`, `favoriteFolders map[string]struct{}` |
| Toggle key | `h` (hide) | `*` (star/favorite) |
| Show/filter key | `H` (toggle hidden visibility) | `F` (filter to favorites only) |
| Visual indicator | Dimmed styling | `★` prefix on item label |
| Persistence | Config JSON | Config JSON |

### Keyboard Bindings

**Currently used keys** (must NOT collide):

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | Navigate |
| `←`, `→` | Collapse/expand folder |
| `Enter` | Launch/toggle |
| `Space` | Multi-select |
| `q`, `Ctrl+C` | Quit |
| `/` | Search |
| `Esc` | Clear/back |
| `f` | Filter panel |
| `s` / `S` | Sort field / sort direction |
| `Tab` / `Shift+Tab` | Pivot / reverse pivot |
| `p` | Preview |
| `r` | Reindex |
| `?` | Help |
| `,` | Config/settings |
| `1`-`4` | Time range filters |
| `h` / `H` | Hide / toggle hidden |
| `w`, `t`, `e` | Launch window/tab/pane |
| `PgUp` / `PgDn` | Preview scroll |
| `n` | Jump next attention |
| `!` | Filter attention |
| `O` | Open all selected |
| `a` / `d` | Select all / deselect all |
| `o` | Conversation sort |

**Proposed new bindings:**

| Key | Action | Rationale |
|-----|--------|-----------|
| `*` | Toggle favorite on selected item | Universal "star" metaphor; unused; doesn't conflict with any existing binding |
| `F` (Shift+F) | Toggle "favorites only" filter | Parallels `H` for hidden; uppercase `F` is free (`f` is filter panel) |

**Why `*`:** The asterisk is universally associated with starring/favoriting (Gmail, GitHub, file managers). It's a single keypress (Shift+8 on US layout), mnemonic, and not used by any existing binding. Alternative candidates (`+`, `#`) are less intuitive.

### Data Model Changes

#### `internal/config/config.go`

Add two new fields to the `Config` struct:

```go
type Config struct {
    // ... existing fields ...
    FavoriteSessions []string `json:"favorite_sessions,omitempty"`
    FavoriteFolders  []string `json:"favorite_folders,omitempty"`
}
```

- **FavoriteSessions**: Session IDs (same format as `HiddenSessions`)
- **FavoriteFolders**: Folder/cwd paths — the canonical storage type for all group-level favorites

These are stored in the same `config.json` file and follow the same sanitization rules.

#### `internal/tui/components/sessionlist.go`

Add favorite tracking sets parallel to `hiddenSet`:

```go
type SessionList struct {
    // ... existing fields ...
    favoriteSessions map[string]struct{}
    favoriteFolders  map[string]struct{}
    favoritesOnly    bool // filter mode: show only favorites
}
```

### Key Behaviors

#### Toggle Favorite (`*` key)

When pressed on a **session item** (any pivot mode):
1. If session ID is in `favoriteSessions` → remove it (unfavorite)
2. If session ID is NOT in `favoriteSessions` → add it (favorite)
3. Save config immediately (same pattern as `handleHideSession()`)
4. Rebuild visible items to update visual indicator

When pressed on a **folder group header** (folder pivot):
1. Use the folder path directly as the key
2. Toggle in `favoriteFolders` set
3. Save and rebuild

When pressed on a **repo group header** (repo pivot):
1. Collect all unique folder paths (cwds) from sessions within that repo group
2. If **one folder** → toggle that folder path directly in `favoriteFolders`
3. If **multiple folders** → show a sub-selection picker listing the folder paths; user picks which to favorite/unfavorite
4. Save and rebuild

When pressed on a **branch group header** (branch pivot):
1. Same resolution as repo: collect unique folder paths from sessions in that branch group
2. If **one folder** → toggle directly
3. If **multiple folders** → show sub-selection picker
4. Save and rebuild

When pressed on a **date group header** (date pivot):
→ No-op. Favoriting a date is not supported.

#### Filter Favorites (`F` key)

Toggle `favoritesOnly` mode:
- **ON**: Only show items where the session is individually favorited OR the session's cwd matches a favorited folder
- **OFF**: Show all items (default), favorites still visually marked
- Visual indicator in the status bar when favorites filter is active (e.g., "★ Favorites")

#### Visual Rendering

- Favorited sessions: Prepend `★ ` to the session title in the list
- Favorited folder headers: Prepend `★ ` to the folder label
- Sessions whose cwd matches a favorited folder: Show `★` even if not individually favorited (inherited from folder)
- Use a distinct color for the star (e.g., yellow/gold from the theme's accent palette)
- In repo/branch pivot modes: group headers show `★` if ALL sessions in the group belong to favorited folders

#### Sort Integration

- When sorting, favorited items should optionally sort to the top (a boolean "favorites first" toggle, or simply respected when `F` filter is active)
- Consider adding `favorites` as a sort field option (cycle with `s` key)

### Files to Modify

1. **`internal/config/config.go`** — Add `FavoriteSessions`, `FavoriteFolders` fields
2. **`internal/tui/keys.go`** — Add `ToggleFavorite` (`*`) and `FilterFavorites` (`F`) key bindings
3. **`internal/tui/components/sessionlist.go`** — Add favorite sets, filter logic, visual rendering, sub-selection picker for multi-folder resolution
4. **`internal/tui/model.go`** — Add `handleToggleFavorite()` and `handleFilterFavorites()` in key handler, load favorites from config on init
5. **`internal/tui/styles.go`** — Add star style (color for `★` indicator)
6. **`docs/KEYBOARD_BINDINGS.md`** — Document new keybindings (if this file exists)

### Edge Cases

- **Favoriting a session that gets deleted**: On next load, stale IDs in config are harmless (session just won't appear). Optionally prune stale favorites on reindex.
- **Favoriting a folder path that changes**: Folder favorites are by exact path string. If a repo moves, the old favorite becomes inert. Acceptable tradeoff.
- **Favoriting + hiding**: A session can be both favorited and hidden. Hidden takes precedence for visibility (unless `H` is toggled to show hidden). The star still renders on hidden-but-visible items.
- **Multi-folder sub-selection picker**: When `*` is pressed on a repo/branch group header with multiple cwds, a small overlay lists the folders. User selects with arrow keys + Enter/Space. Pressing Esc cancels without changes.
- **Empty favorites filter**: If `F` is toggled but no favorites exist, show an empty state message: "No favorites yet. Press * to favorite an item."
- **Config migration**: Old configs without favorite fields deserialize cleanly due to `omitempty` — slices default to nil/empty.
- **Date pivot**: `*` is a no-op on date group headers — no visual affordance or error, just ignored.

## Acceptance Criteria

- [ ] `*` key toggles favorite on the currently selected session (any pivot mode)
- [ ] `*` key on a folder group header toggles favorite on that folder path
- [ ] `*` key on a repo/branch group header resolves to folder path(s); if multiple, shows a sub-selection picker
- [ ] `*` key on a date group header is a no-op
- [ ] `F` key toggles a "favorites only" filter showing individually-favorited sessions and sessions whose cwd matches a favorited folder
- [ ] Favorited items display a `★` visual indicator in the list
- [ ] Sessions inherit `★` from favorited parent folder (without being individually favorited)
- [ ] Favorites persist across application restarts via `config.json`
- [ ] New key bindings (`*`, `F`) do not collide with any existing bindings
- [ ] Favoriting + hiding coexist without conflict (hidden takes visibility precedence)
- [ ] Help overlay (`?`) documents the new `*` and `F` keybindings
- [ ] Status bar indicates when favorites filter is active
- [ ] Empty favorites filter shows a helpful message

## Related

- **Existing pattern**: Hidden sessions (`h`/`H` keys, `HiddenSessions` config, `hiddenSet` map) — direct blueprint
- **Files**: `internal/config/config.go`, `internal/tui/keys.go`, `internal/tui/model.go`, `internal/tui/components/sessionlist.go`
- **Config location**: `%APPDATA%\dispatch\config.json` (Windows), `~/.config/dispatch/config.json` (Unix)
- **Storage types**: Only two — `FavoriteSessions` (session IDs) and `FavoriteFolders` (cwd paths). All group-header favorites resolve to folder paths.
