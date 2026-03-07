package gitinfo

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// FileChange represents a changed file in the repository.
type FileChange struct {
	Path       string // repo-relative, slash-separated
	Status     string // single-letter git status
	Insertions int
	Deletions  int
	Binary     bool
	Staged     bool
	Unstaged   bool
}

// Hunk represents a changed hunk range (0-based lines) in a file.
type Hunk struct {
	Path      string // repo-relative, slash-separated
	StartLine int
	EndLine   int
}

// Diff returns the unified diff for a single file relative to HEAD.
func Diff(root, path string) (string, error) {
	root = strings.TrimSpace(root)
	path = strings.TrimSpace(path)
	if root == "" {
		return "", errors.New("not a git repository")
	}
	if path == "" {
		return "", errors.New("path required")
	}
	pathspec := path
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(root, path); err == nil && rel != "" && rel != "." && !strings.HasPrefix(rel, "..") {
			pathspec = filepath.ToSlash(rel)
		}
	}
	out, err := exec.Command(
		"git",
		"-C",
		root,
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--unified=3",
		"HEAD",
		"--",
		pathspec,
	).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if isMissingHead(msg) {
			return "", nil
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return string(out), nil
}

// Changes returns the current git changes and hunk locations for the repo root.
func Changes(root string) ([]FileChange, []Hunk, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, nil, errors.New("not a git repository")
	}

	statuses, err := statusMap(root)
	if err != nil {
		return nil, nil, err
	}
	numstats, err := numstatMap(root)
	if err != nil {
		return nil, nil, err
	}
	hunks, err := diffHunks(root)
	if err != nil {
		return nil, nil, err
	}

	changes := make([]FileChange, 0, len(statuses)+len(numstats))
	for path, st := range statuses {
		ns, ok := numstats[path]
		change := FileChange{
			Path:     path,
			Status:   st.status,
			Staged:   st.staged,
			Unstaged: st.unstaged,
		}
		if ok {
			change.Insertions = ns.insertions
			change.Deletions = ns.deletions
			change.Binary = ns.binary
		}
		if !ok && (st.status == "?" || st.status == "A") {
			change.Insertions = countFileLines(filepath.Join(root, filepath.FromSlash(path)))
			change.Deletions = 0
		}
		changes = append(changes, change)
	}
	for path, ns := range numstats {
		if _, ok := statuses[path]; ok {
			continue
		}
		changes = append(changes, FileChange{
			Path:       path,
			Status:     "M",
			Insertions: ns.insertions,
			Deletions:  ns.deletions,
			Binary:     ns.binary,
			Unstaged:   true,
		})
	}

	// Add placeholder hunk for untracked files so navigation can land on them.
	for _, ch := range changes {
		if ch.Status == "?" {
			hunks = append(hunks, Hunk{
				Path:      ch.Path,
				StartLine: 0,
				EndLine:   0,
			})
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})
	sort.Slice(hunks, func(i, j int) bool {
		if hunks[i].Path != hunks[j].Path {
			return hunks[i].Path < hunks[j].Path
		}
		return hunks[i].StartLine < hunks[j].StartLine
	})
	return changes, hunks, nil
}

type numstat struct {
	insertions int
	deletions  int
	binary     bool
}

type statusInfo struct {
	status   string
	staged   bool
	unstaged bool
}

func statusMap(root string) (map[string]statusInfo, error) {
	out, err := exec.Command("git", "-C", root, "status", "--porcelain", "-z").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	entries := bytes.Split(out, []byte{0})
	statuses := make(map[string]statusInfo)
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 3 || entry[2] != ' ' {
			continue
		}
		statusX := entry[0]
		statusY := entry[1]
		path := string(entry[3:])
		if statusX == 'R' || statusY == 'R' || statusX == 'C' || statusY == 'C' {
			if i+1 < len(entries) {
				path = string(entries[i+1])
				i++
			}
		}
		status := statusY
		if status == ' ' {
			status = statusX
		}
		if status == 0 {
			status = '?'
		}
		info := statusInfo{
			status:   string(status),
			staged:   statusX != ' ' && statusX != '?',
			unstaged: statusY != ' ',
		}
		if statusX == '?' && statusY == '?' {
			info.staged = false
			info.unstaged = true
		}
		statuses[path] = info
	}
	return statuses, nil
}

