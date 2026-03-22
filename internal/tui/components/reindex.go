package components

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jongio/dispatch/internal/data"
)

// logBatchDelay is the pause after reading the first line to allow more
// lines to accumulate in the channel before flushing them as a batch.
const logBatchDelay = 80 * time.Millisecond

// ReindexFinishedMsg is sent when a reindex completes. The model
// re-queries the session store database on receipt.
type ReindexFinishedMsg struct {
	Err error
}

// reindexLogChanSize is the buffer capacity for the log channel.
const reindexLogChanSize = 500

// ReindexHandle holds the cancel function for a running reindex so
// the caller can abort it from the UI.
type ReindexHandle struct {
	Cancel context.CancelFunc
}

// StartChronicleReindex launches a full chronicle reindex via the Copilot
// CLI in a pseudo-terminal. It returns a ReindexHandle (for cancellation)
// and two Cmds: one that runs the reindex (sending ReindexFinishedMsg on
// completion), and one that pumps log lines into ReindexLogPump messages.
//
// Falls back to Maintain() if the copilot binary is not found.
func StartChronicleReindex() (ReindexHandle, []tea.Cmd) {
	logCh := make(chan string, reindexLogChanSize)
	ctx, cancel := context.WithCancel(context.Background())

	handle := ReindexHandle{Cancel: cancel}

	// Cmd 1: run the reindex, writing log lines to the channel.
	runCmd := func() tea.Msg {
		defer close(logCh)

		err := data.ChronicleReindex(ctx, func(line string) {
			// Non-blocking send — drop lines if the channel is full.
			select {
			case logCh <- line:
			default:
			}
		})

		if errors.Is(err, data.ErrCopilotNotFound) {
			logCh <- "Copilot CLI not found, running index maintenance…"
			if mErr := data.Maintain(); mErr != nil {
				return ReindexFinishedMsg{Err: mErr}
			}
			return ReindexFinishedMsg{Err: nil}
		}
		if err != nil {
			return ReindexFinishedMsg{Err: err}
		}

		// Run Maintain() after chronicle reindex to checkpoint WAL.
		if mErr := data.Maintain(); mErr != nil {
			logCh <- "Warning: post-reindex maintenance: " + mErr.Error()
		}
		return ReindexFinishedMsg{Err: nil}
	}

	// Cmd 2: pump log lines from the channel into tea messages.
	pumpCmd := waitForLog(logCh)

	return handle, []tea.Cmd{runCmd, pumpCmd}
}

// waitForLog returns a Cmd that reads lines from the channel, batching
// them to reduce re-render frequency. It blocks until at least one line
// is available, then pauses briefly and drains all remaining buffered
// lines before returning the batch as a single ReindexLogPump message.
func waitForLog(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		// Block until at least one line is available.
		line, ok := <-ch
		if !ok {
			return nil // channel closed — stop pumping
		}

		lines := []string{line}

		// Brief pause to let more lines accumulate.
		time.Sleep(logBatchDelay)

		// Drain all immediately available lines.
		for {
			select {
			case l, ok := <-ch:
				if !ok {
					return ReindexLogPump{Lines: lines, ch: ch}
				}
				lines = append(lines, l)
			default:
				return ReindexLogPump{Lines: lines, ch: ch}
			}
		}
	}
}

// ReindexLogPump carries a batch of log lines and the channel reference
// so the model can request the next batch.
type ReindexLogPump struct {
	Lines []string
	ch    <-chan string
}

// NextLogCmd returns the Cmd to wait for the next batch of log lines.
func (p ReindexLogPump) NextLogCmd() tea.Cmd {
	return waitForLog(p.ch)
}
