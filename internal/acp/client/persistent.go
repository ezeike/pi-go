package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"

	shared "github.com/dimetron/pi-go/internal/acp"
)

// PersistentSession owns an ACP subprocess and one ACP session across multiple
// prompt turns. Calls to Prompt are serialized because ACP sessions have one
// active turn at a time.
type PersistentSession struct {
	cmd    *exec.Cmd
	stdin  io.Closer
	conn   *acp.ClientSideConnection
	sid    acp.SessionId
	stderr *stderrBuffer

	turnMu  sync.Mutex
	mu      sync.Mutex
	closed  bool
	waited  bool
	onEvent func(shared.Event)
	current *persistentTurn

	stderrDone chan struct{}
	closeOnce  sync.Once
}

// PersistentRequest describes the command and initial ACP session settings.
type PersistentRequest struct {
	Command    []string
	CWD        string
	Env        []string
	RPCTimeout time.Duration
	ClientInfo acp.Implementation
}

// StartPersistentSession launches command, completes initialize and
// session/new once, and keeps stdin open for later Prompt calls.
func StartPersistentSession(ctx context.Context, req PersistentRequest, onEvent func(shared.Event)) (*PersistentSession, error) {
	if len(req.Command) == 0 || strings.TrimSpace(req.Command[0]) == "" {
		return nil, fmt.Errorf("persistent ACP command is required")
	}
	cmd := exec.CommandContext(context.Background(), req.Command[0], req.Command[1:]...)
	cmd.Env = append(os.Environ(), req.Env...)
	cmd.Dir = absDir(req.CWD)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("persistent ACP stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("persistent ACP stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("persistent ACP stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start persistent ACP subprocess: %w", err)
	}

	s := &PersistentSession{cmd: cmd, stdin: stdin, stderr: &stderrBuffer{}, onEvent: onEvent, stderrDone: make(chan struct{})}
	client := &persistentCallbackClient{session: s}
	s.conn = acp.NewClientSideConnection(client, stdin, stdout)
	s.conn.SetLogger(slog.New(slog.DiscardHandler))
	go s.captureStderr(stderr)

	initCtx, cancel := rpcContext(ctx, req.RPCTimeout)
	_, err = s.conn.Initialize(initCtx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersion(acp.ProtocolVersionNumber),
		ClientInfo:      &req.ClientInfo,
	})
	cancel()
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("initialize persistent ACP session: %w", err)
	}

	newCtx, cancel := rpcContext(ctx, req.RPCTimeout)
	created, err := s.conn.NewSession(newCtx, acp.NewSessionRequest{Cwd: absDir(req.CWD), McpServers: []acp.McpServer{}})
	cancel()
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("create persistent ACP session: %w", err)
	}
	s.sid = created.SessionId
	return s, nil
}

// SessionID returns the server-assigned ACP session identifier.
func (s *PersistentSession) SessionID() string { return string(s.sid) }

// Prompt runs one turn without closing the process or ACP session.
func (s *PersistentSession) Prompt(ctx context.Context, prompt string) (shared.RunResult, error) {
	if strings.TrimSpace(prompt) == "" {
		return shared.RunResult{}, fmt.Errorf("persistent ACP prompt is required")
	}
	s.turnMu.Lock()
	defer s.turnMu.Unlock()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return shared.RunResult{}, fmt.Errorf("persistent ACP session is closed")
	}
	turn := &persistentTurn{sessionID: string(s.sid)}
	s.current = turn
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.current = nil
		s.mu.Unlock()
	}()

	resp, err := s.conn.Prompt(ctx, acp.PromptRequest{SessionId: s.sid, Prompt: []acp.ContentBlock{acp.TextBlock(prompt)}})
	if err != nil {
		return shared.RunResult{}, fmt.Errorf("persistent ACP prompt: %w", err)
	}
	return shared.RunResult{Status: shared.StatusSuccess, Result: turn.result(), SessionID: string(s.sid), StopReason: string(resp.StopReason)}, nil
}

// Cancel asks the agent to stop its current turn but leaves the session alive.
func (s *PersistentSession) Cancel(ctx context.Context) error {
	return s.conn.Cancel(ctx, acp.CancelNotification{SessionId: s.sid})
}

// Close terminates the subprocess after closing its ACP transport. It is safe
// to call repeatedly.
func (s *PersistentSession) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		// Give a cooperative ACP agent the same lifecycle signal it receives
		// from other ACP clients before closing stdio. The bounded context keeps
		// a broken agent from making shutdown hang indefinitely.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = s.conn.CloseSession(ctx, acp.CloseSessionRequest{SessionId: s.sid})
		cancel()
		_ = s.stdin.Close()
	})
	<-s.stderrDone
	s.mu.Lock()
	if s.waited {
		s.mu.Unlock()
		return nil
	}
	s.waited = true
	s.mu.Unlock()
	if err := s.cmd.Wait(); err != nil {
		return fmt.Errorf("wait persistent ACP subprocess: %w", err)
	}
	return nil
}

type persistentTurn struct {
	mu        sync.Mutex
	sessionID string
	text      strings.Builder
}

func (t *persistentTurn) append(text string) { t.mu.Lock(); t.text.WriteString(text); t.mu.Unlock() }
func (t *persistentTurn) result() string     { t.mu.Lock(); defer t.mu.Unlock(); return t.text.String() }

func (s *PersistentSession) captureStderr(r io.Reader) {
	defer close(s.stderrDone)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		_, _ = s.stderr.Write([]byte(line + "\n"))
		if strings.TrimSpace(line) != "" {
			s.emit(shared.Event{Type: shared.EventTypeStderr, Content: line, SessionID: string(s.sid)})
		}
	}
}

func (s *PersistentSession) handleUpdate(notification acp.SessionNotification) {
	if chunk := notification.Update.AgentMessageChunk; chunk != nil {
		text := contentBlockText(chunk.Content)
		if strings.TrimSpace(text) != "" {
			s.mu.Lock()
			turn := s.current
			s.mu.Unlock()
			if turn != nil {
				turn.append(text)
			}
			s.emit(shared.Event{Type: shared.EventTypeMessage, Content: text, SessionID: string(notification.SessionId)})
		}
	}
	if thought := notification.Update.AgentThoughtChunk; thought != nil {
		if text := contentBlockText(thought.Content); strings.TrimSpace(text) != "" {
			s.emit(shared.Event{Type: shared.EventTypeProgress, Content: text, SessionID: string(notification.SessionId)})
		}
	}
}

func (s *PersistentSession) emit(event shared.Event) {
	if s.onEvent != nil {
		s.onEvent(event)
	}
}

func rpcContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return ctx, func() {}
}

type persistentCallbackClient struct{ session *PersistentSession }

func (c *persistentCallbackClient) ReadTextFile(context.Context, acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, fmt.Errorf("readTextFile not implemented")
}
func (c *persistentCallbackClient) WriteTextFile(context.Context, acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, fmt.Errorf("writeTextFile not implemented")
}
func (c *persistentCallbackClient) RequestPermission(_ context.Context, req acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: shared.AutoApproveOutcome(req)}, nil
}
func (c *persistentCallbackClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	c.session.handleUpdate(n)
	return nil
}
func (c *persistentCallbackClient) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalCreate)
}
func (c *persistentCallbackClient) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalKill)
}
func (c *persistentCallbackClient) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalOutput)
}
func (c *persistentCallbackClient) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalRelease)
}
func (c *persistentCallbackClient) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, acp.NewMethodNotFound(acp.ClientMethodTerminalWaitForExit)
}
