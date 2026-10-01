package tui

import (
	"os"
	"strings"
	"testing"
)

// The TUI owns the terminal: a stray write to stdout or stderr from the render
// path corrupts the alternate screen, and the panel renders on every frame.
func TestSubagentPanelFilesWriteNothingToTheTerminal(t *testing.T) {
	banned := []string{"fmt.Print", "fmt.Fprint", "log.Print", "log.Fatal", "log.Panic", "os.Stdout", "os.Stderr", "println(", "slog."}
	for _, name := range []string{"subagent_panel.go", "panel_layout.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, tok := range banned {
			if strings.Contains(string(src), tok) {
				t.Errorf("%s contains %q; the panel render path logs and prints nothing", name, tok)
			}
		}
	}
}
