package editor

type editorHighlightState struct {
	spans map[int][]HighlightSpan
	start int
	end   int
}
