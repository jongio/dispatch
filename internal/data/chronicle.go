package data

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Chronicle reindex timeout and timing constants.
const (
	// chronicleStartupWait is the maximum time to wait for the Copilot CLI
	// TUI to finish loading before sending commands. Early-exit detection
	// usually cuts this short — see startupReadyLines.
	chronicleStartupWait = 20 * time.Second

	// startupReadyLines is the number of unique emitted lines from the PTY
	// that indicate the Copilot CLI has finished loading. Once this many
	// meaningful lines have been seen, we wait startupGracePeriod more and
	// then proceed — cutting the startup delay from 20s to ~5-8s.
	startupReadyLines = 5

	// startupGracePeriod is additional time to wait after the readiness
	// threshold is reached, giving the CLI time to fully settle.
	startupGracePeriod = 2 * time.Second

	// chronicleExpWait is how long to wait after sending /experimental on.
	chronicleExpWait = 5 * time.Second

	// chronicleReindexTimeout is the maximum time to wait for the reindex
	// ETL to complete. Large session stores may take a while.
	chronicleReindexTimeout = 120 * time.Second

	// chronicleExitWait is how long to wait after sending /exit.
	chronicleExitWait = 5 * time.Second

	// chronicleReadBuf is the read buffer size for PTY output.
	chronicleReadBuf = 8192
)

// ErrCopilotNotFound is returned when the Copilot CLI binary cannot be
// located on the system.
var ErrCopilotNotFound = errors.New("copilot CLI binary not found")

// ansiRegex strips ANSI escape sequences from terminal output.
var ansiRegex = regexp.MustCompile(
	`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07|\x1b[()][AB012]|\x1b\[[\?]?[0-9;]*[hlm]`,
)

// minLogLineLen is the minimum length for a PTY output line to be
// considered meaningful and forwarded to the log callback. Shorter
// fragments are typically TUI rendering artifacts.
const minLogLineLen = 3

// ErrReindexCancelled is returned when the user cancels a running reindex.
var ErrReindexCancelled = errors.New("reindex cancelled")

// dedupeWindow is the number of recent unique lines to track for
// deduplication. This catches repeated TUI chrome from screen redraws.
const dedupeWindow = 50

