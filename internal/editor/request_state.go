package editor

type RuntimeRequestKind string

const (
	RuntimeRequestShowBranchPicker    RuntimeRequestKind = "show_branch_picker"
	RuntimeRequestSelectBranch        RuntimeRequestKind = "select_branch"
	RuntimeRequestShowWorktrees       RuntimeRequestKind = "show_worktrees"
	RuntimeRequestSwitchWorktree      RuntimeRequestKind = "switch_worktree"
	RuntimeRequestOpenFile            RuntimeRequestKind = "open_file"
	RuntimeRequestBufferSwitched      RuntimeRequestKind = "buffer_switched"
	RuntimeRequestSaveFile            RuntimeRequestKind = "save_file"
	RuntimeRequestReloadFile          RuntimeRequestKind = "reload_file"
	RuntimeRequestFormatBuffer        RuntimeRequestKind = "format_buffer"
	RuntimeRequestPersistAutoReload   RuntimeRequestKind = "persist_auto_reload"
	RuntimeRequestPersistSidebarWidth RuntimeRequestKind = "persist_sidebar_width"
)

type RuntimeRequest struct {
	Kind      RuntimeRequestKind
	Path      string
	Content   string
	Value     string
	PrevValue string
	Line      int
	Col       int
	Bool      bool
	PrevBool  bool
	Force     bool
	QuitAfter bool
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
