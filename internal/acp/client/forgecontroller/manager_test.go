package forgecontroller

import "testing"

func TestCommandText(t *testing.T) {
	tests := []struct {
		name string
		cmd  Command
		want string
		fail bool
	}{
		{name: "status", cmd: Command{Kind: CommandStatus}, want: "/forge-status"},
		{name: "pause", cmd: Command{Kind: CommandPause, Component: "api"}, want: "/forge-pause api"},
		{name: "resume", cmd: Command{Kind: CommandResume, Component: "api"}, want: "/forge-resume api"},
		{name: "resume task", cmd: Command{Kind: CommandResumeTask, TaskID: "task-123"}, want: "/forge-resume-task task-123"},
		{name: "pause needs component", cmd: Command{Kind: CommandPause}, fail: true},
		{name: "unknown", cmd: Command{Kind: "restart"}, fail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cmd.text()
			if tt.fail {
				if err == nil {
					t.Fatal("text() error = nil")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("text() = %q, %v; want %q, nil", got, err, tt.want)
			}
		})
	}
}
