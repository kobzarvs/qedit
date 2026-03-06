package editor

type editorCommandLineState struct {
	text          []rune
	cursor        int
	history       []string
	historyIndex  int
	historyPrefix string
	historyPath   string
}
