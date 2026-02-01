package editor

// Options configures editor behavior without coupling to external config packages.
type Options struct {
	TabWidth             int
	LineNumbers          string
	GitBranchSymbol      string
	SidebarWidth         string
	SidebarMinWidth      int
	SidebarMaxWidth      string
	SidebarCloseOnSelect bool

	KeymapNormal map[string]string
	KeymapInsert map[string]string

	CmdHistoryPath    string
	SearchHistoryPath string

	SessionStore SessionStore
}
