# COMPREHENSIVE KEYBOARD AND MOUSE BINDING DOCUMENTATION
# Dispatch TUI (Bubble Tea v2 Go Application)
# Directory: D:\code\dispatch\internal\tui

## KEYBOARD SHORTCUTS - GLOBAL (Always Available)

### Force Quit (Works in All States)
1. **Ctrl+C** → Force Quit
   - File: D:\code\dispatch\internal\tui\keys.go (line 65)
   - Code: key.NewBinding(key.WithKeys("ctrl+c"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 629-631)
   - Behavior: Closes store and exits application immediately
   - Condition: Works in ALL states, including overlays

## MAIN SESSION LIST VIEW - NAVIGATION AND DISPLAY

### Navigation Keys
2. **k** or **↑ (Up Arrow)** → Move Up in List
   - File: D:\code\dispatch\internal\tui\keys.go (line 58)
   - Code: key.NewBinding(key.WithKeys("up", "k"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 831-834)
   - Behavior: Moves selection up, loads detail for selected item
   - Condition: Only in session list view (stateSessionList)

3. **j** or **↓ (Down Arrow)** → Move Down in List
   - File: D:\code\dispatch\internal\tui\keys.go (line 59)
   - Code: key.NewBinding(key.WithKeys("down", "j"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 836-839)
   - Behavior: Moves selection down, loads detail for selected item
   - Condition: Only in session list view

4. **← (Left Arrow)** → Collapse Folder (if selected item is a folder)
   - File: D:\code\dispatch\internal\tui\keys.go (line 60)
   - Code: key.NewBinding(key.WithKeys("left"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 866-870)
   - Behavior: Collapses expanded folder tree item
   - Condition: Only works if a folder is currently selected

5. **→ (Right Arrow)** → Expand Folder (if selected item is a folder)
   - File: D:\code\dispatch\internal\tui\keys.go (line 61)
   - Code: key.NewBinding(key.WithKeys("right"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 872-876)
   - Behavior: Expands folder to show child items
   - Condition: Only works if a folder is currently selected

### Launch/Interaction Keys
6. **Enter** → Launch Session or Toggle Folder
   - File: D:\code\dispatch\internal\tui\keys.go (line 62)
   - Code: key.NewBinding(key.WithKeys("enter"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 841-846)
   - Behavior: Launches selected session OR toggles folder expansion
   - Condition: Works for both sessions and folders

7. **w** → Open in Window
   - File: D:\code\dispatch\internal\tui\keys.go (line 82)
   - Code: key.NewBinding(key.WithKeys("w"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 848-852)
   - Behavior: Forces launch in a new window
   - Condition: Only when a session (not folder) is selected

8. **t** → Open in Tab
   - File: D:\code\dispatch\internal\tui\keys.go (line 83)
   - Code: key.NewBinding(key.WithKeys("t"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 854-858)
   - Behavior: Forces launch in a new tab
   - Condition: Only when a session (not folder) is selected

9. **e** → Open in Pane
   - File: D:\code\dispatch\internal\tui\keys.go (line 84)
   - Code: key.NewBinding(key.WithKeys("e"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 860-864)
   - Behavior: Forces launch in a split pane of the current tab (Windows Terminal only)
   - Condition: Only when a session (not folder) is selected

### Multi-Select Keys
10. **Space** → Toggle Selection on Current Session
    - File: D:\code\dispatch\internal\tui\keys.go (multi-select handler)
    - Code: key.NewBinding(key.WithKeys(" "))
    - Handler: D:\code\dispatch\internal\tui\model.go (multi-select handler)
    - Behavior: Toggles the ✓ selection indicator on the currently highlighted session. Does not open it.
    - Condition: Only when a session (not folder) is selected in session list view

11. **L** → Launch All Selected Sessions (or All in Folder)
    - File: D:\code\dispatch\internal\tui\keys.go (multi-select handler)
    - Code: key.NewBinding(key.WithKeys("L"))
    - Handler: D:\code\dispatch\internal\tui\model.go (multi-select handler)
    - Behavior: Launches every session that has a ✓ selection indicator. If no sessions are selected and cursor is on a folder, opens all sessions under that folder. Each session opens via the configured launch mode.
    - Condition: In session list view; requires at least one selected session OR cursor on a folder

12. **a** → Select All Visible Sessions
    - File: D:\code\dispatch\internal\tui\keys.go (multi-select handler)
    - Code: key.NewBinding(key.WithKeys("a"))
    - Handler: D:\code\dispatch\internal\tui\model.go (multi-select handler)
    - Behavior: Marks every visible session (respecting current search/filter) with the ✓ selection indicator. Does not select folders.
    - Condition: In session list view

13. **d** → Deselect All
    - File: D:\code\dispatch\internal\tui\keys.go (multi-select handler)
    - Code: key.NewBinding(key.WithKeys("d"))
    - Handler: D:\code\dispatch\internal\tui\model.go (multi-select handler)
    - Behavior: Clears all ✓ selection indicators, returning to normal single-cursor mode.
    - Condition: In session list view

### Search and Filtering
14. **/** → Focus Search Bar / Open Search
   - File: D:\code\dispatch\internal\tui\keys.go (line 66)
   - Code: key.NewBinding(key.WithKeys("/"))
   - Handler: D:\code\dispatch\internal\tui\model.go (lines 823-825)
   - Behavior: Focuses search bar for typing queries
   - Condition: In session list view

15. **Esc** → Clear Search Query or Back to List
    - File: D:\code\dispatch\internal\tui\keys.go (line 67)
    - Code: key.NewBinding(key.WithKeys("esc"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 788-802)
    - Behavior: 
      - If search query active: clears query and reloads list
      - If no query: returns to list (used to exit overlays)
    - Condition: In session list view or after exiting search bar

16. **f** → Open Filter Panel
    - File: D:\code\dispatch\internal\tui\keys.go (line 68)
    - Code: key.NewBinding(key.WithKeys("f"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 827-829)
    - Behavior: Opens directory filter overlay panel
    - Condition: In session list view

### Sorting and Organization
17. **s** → Cycle Sort Order
    - File: D:\code\dispatch\internal\tui\keys.go (line 69)
    - Code: key.NewBinding(key.WithKeys("s"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 878-880)
    - Behavior: Cycles through sort options (Name, Created, Updated, etc.)
    - Condition: In session list view

18. **S** (Shift+S) → Toggle Sort Direction
    - File: D:\code\dispatch\internal\tui\keys.go (line 70)
    - Code: key.NewBinding(key.WithKeys("S"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 882-884)
    - Behavior: Toggles between ascending and descending sort
    - Condition: In session list view

19. **Tab** → Cycle Pivot Mode
    - File: D:\code\dispatch\internal\tui\keys.go (line 71)
    - Code: key.NewBinding(key.WithKeys("tab"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 886-888)
    - Behavior: Cycles grouping: none → folder → repo → branch → date → none
    - Condition: In session list view

### Preview and Display
20. **p** → Toggle Preview Panel
    - File: D:\code\dispatch\internal\tui\keys.go (line 72)
    - Code: key.NewBinding(key.WithKeys("p"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 890-897)
    - Behavior: Shows/hides detailed session info preview on the right
    - Condition: In session list view

20b. **o** → Toggle Conversation Sort Order
     - File: D:\code\dispatch\internal\tui\keys.go (line 101)
     - Code: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "conversation order"))
     - Handler: D:\code\dispatch\internal\tui\model.go (lines 1009-1011)
     - Behavior: Toggles conversation display order between oldest-first and newest-first in the preview pane. Also clickable via the sort arrow in the conversation header. Persisted in config.
     - Condition: Only when preview panel is visible (showPreview=true)

20c. **v** → View Plan in Preview Pane
     - File: D:\code\dispatch\internal\tui\keys.go
     - Code: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view plan"))
     - Handler: D:\code\dispatch\internal\tui\model.go
     - Behavior: Renders the session's plan.md content in the preview pane
     - Condition: Only when a session with a plan.md file is selected

20d. **P** (Shift+P) → Cycle Preview Position
     - File: D:\code\dispatch\internal\tui\keys.go
     - Code: key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "cycle preview position"))
     - Handler: D:\code\dispatch\internal\tui\model.go (cyclePreviewPosition)
     - Behavior: Cycles preview pane position: right → bottom → left → top → right. Persisted in config.
     - Condition: In session list view

21. **PgUp (Page Up)** → Preview Panel Scroll Up
    - File: D:\code\dispatch\internal\tui\keys.go (line 85)
    - Code: key.NewBinding(key.WithKeys("pgup"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 899-903)
    - Behavior: Scrolls preview panel content up by one page
    - Condition: Only when preview panel is visible (showPreview=true)

22. **PgDn (Page Down)** → Preview Panel Scroll Down
    - File: D:\code\dispatch\internal\tui\keys.go (line 86)
    - Code: key.NewBinding(key.WithKeys("pgdown"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 905-909)
    - Behavior: Scrolls preview panel content down by one page
    - Condition: Only when preview panel is visible

### Session Management
23. **r** → Reindex Sessions
    - File: D:\code\dispatch\internal\tui\keys.go (line 73)
    - Code: key.NewBinding(key.WithKeys("r"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 911-922)
    - Behavior: Launches Copilot CLI in a pseudo-terminal and runs /chronicle reindex for a full ETL rebuild. Falls back to FTS5 maintenance if the binary is not found. Shows a streaming log overlay during the operation.
    - Condition: In session list view; ignored if already reindexing

24. **h** → Hide Current Session
    - File: D:\code\dispatch\internal\tui\keys.go (line 80)
    - Code: key.NewBinding(key.WithKeys("h"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 937-938)
    - Behavior: Hides the currently selected session (persisted to config)
    - Condition: Only when a session is selected (not a folder)

25. **H** (Shift+H) → Toggle Hidden Sessions Visibility
    - File: D:\code\dispatch\internal\tui\keys.go (line 81)
    - Code: key.NewBinding(key.WithKeys("H"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 940-943)
    - Behavior: Shows/hides all sessions marked as hidden
    - Condition: In session list view

26. **\*** → Toggle Favorite on Current Session
    - File: D:\code\dispatch\internal\tui\keys.go
    - Code: key.NewBinding(key.WithKeys("*"))
    - Handler: D:\code\dispatch\internal\tui\model.go
    - Behavior: Stars or unstars the currently selected session (persisted to config as favoriteSessions)
    - Condition: Only when a session is selected (not a folder)

27. **\*** → Toggle Favorite
    - File: D:\code\dispatch\internal\tui\keys.go
    - Code: key.NewBinding(key.WithKeys("*"))
    - Handler: D:\code\dispatch\internal\tui\model.go
    - Behavior: Stars or unstars the currently selected session as a favorite
    - Condition: Only when a session is selected (not a folder)

    Note: Favorites filter was previously on `F` — now toggled via the `!` status picker "Favorites only" row.

27b. **c** → Copy Session ID to Clipboard
     - File: D:\code\dispatch\internal\tui\keys.go
     - Code: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy session ID"))
     - Handler: D:\code\dispatch\internal\tui\model.go (handleCopyID)
     - Behavior: Copies the selected session's ID to the system clipboard. Shows "Copied session ID ✓" status on success. Also triggered by clicking the ID row in the preview pane.
     - Condition: Only when a session is selected (not a folder)

### Attention & Session Status
28. **n** → Jump to Next Waiting Session
    - File: D:\code\dispatch\internal\tui\keys.go
    - Code: key.NewBinding(key.WithKeys("n"))
    - Handler: D:\code\dispatch\internal\tui\model.go
    - Behavior: Cycles to the next session with AttentionWaiting status
    - Condition: In session list view

29. **N** (Shift+N) → Resume All Interrupted Sessions
    - File: D:\code\dispatch\internal\tui\keys.go
    - Code: key.NewBinding(key.WithKeys("N"))
    - Handler: D:\code\dispatch\internal\tui\model.go (handleResumeInterrupted)
    - Behavior: Batch-resumes all sessions with AttentionInterrupted status via `ghcs --resume`
    - Condition: In session list view; requires at least one interrupted session

30. **!** → Filter by Attention Status
    - File: D:\code\dispatch\internal\tui\keys.go
    - Code: key.NewBinding(key.WithKeys("!"))
    - Handler: D:\code\dispatch\internal\tui\model.go
    - Behavior: Opens the attention picker overlay to filter sessions by one or more attention states (waiting, active, stale, interrupted, idle), a "Has plan" row, a "Favorites only" row, and work status rows
    - Condition: In session list view

**Attention Status Indicators:**
- ● Waiting (purple) — session needs user input (`assistant.turn_end` or `assistant.message`)
- ● Active (green) — AI is working (live PID + `assistant.turn_start` or `tool_execution`)
- ● Stale (yellow) — running but quiet (live PID, no recent events)
- ⚡ Interrupted (orange) — killed mid-work by crash/reboot (stale lock file + active event)
- ○ Idle (gray) — not running
- Has plan — sessions with a `plan.md` file
- Favorites only — sessions starred as favorites

30b. **R** (Shift+R) → Scan Work Status
    - File: D:\code\dispatch\internal\tui\keys.go
    - Code: key.NewBinding(key.WithKeys("R"))
    - Handler: D:\code\dispatch\internal\tui\model.go
    - Behavior: Scans all sessions with plans for work completion status (quick classification → full parse → optional AI analysis)
    - Condition: In session list view; does not run on startup — must be explicitly triggered

30c. Work status and plan filtering is also available via the **!** status picker (see 30 above). The picker includes "Has plan", "Favorites only", "Incomplete work", and "Complete work" rows below the attention statuses.

### Time Range Filter
31. **1** → Set Time Range to 1 Hour
    - File: D:\code\dispatch\internal\tui\keys.go (line 76)
    - Code: key.NewBinding(key.WithKeys("1"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 924-926)
    - Behavior: Filters sessions to last 1 hour
    - Condition: In session list view

29. **2** → Set Time Range to 1 Day
    - File: D:\code\dispatch\internal\tui\keys.go (line 77)
    - Code: key.NewBinding(key.WithKeys("2"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 927-929)
    - Behavior: Filters sessions to last 1 day
    - Condition: In session list view

30. **3** → Set Time Range to 7 Days
    - File: D:\code\dispatch\internal\tui\keys.go (line 78)
    - Code: key.NewBinding(key.WithKeys("3"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 930-932)
    - Behavior: Filters sessions to last 7 days
    - Condition: In session list view

31. **4** → Set Time Range to All Time
    - File: D:\code\dispatch\internal\tui\keys.go (line 79)
    - Code: key.NewBinding(key.WithKeys("4"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 933-935)
    - Behavior: Shows all sessions (removes time filter)
    - Condition: In session list view

### Info and Settings
32. **?** → Toggle Help Overlay
    - File: D:\code\dispatch\internal\tui\keys.go (line 74)
    - Code: key.NewBinding(key.WithKeys("?"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 804-806)
    - Behavior: Opens/closes comprehensive help modal
    - Condition: Can open from session list; closed by ? or Esc

33. **,** → Open Settings/Config Panel
    - File: D:\code\dispatch\internal\tui\keys.go (line 75)
    - Code: key.NewBinding(key.WithKeys(","))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 808-821)
    - Behavior: Opens configuration panel to modify settings
    - Condition: In session list view

34. **q** → Quit (Graceful)
    - File: D:\code\dispatch\internal\tui\keys.go (line 64)
    - Code: key.NewBinding(key.WithKeys("q"))
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 784-786)
    - Behavior: Closes store and exits application gracefully
    - Condition: In session list view

## SEARCH BAR - FOCUSED STATE

When the search bar is focused (after pressing /):

35. **Up Arrow / k** → Blur Search and Move Selection Up
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 740-744)
    - Behavior: Unfocuses search bar and moves list selection up
    - Condition: Only when search bar is focused

36. **Down Arrow / j** → Blur Search and Move Selection Down
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 745-749)
    - Behavior: Unfocuses search bar and moves list selection down
    - Condition: Only when search bar is focused

37. **Esc** → Blur Search Bar
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 706-725)
    - Behavior: Unfocuses search bar; query stays active if non-empty
    - Condition: Only when search bar is focused

38. **Enter** → Confirm Search
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 726-739)
    - Behavior: Triggers deep search if pending; unfocuses bar
    - Condition: Only when search bar is focused

39. **Any Printable Character** (a-z, A-Z, 0-9, spaces, etc.)
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 750-779)
    - Behavior: Adds character to query; triggers quick search immediately and deep search after delay
    - Condition: Only when search bar is focused

## FILTER PANEL - OVERLAY

When filter panel is open (after pressing f):

40. **↑ (Up Arrow)** → Move Up in Filter List
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 673-674)
    - Behavior: Moves selection up in directory filter tree
    - Condition: Only in stateFilterPanel

41. **↓ (Down Arrow)** → Move Down in Filter List
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 675-676)
    - Behavior: Moves selection down in directory filter tree
    - Condition: Only in stateFilterPanel

42. **← (Left Arrow)** → Collapse Filter Group
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 677-678)
    - Behavior: Collapses expanded directory group
    - Condition: Only in stateFilterPanel

43. **→ (Right Arrow)** → Expand Filter Group
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 679-680)
    - Behavior: Expands directory group to show subdirectories
    - Condition: Only in stateFilterPanel

44. **Space** → Toggle Filter Exclusion
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 681-682)
    - Behavior: Toggles whether selected directory is excluded from results
    - Condition: Only in stateFilterPanel

45. **Enter** → Apply Filters and Close Panel
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 683-692)
    - Behavior: Saves exclusion settings to config and reloads session list
    - Condition: Only in stateFilterPanel

46. **Esc** → Cancel and Close Filter Panel
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 670-672)
    - Behavior: Discards changes and returns to session list
    - Condition: Only in stateFilterPanel

## SHELL PICKER - OVERLAY

When shell picker is open (shown after selecting launch mode):

47. **↑ (Up Arrow)** → Move Up in Shell List
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 655-656)
    - Behavior: Moves selection up in available shells list
    - Condition: Only in stateShellPicker

48. **↓ (Down Arrow)** → Move Down in Shell List
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 657-658)
    - Behavior: Moves selection down in available shells list
    - Condition: Only in stateShellPicker

49. **Enter** → Select Shell and Launch
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 659-665)
    - Behavior: Launches session with selected shell
    - Condition: Only in stateShellPicker

50. **Esc** → Cancel and Return to List
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 653-654)
    - Behavior: Closes shell picker without launching
    - Condition: Only in stateShellPicker

## CONFIG PANEL - OVERLAY

When config panel is open (after pressing ,):

### Non-Edit Mode
51. **↑ (Up Arrow)** → Move Up in Config Options
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1016-1017)
    - Behavior: Moves selection up through config options
    - Condition: When NOT in edit mode within config panel

52. **↓ (Down Arrow)** → Move Down in Config Options
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1018-1019)
    - Behavior: Moves selection down through config options
    - Condition: When NOT in edit mode

53. **Enter** → Select/Edit Config Option
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1020-1022)
    - Behavior: Enters edit mode for selected option
    - Condition: When NOT in edit mode

54. **Esc** → Save and Close Config Panel
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1011-1015)
    - Behavior: Saves all config changes and returns to session list
    - Condition: When NOT in edit mode

### Edit Mode (Inside Text Field)
55. **Esc** → Cancel Edit of Current Field
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 997-998)
    - Behavior: Discards changes to current field, returns to option selection
    - Condition: When in edit mode for a field

56. **Enter** → Confirm Edit of Current Field
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1000-1001)
    - Behavior: Accepts changes to field and returns to option selection
    - Condition: When in edit mode for a field

57. **Any Printable Character** (a-z, A-Z, 0-9, spaces, etc.)
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1003-1006)
    - Behavior: Types character into text field (delegated to textinput)
    - Condition: When in edit mode for a text field

## HELP OVERLAY - MODAL

When help overlay is open (after pressing ?):

58. **?** → Toggle Help (Close)
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 646-649)
    - Behavior: Closes help overlay and returns to session list
    - Condition: In stateHelpOverlay

59. **Esc** → Close Help
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 646-649)
    - Behavior: Closes help overlay and returns to session list
    - Condition: In stateHelpOverlay

## MOUSE INTERACTIONS

### Left Mouse Button (Click)
60. **Single Click on Session** → Select Session
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1096-1163)
    - Behavior: Moves selection to clicked item; deferred timer allows double-click detection
    - Timing: Single click fires after 300ms (doubleClickTimeout constant at line 40)
    - Condition: Only in stateSessionList, within list area (not preview pane)

61. **Double Click on Session** → Launch Session
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1122-1144)
    - Behavior: Launches selected session with default or override mode
    - Ctrl Modifier: Forces window launch (config.LaunchModeWindow)
    - Shift Modifier: Forces tab launch (config.LaunchModeTab)
    - Condition: Only in stateSessionList, within list area

62. **Double Click on Folder + Ctrl** → Launch New Session in Window
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1127-1135)
    - Behavior: Creates new session in folder's path, opens in new window
    - Condition: Double-click on folder item + Ctrl modifier pressed

63. **Double Click on Folder + Shift** → Launch New Session in Tab
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1127-1135)
    - Behavior: Creates new session in folder's path, opens in new tab
    - Condition: Double-click on folder item + Shift modifier pressed

64. **Double Click on Folder** → Launch New Session with Default Mode
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1127-1135)
    - Behavior: Creates new session in folder's path
    - Condition: Double-click on folder item (no modifiers)

65. **Ctrl + Click on Session** → Toggle Selection Without Opening
    - Handler: D:\code\dispatch\internal\tui\model.go (multi-select handler)
    - Behavior: Toggles the ✓ selection indicator on the clicked session without changing the primary cursor. Allows building a selection set with the mouse.
    - Condition: Only in stateSessionList, clicking on a session item (not folder)

66. **Shift + Click on Session** → Range Select
    - Handler: D:\code\dispatch\internal\tui\model.go (multi-select handler)
    - Behavior: Selects all sessions between the last-clicked session and the current click target (inclusive). Folders in the range are skipped.
    - Condition: Only in stateSessionList, requires a prior click anchor point

67. **Double-Click (with selections active)** → Open All Selected Sessions
    - Handler: D:\code\dispatch\internal\tui\model.go (multi-select handler)
    - Behavior: When one or more sessions have the ✓ indicator, double-clicking any session opens all selected sessions instead of just the double-clicked one. Each opens via the configured launch mode.
    - Condition: Only in stateSessionList, when selectedSessions set is non-empty

68. **Click on Header Area (Search Bar)** → Focus Search
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1101-1103, 1168-1230)
    - Behavior: Focuses search bar for typing; click position determines if on search area
    - Condition: Click on Y=0 (title line), X >= title width

### Header Badge Clicks
69. **Click Time Range Badge** → Set Time Range
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1170-1198, 1216-1230)
    - Behavior: Sets time range filter to clicked option (1h, 1d, 7d, all)
    - Condition: Click on Y=1 (badge line), within time range segment

70. **Click Sort Indicator** → Cycle Sort Order
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1170-1198)
    - Behavior: Cycles sort field to next option
    - Condition: Click on Y=1, within sort indicator area

71. **Click Pivot Indicator** → Cycle Pivot Mode
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1170-1198)
    - Behavior: Cycles pivot grouping mode
    - Condition: Click on Y=1, within pivot area

### Scroll Wheel (Mouse)

### Preview Pane Clicks
71b. **Click on Session ID Row** → Copy Session ID to Clipboard
     - Handler: D:\code\dispatch\internal\tui\model.go (handleMouse → HitSessionID)
     - Behavior: Copies the session ID to the system clipboard and shows "Copied session ID ✓" status
     - Condition: Preview pane visible, click lands on the "ID: ..." row

72. **Mouse Wheel Up (List Area)** → Scroll List Up
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1076-1084)
    - Behavior: Scrolls session list up by 3 items
    - Condition: Only in stateSessionList, when mouse is over list (not preview)

73. **Mouse Wheel Down (List Area)** → Scroll List Down
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1086-1094)
    - Behavior: Scrolls session list down by 3 items
    - Condition: Only in stateSessionList, when mouse is over list

74. **Mouse Wheel Up (Preview Area)** → Scroll Preview Up
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1076-1084)
    - Behavior: Scrolls preview panel content up by 3 lines
    - Condition: Only in stateSessionList, when mouse is over preview pane

75. **Mouse Wheel Down (Preview Area)** → Scroll Preview Down
    - Handler: D:\code\dispatch\internal\tui\model.go (lines 1086-1094)
    - Behavior: Scrolls preview panel content down by 3 lines
    - Condition: Only in stateSessionList, when mouse is over preview pane

## ADDITIONAL NOTES

### State Diagram
- **stateLoading** → Initial state during data load
- **stateSessionList** → Main view; most keys active
- **stateHelpOverlay** → Help modal; only ? and Esc close it
- **stateShellPicker** → Shell selection; up/down/enter/esc only
- **stateFilterPanel** → Filter overlay; navigation and select/apply
- **stateAttentionPicker** → Attention status filter; up/down/enter/esc, space to toggle
- **stateConfigPanel** → Config editor; navigation, enter to edit, esc to save

### Key Binding Implementation
- Uses Bubble Tea's key.Binding system (charmbracelet/bubbles/key)
- Key matching via key.Matches(msg, keyBindingName)
- All key bindings defined in D:\code\dispatch\internal\tui\keys.go (lines 57-87)

### Modifiers
- **Ctrl+C** → Force quit (special case, always active)
- **Shift+S** → Requires capital S (handled by Bubble Tea as shift+s)
- **Mouse modifiers (Ctrl, Shift)** → Ctrl+click toggles multi-select; Shift+click range-selects; on double-click they override launch mode
- **Alt key** → Not currently used for keyboard shortcuts
