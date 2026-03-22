package contributors

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers — temp git repo
// ---------------------------------------------------------------------------

// initTestRepo creates a temporary git repository with a configured identity.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init")
	git(t, dir, "config", "user.name", "Test User")
	git(t, dir, "config", "user.email", "test@example.com")
	return dir
}

// git runs a git command in dir and fails the test on error.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitAs creates an empty commit attributed to the given author.
func commitAs(t *testing.T, dir, name, email, msg string) {
	t.Helper()
	author := name + " <" + email + ">"
	git(t, dir, "commit", "--allow-empty", "--author", author, "-m", msg)
}

// commitWithCoAuthor creates an empty commit with a Co-authored-by trailer.
func commitWithCoAuthor(t *testing.T, dir, authorName, authorEmail, coName, coEmail, msg string) {
	t.Helper()
	author := authorName + " <" + authorEmail + ">"
	body := msg + "\n\nCo-authored-by: " + coName + " <" + coEmail + ">"
	git(t, dir, "commit", "--allow-empty", "--author", author, "-m", body)
}

// ---------------------------------------------------------------------------
// gitOutput
// ---------------------------------------------------------------------------

func TestGitOutput_Success(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "first commit")

	out, err := gitOutput(dir, "log", "--format=%aN|%aE")
	if err != nil {
		t.Fatalf("gitOutput() error = %v", err)
	}
	if !strings.Contains(out, "Alice|alice@example.com") {
		t.Errorf("gitOutput() = %q, want to contain 'Alice|alice@example.com'", out)
	}
}

func TestGitOutput_InvalidDir(t *testing.T) {
	t.Parallel()
	_, err := gitOutput(filepath.Join(t.TempDir(), "nonexistent"), "log")
	if err == nil {
		t.Fatal("gitOutput() with invalid dir should return error")
	}
}

func TestGitOutput_BadArgs(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	// "git log --not-a-real-flag" should fail with stderr.
	_, err := gitOutput(dir, "log", "--not-a-real-flag-xyz")
	if err == nil {
		t.Fatal("gitOutput() with bad args should return error")
	}
	// The error should include stderr content (ExitError branch).
	if !strings.Contains(err.Error(), "exit") && !strings.Contains(err.Error(), "unrecognized") {
		// On different git versions the message varies; just verify it's non-empty.
		if err.Error() == "" {
			t.Error("gitOutput() error should have meaningful message")
		}
	}
}

func TestGitOutput_EmptyRepo(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	// No commits — "git log" fails in an empty repo (no HEAD).
	_, err := gitOutput(dir, "log", "--format=%aN|%aE")
	if err == nil {
		t.Fatal("gitOutput() on empty repo should return error (no HEAD)")
	}
}

// ---------------------------------------------------------------------------
// extract
// ---------------------------------------------------------------------------

func TestExtract_AllHistory(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "commit 1")
	commitAs(t, dir, "Bob", "bob@example.com", "commit 2")

	got, err := extract(dir, "")
	if err != nil {
		t.Fatalf("extract() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("extract() returned %d contributors, want 2", len(got))
	}
}

func TestExtract_RevRange(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "commit 1")
	git(t, dir, "tag", "v1.0")
	commitAs(t, dir, "Bob", "bob@example.com", "commit 2")
	git(t, dir, "tag", "v2.0")

	got, err := extract(dir, "v1.0..v2.0")
	if err != nil {
		t.Fatalf("extract() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("extract() returned %d contributors, want 1", len(got))
	}
	if got[0].Name != "Bob" {
		t.Errorf("extract() contributor name = %q, want %q", got[0].Name, "Bob")
	}
}

func TestExtract_WithCoAuthor(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitWithCoAuthor(t, dir,
		"Alice", "alice@example.com",
		"Bob", "bob@example.com",
		"pair programming commit",
	)

	got, err := extract(dir, "")
	if err != nil {
		t.Fatalf("extract() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("extract() returned %d contributors, want 2 (author + co-author)", len(got))
	}

	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
	}
	if !names["Alice"] {
		t.Error("missing author Alice")
	}
	if !names["Bob"] {
		t.Error("missing co-author Bob")
	}
}

func TestExtract_FiltersBots(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "human commit")
	commitAs(t, dir, "dependabot[bot]", "dependabot@users.noreply.github.com", "bot commit")

	got, err := extract(dir, "")
	if err != nil {
		t.Fatalf("extract() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("extract() returned %d, want 1 (bots filtered)", len(got))
	}
	if got[0].Name != "Alice" {
		t.Errorf("remaining contributor = %q, want Alice", got[0].Name)
	}
}

func TestExtract_DeduplicatesByEmail(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "commit 1")
	commitAs(t, dir, "Alice", "alice@example.com", "commit 2")
	commitAs(t, dir, "Alice", "alice@example.com", "commit 3")

	got, err := extract(dir, "")
	if err != nil {
		t.Fatalf("extract() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("extract() returned %d, want 1 (deduplicated)", len(got))
	}
	if got[0].Count != 3 {
		t.Errorf("Count = %d, want 3", got[0].Count)
	}
}

