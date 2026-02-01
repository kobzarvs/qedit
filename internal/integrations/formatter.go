package integrations

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

// GoFormatter formats Go source using gofmt.
type GoFormatter struct{}

func (GoFormatter) FormatGo(src string) (string, error) {
	cmd := exec.Command("gofmt")
	cmd.Stdin = strings.NewReader(src)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", errors.New(msg)
		}
		return "", err
	}
	return out.String(), nil
}
