# Internal/Platform Test Coverage Analysis (60.6%)

## SHELL.GO - EXPORTED FUNCTIONS STATUS

### FULLY TESTED (with good coverage):
1. **DefaultTerminal()** - Lines 326-341
   - Tests: TestDefaultTerminalReturnsNonEmpty, TestDefaultTerminalReturnsExpectedPerOS
   - Status: ✓ Complete

2. **DefaultShell()** - Lines 314-319
   - Tests: TestDefaultShellReturnsValidShellInfo, TestDefaultShellHasPath
   - Status: ✓ Complete

3. **DetectTerminals()** - Lines 768-777
   - Tests: TestDetectTerminalsNonEmpty, TestDetectTerminalsValidFields
   - Status: ✓ Complete (delegates to platform-specific functions)

4. **DetectShells()** - Lines 304-309
   - Tests: TestDetectShellsNonEmpty, TestDetectShellsValidFields
   - Status: ✓ Complete (delegates to platform-specific functions)

5. **BuildResumeArgs()** - Lines 98-113
   - Tests: TestBuildResumeArgs (5 comprehensive test cases)
   - Status: ✓ Complete

6. **NewResumeCmd()** - Lines 122-147
   - Tests: TestNewResumeCmd_CustomCommand, TestNewResumeCmd_EmptySessionIDStartsNewSession, TestNewResumeCmd_CustomCommandEmptyTemplate
   - Status: ✓ Complete

7. **FindCLIBinary()** - Lines 59-67
   - Tests: TestFindCLIBinary_ReturnsStringOrEmpty
   - Status: ✓ Basic (only smoke test)

### HELPER FUNCTIONS (NOT EXPORTED - lowercase):
8. **buildResumeCommandString()** - Lines 190-228
   - Tests: Multiple tests in shell_test.go and platform_additional_test.go
   - Status: ✓ Complete

9. **shellQuote()** - Lines 234-245
   - Tests: TestShellQuote (7 cases), TestShellQuoteAllMetachars, TestShellQuotePreservesContent
   - Status: ✓ Complete

10. **psQuote()** - Lines 255-269
    - Tests: TestPsQuote, TestPsQuoteNoDoubleQuotes, TestPsQuoteMultipleDoubleQuotes, TestPsQuoteEmptyInput, TestPsQuoteEscapesMetachars
    - Status: ✓ Complete

11. **cmdQuote()** - Lines 275-281
    - Tests: TestCmdQuote, TestCmdQuoteStripsNullBytes
    - Status: ✓ Complete

12. **cmdEscape()** - Lines 285-301
    - Tests: TestCmdEscape_EmptyString through TestCmdEscape (13 test cases)
    - Status: ✓ Complete

13. **validateSessionID()** - Lines 88-93
    - Tests: TestValidateSessionID (20+ cases), TestValidateSessionID_BoundaryLengths
    - Status: ✓ Complete

14. **escapeAppleScript()** - Lines 700-713
    - Tests: TestEscapeAppleScript (7 cases), TestEscapeAppleScript_Additional, TestEscapeAppleScript_ControlChars, TestEscapeAppleScript_TabsAndNewlines
    - Status: ✓ Complete

15. **buildCustomCmd()** - Lines 167-177
    - Tests: TestBuildCustomCmd (4 cases), TestBuildCustomCmd_RejectsNewlines
    - Status: ✓ Complete

16. **validateCustomCommand()** - Lines 155-163
    - Tests: TestValidateCustomCommand, TestValidateCustomCommand_TabsAllowed, TestValidateCustomCommand_LongCommand
    - Status: ✓ Complete

17. **resolvedCwd()** - Lines 76-84
    - Tests: TestResolvedCwd_EmptyString, TestResolvedCwd_ExistingDir, TestResolvedCwd_NonexistentDir, TestResolvedCwd_FileNotDir
    - Status: ✓ Complete

### PLATFORM-SPECIFIC FUNCTIONS (Windows-ONLY - NOT testable on Windows in isolation):
18. **detectWindowsShells()** - Lines 406-443
    - Tests: INDIRECTLY via DetectShells()
    - Status: ⚠ NO DIRECT TESTS (private function, covered via DetectShells on Windows)
    - Coverage: Partial (depends on available shells in environment)

