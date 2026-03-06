package editor

import "time"

type editorInteractionState struct {
	freeScroll         bool
	lastScrollTime     time.Time
	resizeDragging     bool
	resizeTarget       resizeTarget
	resizeSidebarWidth string
}
