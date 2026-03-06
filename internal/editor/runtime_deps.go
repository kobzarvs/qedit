package editor

type editorRuntimeDeps struct {
	systemClipboard Clipboard
	formatter       Formatter
	merger          Merger
	terminalZoomer  TerminalZoomer
	sessionStore    SessionStore
	historyStore    HistoryStore
	fileStore       FileStore
	undoStore       UndoStore
	languageRuntime LanguageRuntime
}
