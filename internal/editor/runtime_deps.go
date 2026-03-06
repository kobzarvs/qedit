package editor

type editorRuntimeDeps struct {
	systemClipboard Clipboard
	terminalZoomer  TerminalZoomer
	workspace       WorkspaceRuntime
	persistence     PersistenceRuntime
	languageRuntime LanguageRuntime
	gitRuntime      GitRuntime
}
