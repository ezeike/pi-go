// Package forgecontroller provides an ACP client for Forge Controller.
//
// Forge Controller is itself an ACP agent that exposes orchestration commands
// over stdio. The shared client package owns the ACP lifecycle; this adapter
// only selects the controller command.
package forgecontroller

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"

	shared "github.com/dimetron/pi-go/internal/acp"
	client "github.com/dimetron/pi-go/internal/acp/client"
)

// DefaultCommand is the installed Forge Controller command.
var DefaultCommand = []string{"forge-controller"}

// EnvCommand overrides the controller command. It accepts a command followed
// by whitespace-separated arguments, for example:
//
//	PI_ACP_FORGE_CONTROLLER_CMD="/opt/forge/bin/forge-controller"
const EnvCommand = "PI_ACP_FORGE_CONTROLLER_CMD"

const rpcTimeout = 60 * time.Second

// Runner launches Forge Controller and manages its ACP client-side session.
type Runner struct {
	ClientInfo acp.Implementation
	Logger     *slog.Logger
	ExtraEnv   []string
}

// Start launches one Forge Controller ACP prompt turn.
func (r Runner) Start(ctx context.Context, req shared.RunRequest) (*client.RunningSession, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	command, err := resolveCommand()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	runner := client.Runner{ClientInfo: r.ClientInfo, Logger: r.Logger, ExtraEnv: r.ExtraEnv}
	req.Command = command
	req.RPCTimeout = rpcTimeout
	return runner.StartCommand(ctx, cmd, req)
}

func resolveCommand() ([]string, error) {
	command := strings.Fields(os.Getenv(EnvCommand))
	if len(command) == 0 {
		command = append([]string(nil), DefaultCommand...)
	}
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return nil, fmt.Errorf("%s is empty", EnvCommand)
	}
	return command, nil
}