19. **defaultWindowsShell()** - Lines 445-455
    - Tests: INDIRECTLY via DefaultShell()
    - Status: ⚠ NO DIRECT TESTS (private function, covered via DefaultShell on Windows)

20. **launchWindowsSession()** - Lines 457-502
    - Tests: NONE (Windows-only platform-specific launch code)
    - Status: ✗ UNTESTED
    - What it needs: Tests for Windows Terminal tab/window/pane modes, cmd.exe fallback path
    - Why hard to test: Requires wt.exe, Windows Terminal, cmd.exe to actually run
    - Testable approach: Mock exec.Command, test logic without actual process execution

21. **buildStartCmdLine()** - Lines 512-539
    - Tests: NONE DIRECT (helper for launchWindowsSession fallback)
    - Status: ✗ UNTESTED
    - What it needs: Test cmd.exe command line building for PowerShell, cmd, and generic shells
    - Testable on Windows: YES - can test string construction without actually running processes

22. **launchInPlaceWindows()** - Lines 541-563
    - Tests: NONE (Windows-only in-place launch)
    - Status: ✗ UNTESTED
    - What it needs: Test argument building for PowerShell, cmd.exe, and other shells
    - Testable on Windows: YES - can test exec.Cmd construction and Dir/Stdin/Stdout/Stderr setup

### PLATFORM-SPECIFIC FUNCTIONS (Unix/Darwin-ONLY):
23. **detectUnixShells()** - Lines 569-605
    - Tests: INDIRECTLY via DetectShells()
    - Status: ⚠ INCOMPLETE (no test for /etc/shells parsing path or fallback)
    - Coverage: Partial, depends on /etc/shells availability

24. **defaultUnixShell()** - Lines 607-623
    - Tests: INDIRECTLY via DefaultShell()
    - Status: ⚠ INCOMPLETE (no specific test for SHELL env var fallback)

25. **launchDarwinSession()** - Lines 625-694
    - Tests: NONE (macOS-only, requires osascript)
    - Status: ✗ UNTESTED
    - What it needs: Test Terminal.app, iTerm2, WezTerm AppleScript command building
    - Testable on Windows: NO (macOS-only)

26. **launchLinuxSession()** - Lines 715-760
    - Tests: NONE (Linux-only terminal launching)
    - Status: ✗ UNTESTED
    - What it needs: Test terminal selection logic for alacritty, kitty, wezterm, gnome-terminal, konsole, xfce4-terminal, xterm
    - Testable on Windows: Partially - can test logic without running actual terminals

27. **launchInPlaceUnix()** - lines 44-56 in launch_unix.go
    - Tests: FilterEnv tests indirectly prove filterEnv works
    - Status: ⚠ INCOMPLETE (syscall.Exec never returns; Chdir tested indirectly only)

### EXPORTED FUNCTIONS (in LaunchSession/LaunchSessionInPlace):
28. **LaunchSession()** - Lines 351-374
    - Tests: TestLaunchSession_* (several error cases)
    - Status: ⚠ INCOMPLETE
    - Issue: Tests only cover error paths; actual launching requires platform-specific code
    - Missing: Test successful launch paths with valid shell/terminal

29. **LaunchSessionInPlace()** - Lines 382-400
    - Tests: TestLaunchSessionInPlace_* (error cases)
    - Status: ⚠ INCOMPLETE
    - Issue: Only error path tests; actual process replacement/execution not tested
    - Missing: Test successful execution paths

## FONTS.GO - EXPORTED FUNCTIONS:

### FULLY TESTED:
1. **IsNerdFontInstalled()** - Lines 42-51
   - Tests: TestIsNerdFontInstalled_ReturnsBool (smoke test only)
   - Status: ✓ (though only smoke test)

2. **InstallNerdFont()** - Lines 56-84
   - Tests: NONE (requires actual downloading)
   - Status: ✗ UNTESTED (integration function, hard to test)

