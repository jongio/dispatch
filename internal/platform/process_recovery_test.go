package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStopRecoveryProcessTreeRejectsInvalidPID(t *testing.T) {
	err := StopRecoveryProcessTree(context.Background(), 0, time.Millisecond, time.Millisecond)
	if err == nil {
		t.Fatal("expected invalid PID error")
	}
}

func TestRecoveryHelperProcess(t *testing.T) {
	if os.Getenv("DISPATCH_RECOVERY_HELPER") != "1" {
		return
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestStopRecoveryProcessTreeStopsVerifiedSubprocess(t *testing.T) {
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := "copilot"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(t.TempDir(), name)
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, content, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(target, "-test.run=TestRecoveryHelperProcess")
	cmd.Env = append(os.Environ(), "DISPATCH_RECOVERY_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	count, _, err := CheckRecoveryProcessTree(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("CheckRecoveryProcessTree: %v", err)
	}
	if count < 1 {
		t.Fatalf("process count = %d, want at least 1", count)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := StopRecoveryProcessTree(ctx, cmd.Process.Pid, 25*time.Millisecond, 2*time.Second); err != nil {
		t.Fatalf("StopRecoveryProcessTree: %v", err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("force-stopped helper unexpectedly exited successfully")
	}
	cmd.Process = nil
}
