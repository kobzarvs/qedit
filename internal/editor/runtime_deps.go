package editor

type editorRuntimeDeps struct {
	systemClipboard Clipboard
	formatter       Formatter
	merger          Merger
	terminalZoomer  TerminalZoomer
	persistence     PersistenceRuntime
	fileStore       FileStore
	languageRuntime LanguageRuntime
}
