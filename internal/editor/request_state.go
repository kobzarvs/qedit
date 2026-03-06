package editor

type RuntimeRequestKind string

const (
	RuntimeRequestShowBranchPicker RuntimeRequestKind = "show_branch_picker"
	RuntimeRequestSelectBranch     RuntimeRequestKind = "select_branch"
	RuntimeRequestShowWorktrees    RuntimeRequestKind = "show_worktrees"
	RuntimeRequestSwitchWorktree   RuntimeRequestKind = "switch_worktree"
	RuntimeRequestOpenFile         RuntimeRequestKind = "open_file"
	RuntimeRequestBufferSwitched   RuntimeRequestKind = "buffer_switched"
)

type RuntimeRequest struct {
	Kind  RuntimeRequestKind
	Path  string
	Value string
	Line  int
	Col   int
}

type editorRequestState struct {
	queue []RuntimeRequest
}

func (e *Editor) enqueueRuntimeRequest(req RuntimeRequest) {
	e.requests.queue = append(e.requests.queue, req)
}

func (e *Editor) ConsumeRuntimeRequest() (RuntimeRequest, bool) {
	if len(e.requests.queue) == 0 {
		return RuntimeRequest{}, false
	}
	req := e.requests.queue[0]
	e.requests.queue = e.requests.queue[1:]
	return req, true
}