### HELPER FUNCTIONS (not exported):
3. **hasNerdFontFiles()** - Lines 138-150
   - Tests: TestHasNerdFontFiles_* (9 test cases)
   - Status: ✓ Complete

4. **downloadFile()** - Lines 224-263
   - Tests: NONE (uses actual HTTP)
   - Status: ✗ UNTESTED (would need HTTP mocking)

5. **extractTTF()** - Lines 266-268
   - Tests: TestExtractTTF_* (many comprehensive tests)
   - Status: ✓ Complete

6. **extractTTFWithLimits()** - Lines 273-331
   - Tests: TestExtractTTF_* suite
   - Status: ✓ Complete

7. **copyFile()** - Lines 333-351
   - Tests: TestCopyFile_* (5+ test cases)
   - Status: ✓ Complete

### Platform-specific (not exported):
8. **isNerdFontInstalledWindows()** - Lines 90-103
   - Tests: NONE (platform-specific folder checks)
   - Status: ✗ UNTESTED

9. **isNerdFontInstalledDarwin()** - Lines 105-114
   - Tests: NONE
   - Status: ✗ UNTESTED

10. **isNerdFontInstalledLinux()** - Lines 116-134
    - Tests: NONE
    - Status: ✗ UNTESTED

11. **installFontsWindows()** - Lines 156-180
    - Tests: NONE (registry operations, folder creation)
    - Status: ✗ UNTESTED

12. **installFontsDarwin()** - Lines 182-198
    - Tests: NONE
    - Status: ✗ UNTESTED

13. **installFontsLinux()** - Lines 200-218
    - Tests: NONE (fc-cache execution)
    - Status: ✗ UNTESTED

## TERMDETECT.GO - EXPORTED FUNCTIONS:

### FULLY TESTED:
1. **DetectTerminalScheme()** - Lines 18-32
   - Tests: TestDetectTerminalScheme_DoesNotPanic, TestDetectTerminalScheme_ReturnsNilOrValidScheme
   - Status: ✓ (smoke tests)

2. **DetectIsDark()** - Lines 36-38
   - Tests: TestDetectIsDark_DoesNotPanic
   - Status: ✓ (smoke test)

### HELPER FUNCTIONS (not exported):
3. **detectViaOSC()** - Lines 44-72
   - Tests: NONE (OSC detection depends on terminal capabilities)
   - Status: ✗ UNTESTED

4. **colorToHex()** - Lines 76-94
   - Tests: TestColorToHex_* (8 test cases including nil, RGB, ANSI)
   - Status: ✓ Complete

5. **wtToColorScheme()** - Lines 97-121
   - Tests: TestWtToColorScheme_* (3 test cases)
   - Status: ✓ Complete

## WTTHEME.GO - EXPORTED FUNCTIONS (Windows-only, build tag):

### FULLY TESTED:
1. **DetectWTColorScheme()** - Lines 103-115
   - Tests: TestDetectWTColorScheme_DoesNotPanic (smoke test)
   - Status: ✓ (light smoke test)

### HELPER FUNCTIONS:
2. **wtSettingsPaths()** - Lines 118-129
   - Tests: TestWtSettingsPaths_ReturnsPathsWhenLocalAppDataSet
   - Status: ⚠ Incomplete (only tests when LOCALAPPDATA set)

3. **parseWTSettings()** - Lines 133-139
   - Tests: TestParseWTSettings_ValidFile, TestParseWTSettings_NonexistentFile, TestParseWTSettings_InvalidJSONFile
   - Status: ✓ Good

4. **parseWTSettingsData()** - Lines 142-177
   - Tests: TestParseWTSettings_* (comprehensive JSON parsing tests - 7+ cases)
   - Status: ✓ Complete

### HELPER TYPES:
5. **colorSchemeRef.UnmarshalJSON()** - Lines 50-68
   - Tests: TestColorSchemeRef_UnmarshalJSON_* (4 test cases)
   - Status: ✓ Complete

6. **colorSchemeRef.resolve()** - Lines 72-80
   - Tests: TestColorSchemeRef_Resolve_* (5 test cases)
   - Status: ✓ Complete

## PATHS.GO - EXPORTED FUNCTIONS:

