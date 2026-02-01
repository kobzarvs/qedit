package integrations

import (
	"os/exec"
	"strings"
)

// MacClipboard implements Clipboard using pbcopy/pbpaste.
type MacClipboard struct{}

func (MacClipboard) Write(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (MacClipboard) Read() (string, error) {
	cmd := exec.Command("pbpaste")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
