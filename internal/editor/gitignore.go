package editor

import (
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type gitignorePattern struct {
	Pattern   string
	IsDir     bool
	Negation  bool
	matchBase bool
	anchored  bool
	regex     *regexp.Regexp
}

func findGitRoot(fs FileStore, dir string) string {
	if fs == nil {
		return ""
	}
	start := dir
	info, err := fs.Stat(start)
	if err != nil {
		return ""
	}
	if !info.IsDir {
		start = filepath.Dir(start)
	}
	for {
		gitPath := filepath.Join(start, ".git")
		if _, err := fs.Stat(gitPath); err == nil {
			return start
		}
		parent := filepath.Dir(start)
		if parent == start {
			break
		}
		start = parent
	}
	return ""
}

func loadGitignore(fs FileStore, gitRoot string) []gitignorePattern {
	if fs == nil || gitRoot == "" {
		return nil
	}
	filePath := filepath.Join(gitRoot, ".gitignore")
	data, err := fs.Read(filePath)
	if err != nil {
		return nil
	}

	var patterns []gitignorePattern
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pattern, ok := parseGitignorePattern(line)
		if !ok {
			continue
		}
		patterns = append(patterns, pattern)
	}
	return patterns
}

func parseGitignorePattern(line string) (gitignorePattern, bool) {
	negation := false
	if strings.HasPrefix(line, "!") {
		negation = true
		line = strings.TrimSpace(line[1:])
	}
	if line == "" {
		return gitignorePattern{}, false
	}

	isDir := strings.HasSuffix(line, "/")
	if isDir {
		line = strings.TrimSuffix(line, "/")
	}

	anchored := strings.HasPrefix(line, "/")
	if anchored {
		line = strings.TrimPrefix(line, "/")
	}

	line = filepath.ToSlash(line)
	matchBase := !strings.Contains(line, "/")

	re := compileGitignoreRegex(line, anchored, matchBase)
	if re == nil {
		return gitignorePattern{}, false
	}
	return gitignorePattern{
		Pattern:   line,
		IsDir:     isDir,
		Negation:  negation,
		matchBase: matchBase,
		anchored:  anchored,
		regex:     re,
	}, true
}

func compileGitignoreRegex(pattern string, anchored bool, matchBase bool) *regexp.Regexp {
	if pattern == "" {
		return nil
	}
	regex := globToRegex(pattern)
	if matchBase || anchored {
		re, err := regexp.Compile(regex)
		if err != nil {
			return nil
		}
		return re
	}
	core := strings.TrimPrefix(regex, "^")
	core = strings.TrimSuffix(core, "$")
	regex = "^(?:.*/)?" + core + "(?:/.*)?$"
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil
	}
	return re
}

func globToRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString(".")
		default:
			ch := pattern[i]
			if strings.ContainsRune(`.+()|[]{}^$\\`, rune(ch)) {
				b.WriteByte('\\')
			}
			b.WriteByte(ch)
		}
	}
	b.WriteString("$")
	return b.String()
}

func matchesGitignore(patterns []gitignorePattern, relPath string, isDir bool) bool {
	relPath = filepath.ToSlash(relPath)
	matched := false
	for _, p := range patterns {
		if p.IsDir && !isDir {
			continue
		}
		target := relPath
		if p.matchBase {
			target = path.Base(relPath)
		}
		if p.regex == nil {
			if p.matchBase && plainPatternMatch(p.Pattern, target) {
				if p.Negation {
					matched = false
				} else {
					matched = true
				}
			}
			continue
		}
		if p.regex.MatchString(target) || (p.matchBase && plainPatternMatch(p.Pattern, target)) {
			if p.Negation {
				matched = false
			} else {
				matched = true
			}
		}
	}
	return matched
}

func plainPatternMatch(pattern, target string) bool {
	if pattern == "" || target == "" {
		return false
	}
	if strings.ContainsAny(pattern, "*?[]") {
		return false
	}
	return pattern == target
}
