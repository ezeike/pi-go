package forgecontroller

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	shared "github.com/dimetron/pi-go/internal/acp"
	client "github.com/dimetron/pi-go/internal/acp/client"
)

// CommandKind is a Forge Controller command accepted by its ACP agent.
type CommandKind string

const (
	CommandStatus     CommandKind = "status"
	CommandPause      CommandKind = "pause"
	CommandResume     CommandKind = "resume"
	CommandResumeTask CommandKind = "resume_task"
)

// Command is the structured form of Forge Controller's slash commands.
type Command struct {
	Kind      CommandKind
	Component string
	TaskID    string
}

func (c Command) text() (string, error) {
	switch c.Kind {
	case CommandStatus:
		return "/forge-status", nil
	case CommandPause:
		if strings.TrimSpace(c.Component) == "" {
			return "", fmt.Errorf("forge pause requires a component")
		}
		return "/forge-pause " + c.Component, nil
	case CommandResume:
		if strings.TrimSpace(c.Component) == "" {
			return "", fmt.Errorf("forge resume requires a component")
		}
		return "/forge-resume " + c.Component, nil
	case CommandResumeTask:
		if strings.TrimSpace(c.TaskID) == "" {
			return "", fmt.Errorf("forge resume_task requires a task_id")
		}
		return "/forge-resume-task " + c.TaskID, nil
	default:
		return "", fmt.Errorf("unknown Forge Controller command %q", c.Kind)
	}
}

// Manager keeps one persistent Forge Controller ACP session per project.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*client.PersistentSession
	start    func(context.Context, string, func(shared.Event)) (*client.PersistentSession, error)
}

// NewManager creates a project-scoped Forge Controller session manager.
func NewManager() *Manager {
	m := &Manager{sessions: make(map[string]*client.PersistentSession)}
	m.start = m.startSession
	return m
}

// Run sends one command to the project's persistent controller session.
func (m *Manager) Run(ctx context.Context, cwd string, command Command, onEvent func(shared.Event)) (shared.RunResult, error) {
	text, err := command.text()
	if err != nil {
		return shared.RunResult{}, err
	}
	key, err := projectKey(cwd)
	if err != nil {
		return shared.RunResult{}, err
	}
	m.mu.Lock()
	session := m.sessions[key]
	if session == nil {
		session, err = m.start(ctx, key, onEvent)
		if err == nil {
			m.sessions[key] = session
		}
	}
	m.mu.Unlock()
	if err != nil {
		return shared.RunResult{}, err
	}
	return session.Prompt(ctx, text)
}

// CloseProject releases the controller process for one project.
func (m *Manager) CloseProject(cwd string) error {
	key, err := projectKey(cwd)
	if err != nil {
		return err
	}
	m.mu.Lock()
	session := m.sessions[key]
	delete(m.sessions, key)
	m.mu.Unlock()
	if session == nil {
		return nil
	}
	return session.Close()
}

// CloseAll releases every managed controller process.
func (m *Manager) CloseAll() error {
	m.mu.Lock()
	sessions := m.sessions
	m.sessions = make(map[string]*client.PersistentSession)
	m.mu.Unlock()
	var first error
	for _, session := range sessions {
		if err := session.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *Manager) startSession(ctx context.Context, cwd string, onEvent func(shared.Event)) (*client.PersistentSession, error) {
	command, err := resolveCommand()
	if err != nil {
		return nil, err
	}
	return client.StartPersistentSession(ctx, client.PersistentRequest{Command: command, CWD: cwd, RPCTimeout: rpcTimeout}, onEvent)
}

func projectKey(cwd string) (string, error) {
	if strings.TrimSpace(cwd) == "" {
		return "", fmt.Errorf("Forge Controller project directory is required")
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve Forge Controller project directory: %w", err)
	}
	return filepath.Clean(abs), nil
}
