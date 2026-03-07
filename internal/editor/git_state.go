package editor

import "time"

type editorGitState struct {
	branch          string
	mainBranch      string
	branchSymbol    string
	root            string
	changes         []GitFileChange
	changeHunks     []GitChangeHunk
	diffHighlight   *GitChangeHunk
	diffPreview     gitDiffPreviewState
	pendingDiffJump bool
	changesUpdated  time.Time
	changesVersion  uint64
}