### FULLY TESTED:
1. **SessionStorePath()** - Lines 24-33
   - Tests: TestSessionStorePathNonEmpty, TestSessionStorePathEndsWith, TestSessionStorePathAbsolute, TestSessionStorePathContainsHomeDir, TestSessionStorePath_DispatchDBOverride
   - Status: ✓ Complete

2. **ConfigDir()** - Lines 40-46
   - Tests: TestConfigDirNonEmpty, TestConfigDirEndsWithDispatch, TestConfigDirAbsolute, TestConfigDirUsesOSConfigBase
   - Status: ✓ Complete

3. **EnsureConfigDir()** - Lines 50-59
   - Tests: TestEnsureConfigDirCreatesDirectory, TestEnsureConfigDirIdempotent
   - Status: ✓ Complete

4. **DefaultShellConfigPath()** - Lines 63-69
   - Tests: TestDefaultShellConfigPathNonEmpty, TestDefaultShellConfigPathEndsWithShellConf, TestDefaultShellConfigPathUnderConfigDir
   - Status: ✓ Complete

5. **Constants** - appName, sessionStoreRel, shellConfigFile, configDirPerm
   - Tests: TestAppNameConstant, TestSessionStoreRelConstant, TestShellConfigFileConstant
   - Status: ✓ Complete

## LAUNCH_UNIX.GO / LAUNCH_WINDOWS.GO:

### FULLY TESTED:
1. **filterEnv()** (in launch_unix.go) - Lines 23-42
   - Tests: TestFilterEnv_* (4 test cases covering dangerous var removal)
   - Status: ✓ Complete

