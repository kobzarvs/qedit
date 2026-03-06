package editor

import (
	"strings"
	"time"
)

const (
	notificationHoldDuration = 1500 * time.Millisecond
	notificationFadeDuration = 500 * time.Millisecond
)

// Notify shows a transient notification at the top of the screen.
func (e *Editor) Notify(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	e.ui.notificationMessage = msg
	e.ui.notificationStarted = time.Now()
}

func (e *Editor) notificationActive(now time.Time) bool {
	if e.ui.notificationMessage == "" {
		return false
	}
	elapsed := now.Sub(e.ui.notificationStarted)
	if elapsed >= notificationHoldDuration+notificationFadeDuration {
		e.ui.notificationMessage = ""
		return false
	}
	return true
}

func (e *Editor) notificationStyle(now time.Time) Style {
	if e.ui.notificationMessage == "" {
		return e.styleNotificationBright
	}
	elapsed := now.Sub(e.ui.notificationStarted)
	if elapsed < notificationHoldDuration || len(e.notificationFadeStyles) == 0 {
		return e.styleNotificationBright
	}
	progress := float64(elapsed-notificationHoldDuration) / float64(notificationFadeDuration)
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	if len(e.notificationFadeStyles) == 1 {
		return e.notificationFadeStyles[0]
	}
	idx := int(progress * float64(len(e.notificationFadeStyles)-1))
	if idx < 0 {
		idx = 0
	} else if idx >= len(e.notificationFadeStyles) {
		idx = len(e.notificationFadeStyles) - 1
	}
	return e.notificationFadeStyles[idx]
}
