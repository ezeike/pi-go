package tools

import (
	"context"
	"errors"
	"os/exec"
)

// CommandFactory builds the command the tools run for an external process: the
// bash tool's shell and the git tools' git.
//
// The default runs the process directly on the host. An embedder that must not
// execute agent-driven processes on the host installs a factory whose commands
// are wrapped by its own sandbox (bwrap, docker, ...). File access is a
// separate matter: it stays on the [Sandbox]'s os.Root.
//
// The contract a factory must meet:
//
//   - dir is the directory the process should run in. The factory decides how
//     it maps into its own environment.
//   - The returned command is already bound to ctx (build it with
//     exec.CommandContext), so cancelling ctx stops it.
//   - The factory owns process isolation. A command it returns is never passed
//     to procs.Isolate, because Isolate replaces SysProcAttr, Cancel and
//     WaitDelay, which a sandbox wrapper may set on purpose. A factory whose
//     processes spawn a tree must install a Cancel that stops the whole tree.
//   - Stdin, Stdout and Stderr are left unset; the caller wires them.
//   - The bash tool asks for `bash -c <script>`, so the environment behind the
//     factory must provide bash.
type CommandFactory func(ctx context.Context, dir, name string, args ...string) (*exec.Cmd, error)

// SetCommandFactory makes the sandbox's tools start external processes through
// f instead of directly on the host. A nil f restores the host default.
//
// Call it before the sandbox is handed to the tools: it is not safe to change
// while a tool call is in flight.
func (s *Sandbox) SetCommandFactory(f CommandFactory) {
	s.commandFactory = f
}

// hasCommandFactory reports whether an embedder replaced the host default.
func (s *Sandbox) hasCommandFactory() bool {
	return s.commandFactory != nil
}

// newCommand builds an external-process command for the tools, through the
// installed factory or, absent one, directly on the host.
func (s *Sandbox) newCommand(ctx context.Context, dir, name string, args ...string) (*exec.Cmd, error) {
	if s.commandFactory == nil {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		return cmd, nil
	}
	cmd, err := s.commandFactory(ctx, dir, name, args...)
	if err != nil {
		return nil, err
	}
	if cmd == nil {
		return nil, errors.New("command factory returned no command")
	}
	return cmd, nil
}
