package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jongio/dispatch/internal/data"
	"github.com/jongio/dispatch/internal/tui/components"
)

const projectRootScanDepth = 3

var projectScanSkipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
}

func discoverProjectReposCmd(roots []string) tea.Cmd {
	if len(roots) == 0 {
		return nil
	}
	roots = append([]string(nil), roots...)
	return func() tea.Msg {
		repos, err := discoverGitRepos(roots, projectRootScanDepth)
		if err != nil {
			return projectQuickStartsMsg{err: err}
		}
		return projectQuickStartsMsg{repos: repos}
	}
}

func (m *Model) loadProjectReposCmd() tea.Cmd {
	if m.projectReposLoaded || m.projectReposLoading || len(m.cfg.ProjectRoots) == 0 {
		return nil
	}
	m.projectReposLoading = true
	return discoverProjectReposCmd(m.cfg.ProjectRoots)
}

func (m *Model) refreshProjectQuickStarts() {
	sessions := m.sessions
	if m.groups != nil {
		sessions = sessionsFromGroups(m.groups)
	}
	m.quickStarts = filterQuickStartRepos(m.projectRepos, sessions)
}

func discoverGitRepos(roots []string, maxDepth int) ([]components.QuickStart, error) {
	seen := map[string]struct{}{}
	var repos []components.QuickStart
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		cleanRoot, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(cleanRoot); err != nil {
			continue
		}
		err = filepath.WalkDir(cleanRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			name := d.Name()
			if _, skip := projectScanSkipDirs[name]; skip && path != cleanRoot {
				return filepath.SkipDir
			}
			depth, ok := pathDepth(cleanRoot, path)
			if !ok || depth > maxDepth {
				return filepath.SkipDir
			}
			if hasGitDir(path) {
				clean := filepath.Clean(path)
				if _, ok := seen[clean]; !ok {
					seen[clean] = struct{}{}
					repos = append(repos, components.QuickStart{Name: filepath.Base(clean), Path: clean})
				}
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(repos, func(i, j int) bool {
		return strings.ToLower(repos[i].Path) < strings.ToLower(repos[j].Path)
	})
	return repos, nil
}

func filterQuickStartRepos(repos []components.QuickStart, sessions []data.Session) []components.QuickStart {
	var out []components.QuickStart
	for _, repo := range repos {
		if repo.Path == "" || repoHasSession(repo.Path, sessions) {
			continue
		}
		out = append(out, repo)
	}
	return out
}

func sessionsFromGroups(groups []data.SessionGroup) []data.Session {
	var sessions []data.Session
	for _, group := range groups {
		sessions = append(sessions, group.Sessions...)
	}
	return sessions
}

func repoHasSession(repoPath string, sessions []data.Session) bool {
	repoPath = filepath.Clean(repoPath)
	for _, sess := range sessions {
		if sess.Cwd == "" {
			continue
		}
		cwd := filepath.Clean(sess.Cwd)
		if cwd == repoPath {
			return true
		}
		if rel, err := filepath.Rel(repoPath, cwd); err == nil && rel != "." && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			return true
		}
	}
	return false
}

func pathDepth(root, path string) (int, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return 0, false
	}
	if rel == "." {
		return 0, true
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return 0, false
	}
	return len(strings.Split(rel, string(filepath.Separator))), true
}

func hasGitDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}