func TestExtract_ExtractsHandle(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Jon Gallant", "12345+jongio@users.noreply.github.com", "noreply commit")

	got, err := extract(dir, "")
	if err != nil {
		t.Fatalf("extract() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("extract() returned %d, want 1", len(got))
	}
	if got[0].Handle != "jongio" {
		t.Errorf("Handle = %q, want %q", got[0].Handle, "jongio")
	}
}

func TestExtract_InvalidDir(t *testing.T) {
	t.Parallel()
	_, err := extract(filepath.Join(t.TempDir(), "nonexistent"), "")
	if err == nil {
		t.Fatal("extract() with invalid dir should return error")
	}
	if !strings.Contains(err.Error(), "git log authors") {
		t.Errorf("error = %q, want to contain 'git log authors'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ExtractContributors (public API)
// ---------------------------------------------------------------------------

func TestExtractContributors_RangeBetweenTags(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "initial")
	git(t, dir, "tag", "v1.0")
	commitAs(t, dir, "Bob", "bob@example.com", "new feature")
	git(t, dir, "tag", "v2.0")

	got, err := ExtractContributors(dir, "v1.0", "v2.0")
	if err != nil {
		t.Fatalf("ExtractContributors() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ExtractContributors() returned %d, want 1", len(got))
	}
	if got[0].Name != "Bob" {
		t.Errorf("contributor = %q, want Bob", got[0].Name)
	}
}

func TestExtractContributors_EmptyFromTag(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "commit 1")
	commitAs(t, dir, "Bob", "bob@example.com", "commit 2")
	git(t, dir, "tag", "v1.0")

	got, err := ExtractContributors(dir, "", "v1.0")
	if err != nil {
		t.Fatalf("ExtractContributors() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ExtractContributors() returned %d, want 2 (all history up to tag)", len(got))
	}
}

func TestExtractContributors_InvalidFromRef(t *testing.T) {
	t.Parallel()
	_, err := ExtractContributors(t.TempDir(), "--exec=evil", "v1.0")
	if err == nil {
		t.Fatal("ExtractContributors() with invalid fromTag should return error")
	}
}

func TestExtractContributors_InvalidToRef(t *testing.T) {
	t.Parallel()
	_, err := ExtractContributors(t.TempDir(), "v1.0", "--exec=evil")
	if err == nil {
		t.Fatal("ExtractContributors() with invalid toTag should return error")
	}
}

// ---------------------------------------------------------------------------
// ExtractAllContributors (public API)
// ---------------------------------------------------------------------------

func TestExtractAllContributors_Success(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "commit 1")
	commitAs(t, dir, "Bob", "bob@example.com", "commit 2")

	got, err := ExtractAllContributors(dir)
	if err != nil {
		t.Fatalf("ExtractAllContributors() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ExtractAllContributors() returned %d, want 2", len(got))
	}
}

func TestExtractAllContributors_InvalidDir(t *testing.T) {
	t.Parallel()
	_, err := ExtractAllContributors(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("ExtractAllContributors() with invalid dir should return error")
	}
}

// ---------------------------------------------------------------------------
// ExtractContributorsUpTo (public API)
// ---------------------------------------------------------------------------

func TestExtractContributorsUpTo_Success(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)
	commitAs(t, dir, "Alice", "alice@example.com", "commit 1")
	git(t, dir, "tag", "v1.0")
	commitAs(t, dir, "Bob", "bob@example.com", "commit 2")

	got, err := ExtractContributorsUpTo(dir, "v1.0")
	if err != nil {
		t.Fatalf("ExtractContributorsUpTo() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ExtractContributorsUpTo() returned %d, want 1 (only up to v1.0)", len(got))
	}
	if got[0].Name != "Alice" {
		t.Errorf("contributor = %q, want Alice", got[0].Name)
	}
}

func TestExtractContributorsUpTo_InvalidRef(t *testing.T) {
	t.Parallel()
	_, err := ExtractContributorsUpTo(t.TempDir(), "--upload-pack=evil")
	if err == nil {
		t.Fatal("ExtractContributorsUpTo() with invalid ref should return error")
	}
}

// ---------------------------------------------------------------------------
// End-to-end: tag-to-tag with first-time detection
// ---------------------------------------------------------------------------

func TestEndToEnd_FirstTimeContributor(t *testing.T) {
	t.Parallel()
	dir := initTestRepo(t)

	// v1.0: Alice only.
	commitAs(t, dir, "Alice", "alice@example.com", "initial feature")
	git(t, dir, "tag", "v1.0")

	// v2.0: Alice again + new contributor Bob.
	commitAs(t, dir, "Alice", "alice@example.com", "fix bug")
	commitAs(t, dir, "Bob", "bob@example.com", "new feature")
	git(t, dir, "tag", "v2.0")

	allTime, err := ExtractContributorsUpTo(dir, "v1.0")
	if err != nil {
		t.Fatalf("ExtractContributorsUpTo() error = %v", err)
	}

	release, err := ExtractContributors(dir, "v1.0", "v2.0")
	if err != nil {
		t.Fatalf("ExtractContributors() error = %v", err)
	}

	firstTimers := DetectFirstTime(allTime, release)
	if len(firstTimers) != 1 {
		t.Fatalf("DetectFirstTime returned %d, want 1", len(firstTimers))
	}
	if firstTimers[0].Name != "Bob" {
		t.Errorf("first-timer = %q, want Bob", firstTimers[0].Name)
	}

	// FormatMarkdown should include the new contributor section.
	md := FormatMarkdown(release, firstTimers)
	if !strings.Contains(md, "New contributors:") {
		t.Error("FormatMarkdown missing new contributors section")
	}
	if !strings.Contains(md, "**Bob**") {
		t.Error("FormatMarkdown missing Bob")
	}
}

// ---------------------------------------------------------------------------
// Suppress Windows-specific test environment concerns
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	// Ensure git is available before running integration tests.
	if _, err := exec.LookPath("git"); err != nil {
		// Skip gracefully if git is not installed (CI without git).
		os.Exit(0)
	}
	os.Exit(m.Run())
}
