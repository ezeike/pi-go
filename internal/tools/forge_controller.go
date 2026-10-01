package tools

import (
	"fmt"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"

	shared "github.com/dimetron/pi-go/internal/acp"
	"github.com/dimetron/pi-go/internal/acp/client/forgecontroller"
)

// ForgeControllerInput is one structured command for the project-scoped Forge
// Controller session.
type ForgeControllerInput struct {
	Command   string `json:"command" jsonschema:"one of: status, pause, resume, resume_task"`
	Component string `json:"component,omitempty" jsonschema:"component name; required for pause and resume"`
	TaskID    string `json:"task_id,omitempty" jsonschema:"task ID; required for resume_task"`
}

type ForgeControllerOutput struct {
	SessionID  string `json:"session_id"`
	Output     string `json:"output,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
}

// NewForgeControllerTool creates the model-facing controller command tool.
func NewForgeControllerTool(manager *forgecontroller.Manager, cwd string) (tool.Tool, error) {
	if manager == nil {
		return nil, fmt.Errorf("Forge Controller manager is required")
	}
	handler := func(ctx agent.Context, input ForgeControllerInput) (ForgeControllerOutput, error) {
		result, err := manager.Run(resolveContext(ctx), cwd, forgecontroller.Command{
			Kind:      forgecontroller.CommandKind(strings.ToLower(strings.TrimSpace(input.Command))),
			Component: input.Component,
			TaskID:    input.TaskID,
		}, func(shared.Event) {})
		if err != nil {
			return ForgeControllerOutput{}, err
		}
		return ForgeControllerOutput{SessionID: result.SessionID, Output: result.Result, StopReason: result.StopReason}, nil
	}
	return newLongRunningTool("forge_controller", "Control the persistent Forge Controller session for this project. Use status, pause, resume, or resume_task; do not send raw slash commands.", handler)
}
