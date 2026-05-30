package editor

import (
	"fmt"
	"strings"
	"testing"
)

func TestStatuslineShowsProfileAfterMode(t *testing.T) {
	tests := []struct {
		profile string
		want    string
	}{
		{BehaviorProfileVim, " NORMAL | Vim |"},
		{BehaviorProfileHelix, " NORMAL | Helix |"},
		{BehaviorProfileBasic, " BASIC | Basic |"},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			e := newSimulatedProfileEditor(tt.profile, "line")
			left := fmt.Sprintf(" %s | %s | line ", e.currentModeLabel(), e.currentProfileLabel())
			right := " Ln 1, Col 1"
			line := string(composeStatusLine(left, right, 80))
			if !strings.Contains(line, tt.want) {
				t.Fatalf("status line = %q, want substring %q", line, tt.want)
			}
		})
	}
}
