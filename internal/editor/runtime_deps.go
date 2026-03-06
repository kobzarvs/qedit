package editor

type editorRuntimeDeps struct {
	systemClipboard        Clipboard
	formatter              Formatter
	terminalZoomer         TerminalZoomer
	sessionStore           SessionStore
	historyStore           HistoryStore
	fileStore              FileStore
	nodeStackFunc          NodeStackFunc
	lspGotoFunc            LSPGotoFunc
	highlightRangeFunc     HighlightRangeFunc
	autoReloadConfigHook   func(enabled bool) error
	sidebarWidthConfigHook func(width string) error
}
