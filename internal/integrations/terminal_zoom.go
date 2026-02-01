package integrations

import (
	"fmt"
	"os/exec"
)

// TerminalZoomer sends zoom steps to the terminal via AppleScript.
type TerminalZoomer struct{}

func (TerminalZoomer) ZoomStep(zoomIn bool) error {
	key := "+"
	if !zoomIn {
		key = "-"
	}
	script := fmt.Sprintf(`tell application "System Events" to keystroke "%s" using command down`, key)
	return exec.Command("osascript", "-e", script).Run()
}