func numstatMap(root string) (map[string]numstat, error) {
	out, err := exec.Command("git", "-C", root, "diff", "--numstat", "HEAD").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if isMissingHead(msg) {
			return map[string]numstat{}, nil
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	m := make(map[string]numstat)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		path := parseDiffPath(parts[2])
		ns := numstat{}
		if parts[0] == "-" || parts[1] == "-" {
			ns.binary = true
		} else {
			if v, err := strconv.Atoi(parts[0]); err == nil {
				ns.insertions = v
			}
			if v, err := strconv.Atoi(parts[1]); err == nil {
				ns.deletions = v
			}
		}
		m[path] = ns
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return m, nil
}

var hunkHeaderRe = regexp.MustCompile(`@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
var braceRenameRe = regexp.MustCompile(`\{([^{}]*?) => ([^{}]*?)\}`)

func diffHunks(root string) ([]Hunk, error) {
	out, err := exec.Command("git", "-C", root, "diff", "--unified=0", "HEAD").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if isMissingHead(msg) {
			return nil, nil
		}
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}
	var hunks []Hunk
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	currentPath := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "diff --git ") {
			currentPath = parseDiffGitPath(line)
			continue
		}
		if !strings.HasPrefix(line, "@@ ") || currentPath == "" {
			continue
		}
		matches := hunkHeaderRe.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		startLine, err := strconv.Atoi(matches[1])
		if err != nil || startLine < 1 {
			continue
		}
		count := 1
		if len(matches) > 2 && matches[2] != "" {
			if v, err := strconv.Atoi(matches[2]); err == nil {
				count = v
			}
		}
		if count == 0 {
			anchor := startLine - 1
			if anchor < 0 {
				anchor = 0
			}
			hunks = append(hunks, Hunk{
				Path:      currentPath,
				StartLine: anchor,
				EndLine:   anchor,
			})
			continue
		}
		hunks = append(hunks, Hunk{
			Path:      currentPath,
			StartLine: startLine - 1,
			EndLine:   startLine - 1 + count - 1,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hunks, nil
}

func parseDiffGitPath(line string) string {
	const prefix = "diff --git "
	rest := strings.TrimPrefix(line, prefix)
	idx := strings.Index(rest, " b/")
	if idx == -1 {
		return ""
	}
	path := strings.TrimSpace(rest[idx+3:])
	if len(path) > 1 && (path[0] == '"' || path[0] == '\'') {
		if unquoted, err := strconv.Unquote(path); err == nil {
			path = unquoted
		} else {
			path = strings.Trim(path, "\"'")
		}
	}
	return path
}

func parseDiffPath(path string) string {
	if !strings.Contains(path, "=>") {
		return path
	}
	if strings.Contains(path, "{") && strings.Contains(path, "}") {
		return braceRenameRe.ReplaceAllString(path, "$2")
	}
	if strings.Contains(path, " => ") {
		parts := strings.Split(path, " => ")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return path
}

func isMissingHead(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "bad revision") ||
		strings.Contains(lower, "unknown revision") ||
		strings.Contains(lower, "ambiguous argument 'head'") ||
		strings.Contains(lower, "fatal: bad revision") ||
		strings.Contains(lower, "fatal: ambiguous argument 'head'")
}

func countFileLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	if len(data) == 0 {
		return 0
	}
	count := bytes.Count(data, []byte{'\n'})
	// If file does not end with newline, count last line.
	if data[len(data)-1] != '\n' {
		count++
	}
	return count
}