2. **launchInPlaceUnix()** (in launch_unix.go) - Lines 44-56
   - Tests: NONE (actual syscall.Exec; can't test return)
   - Status: ⚠ UNTESTED (impossible to truly test Exec itself)

3. **launchInPlaceUnix()** (in launch_windows.go) - Lines 10-12
   - Tests: N/A (stub, always errors)
   - Status: N/A

---

## SUMMARY OF COVERAGE GAPS

### WINDOWS-TESTABLE (on this Windows dev environment):

| Function | File | Lines | Current Status | Test Type Needed | Priority |
|----------|------|-------|-----------------|------------------|----------|
| buildStartCmdLine | shell.go | 512-539 | ✗ NOT TESTED | Unit: Test cmd line building for pwsh, cmd.exe, bash | HIGH |
| launchInPlaceWindows | shell.go | 541-563 | ✗ NOT TESTED | Unit: Test exec.Cmd construction, args building | HIGH |
| launchWindowsSession | shell.go | 457-502 | ✗ NOT TESTED | Unit: Test wt.exe args building, fallback path | MEDIUM |
| detectWindowsShells | shell.go | 406-443 | ⚠ PARTIAL | Unit: Direct test of shell detection logic | MEDIUM |
| defaultWindowsShell | shell.go | 445-455 | ⚠ PARTIAL | Unit: Direct test of default shell selection | MEDIUM |
| installFontsWindows | fonts.go | 156-180 | ✗ NOT TESTED | Unit: Mock registry ops, test file copying | MEDIUM |
| isNerdFontInstalledWindows | fonts.go | 90-103 | ✗ NOT TESTED | Unit: Test font folder checks | LOW |
| LaunchSession | shell.go | 351-374 | ⚠ PARTIAL | Integration: Test successful launch path | MEDIUM |
| LaunchSessionInPlace | shell.go | 382-400 | ⚠ PARTIAL | Integration: Test successful exec path | MEDIUM |
| detectViaOSC | termdetect.go | 44-72 | ✗ NOT TESTED | Unit: Test OSC detection with mock terminal | LOW |
| wtSettingsPaths | wttheme.go | 118-129 | ⚠ PARTIAL | Unit: Test when LOCALAPPDATA is empty | LOW |

### NOT TESTABLE ON WINDOWS (platform-specific to Unix/Darwin):
- launchDarwinSession (shell.go:625-694)
- launchLinuxSession (shell.go:715-760)  
- detectUnixShells (shell.go:569-605) - partial
- defaultUnixShell (shell.go:607-623) - partial
- isNerdFontInstalledDarwin (fonts.go:105-114)
- isNerdFontInstalledLinux (fonts.go:116-134)
- installFontsDarwin (fonts.go:182-198)
- installFontsLinux (fonts.go:200-218)
- launchInPlaceUnix (launch_unix.go:44-56) - essentially untestable

### CAN'T TEST (require external tools/resources):
- InstallNerdFont (fonts.go:56-84) - requires HTTP download
- downloadFile (fonts.go:224-263) - requires real HTTP
- isNerdFontInstalledLinux fc-list check (fonts.go:129) - requires fc-list binary

---

## RECOMMENDED HIGH-PRIORITY TESTS FOR WINDOWS

1. **buildStartCmdLine** (shell.go:512-539) - Lines of code affected: 28
   Create test cases covering:
   - PowerShell: buildStartCmdLine(pwsh shell, cmd) → verifies -Command psQuote wrapping
   - Windows PowerShell: buildStartCmdLine(powershell.exe shell, cmd) → verifies -Command psQuote wrapping
   - cmd.exe: buildStartCmdLine(cmd shell, cmd) → verifies /k cmdEscape wrapping
   - Git Bash/Other: buildStartCmdLine(bash shell, cmd) → verifies -c passthrough
   
   Example test structure:
   `go
   func TestBuildStartCmdLine_PowerShell(t *testing.T) {
       shell := ShellInfo{Name: "PowerShell", Path: "pwsh.exe", Args: []string{"-NoLogo"}}
       cmd := "echo hello"
       got := buildStartCmdLine(shell, cmd)
       if !strings.Contains(got, -Command) { t.Error("missing -Command flag") }
       if !strings.HasPrefix(got, cmd.exe /c start "") { t.Error("wrong start command") }
   }
   `

2. **launchInPlaceWindows** (shell.go:541-563) - Lines of code affected: 23
   Create test cases for:
   - PowerShell args construction
   - cmd.exe /c quoting
   - Bash -c passthrough
   - Cwd directory setting
   - Stdin/Stdout/Stderr assignment
   
   Example:
   `go
   func TestLaunchInPlaceWindows_PowerShellArgs(t *testing.T) {
       shell := ShellInfo{Name: "PowerShell", Path: "pwsh.exe", Args: []string{"-NoLogo"}}
       resumeCmd := "copilot --resume abc123"
       cmd, _ := mockExecCommand(func() {}) // capture built command
       launchInPlaceWindows(shell, resumeCmd, "")
       // Assert cmd.Args contains "-Command" and psQuote(resumeCmd)
   }
   `

3. **detectWindowsShells** (shell.go:406-443) - Lines of code affected: 37
   Create comprehensive test checking:
   - pwsh.exe detection
   - powershell.exe detection
   - cmd.exe detection
   - Git Bash detection at multiple paths
   - WSL detection
   
   Example:
   `go
   func TestDetectWindowsShells_ReturnsList(t *testing.T) {
       shells := detectWindowsShells()
       // On Windows, should have at least cmd.exe
       found := false
       for _, s := range shells {
           if strings.Contains(s.Name, "Command") { found = true }
       }
       if !found { t.Error("cmd.exe not detected") }
   }
   `

4. **launchWindowsSession** (shell.go:457-502) - Lines of code affected: 45
   Create tests for:
   - Windows Terminal tab mode (-w 0 new-tab)
   - Windows Terminal new window mode (-w new new-tab)
   - Windows Terminal pane mode with direction
   - Starting directory handling
   - PowerShell/cmd.exe/bash command quoting
   - Fallback to cmd.exe /c start when wt.exe unavailable
   
   Example:
   `go
   func TestLaunchWindowsSession_WTTabMode(t *testing.T) {
       shell := ShellInfo{Name: "PowerShell", Path: "pwsh.exe"}
       // Mock exec.Command to capture what would be executed
       gotCmd := captureCommand(func() {
           launchWindowsSession(shell, "cmd", "Windows Terminal", "", "", "")
       })
       // Assert args contain: -w 0 new-tab --startingDirectory ...
   }
   `