// ChronicleReindex launches the Copilot CLI in a pseudo-terminal, sends
// the /chronicle reindex slash command, and streams cleaned output lines
// to the provided callback. This performs the full ETL: reading session
// JSONL files, workspace metadata, and checkpoints into the SQLite store.
//
// If the Copilot CLI binary cannot be found, it returns ErrCopilotNotFound
// and the caller should fall back to Maintain().
//
// The ctx parameter supports cancellation — when cancelled the PTY is
// closed immediately and ErrReindexCancelled is returned.
//
// The onLine callback receives each non-empty line of stripped output. It
// is called from a goroutine and must be safe for concurrent use.
func ChronicleReindex(ctx context.Context, onLine func(line string)) error {
	binary := findCopilotBinary()
	if binary == "" {
		return ErrCopilotNotFound
	}

	ptty, err := startPTY(binary)
	if err != nil {
		return fmt.Errorf("starting copilot PTY: %w", err)
	}
	defer ptty.Close() //nolint:errcheck // best-effort cleanup

	// Pump PTY output into a channel so we can read with timeouts.
	outCh := make(chan string, 1000)
	go func() {
		buf := make([]byte, chronicleReadBuf)
		for {
			n, rErr := ptty.Read(buf)
			if n > 0 {
				outCh <- string(buf[:n])
			}
			if rErr != nil {
				close(outCh)
				return
			}
		}
	}()

	// status emits a synthetic progress message directly to onLine,
	// bypassing dedup/filtering since these are our own status updates.
	status := func(msg string) {
		if onLine != nil {
			onLine(msg)
		}
	}

	// ----- Streaming state shared across all collect calls -----
	var lineBuf strings.Builder   // current line being assembled
	var allOutput strings.Builder // accumulated emitted lines for success check
	var linesEmitted int          // total unique lines emitted (for startup readiness)

	// Dedup window — skip lines seen in the last dedupeWindow unique
	// lines. This catches repeated TUI chrome from screen redraws.
	seen := make(map[string]struct{}, dedupeWindow)
	var seenOrder []string

	// emit flushes lineBuf as a completed line, deduplicates, and
	// sends to onLine if the line is meaningful and not recently seen.
	emit := func() {
		line := strings.TrimSpace(lineBuf.String())
		lineBuf.Reset()
		if len(line) < minLogLineLen {
			return
		}
		if _, dup := seen[line]; dup {
			return
		}
		// Add to dedup window, evicting oldest if full.
		if len(seenOrder) >= dedupeWindow {
			delete(seen, seenOrder[0])
			seenOrder = seenOrder[1:]
		}
		seen[line] = struct{}{}
		seenOrder = append(seenOrder, line)
		allOutput.WriteString(line)
		allOutput.WriteByte('\n')
		linesEmitted++
		if onLine != nil {
			onLine(line)
		}
	}

	// processChunk strips ANSI codes and processes characters:
	//   \n — emits the completed line
	//   \r — resets the line buffer (handles TUI spinner/progress overwrites)
	//   control chars < 32 (except \t) — discarded
	processChunk := func(raw string) {
		stripped := ansiRegex.ReplaceAllString(raw, "")
		for _, r := range stripped {
			switch {
			case r == '\n':
				emit()
			case r == '\r':
				lineBuf.Reset()
			case r >= 32 || r == '\t':
				lineBuf.WriteRune(r)
			}
		}
	}

	// collect drains PTY output for the given duration, processing
	// chunks as they arrive in real time. When earlyLines > 0, it
	// enables startup readiness detection: once linesEmitted reaches
	// the threshold, a grace period timer starts and collect returns
	// early — cutting the startup delay from 20s to ~5-8s.
	collect := func(d time.Duration, earlyLines int) error {
		deadline := time.After(d)
		var earlyTimer <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				_ = ptty.Close()
				return ErrReindexCancelled
			case chunk, ok := <-outCh:
				if !ok {
					emit() // flush remaining
					return nil
				}
				processChunk(chunk)
				if earlyLines > 0 && earlyTimer == nil && linesEmitted >= earlyLines {
					earlyTimer = time.After(startupGracePeriod)
				}
			case <-earlyTimer:
				emit() // flush remaining
				return nil
			case <-deadline:
				emit() // flush remaining
				return nil
			}
		}
	}

	status("⏳ Starting Copilot CLI...")

	// 1. Wait for the TUI to finish loading, with early exit once
	// we've seen enough meaningful output lines from the CLI.
	if err := collect(chronicleStartupWait, startupReadyLines); err != nil {
		return err
	}

	status("⏳ Enabling experimental features...")

	// 2. Enable experimental mode (required for /chronicle commands).
	if _, err := ptty.Write([]byte("/experimental on\r\n")); err != nil {
		return fmt.Errorf("sending /experimental on: %w", err)
	}
	if err := collect(chronicleExpWait, 0); err != nil {
		return err
	}

	status("⏳ Starting chronicle reindex...")

	// 3. Send /chronicle reindex.
	if _, err := ptty.Write([]byte("/chronicle reindex\r\n")); err != nil {
		return fmt.Errorf("sending /chronicle reindex: %w", err)
	}
	if err := collect(chronicleReindexTimeout, 0); err != nil {
		return err
	}

	// 4. Send /exit to shut down cleanly.
	ptty.Write([]byte("/exit\r\n")) //nolint:errcheck
	if err := collect(chronicleExitWait, 0); err != nil {
		return err
	}

	// Check for success markers in the output.
	lower := strings.ToLower(allOutput.String())
	if strings.Contains(lower, "reindexed") || strings.Contains(lower, "sessions") {
		return nil
	}
	if strings.Contains(lower, "error") {
		return fmt.Errorf("chronicle reindex reported an error")
	}

	// No explicit success/error — assume it worked if we got this far.
	return nil
}
