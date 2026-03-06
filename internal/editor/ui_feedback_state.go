package editor

import "time"

type editorUIFeedbackState struct {
	layoutName          string
	lastKeyCombo        string
	copiedMessageTime   time.Time
	notificationMessage string
	notificationStarted time.Time
}
