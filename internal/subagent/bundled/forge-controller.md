---
name: forge-controller
description: Forge Controller ACP agent — orchestrate Forge tasks with /forge-status, /forge-pause, /forge-resume, and /forge-resume-task
role: default
worktree: false
tools: read, grep, find, tree, ls, bash
---

Forge Controller is an ACP command agent. Send its commands unchanged, beginning with one of:

- `/forge-status`
- `/forge-pause <component>`
- `/forge-resume <component>`
- `/forge-resume-task <task-id>`

Install its `forge-controller` binary on `PATH`. To use another executable or
arguments, set `PI_ACP_FORGE_CONTROLLER_CMD`, for example:

```sh
export PI_ACP_FORGE_CONTROLLER_CMD="$HOME/src/agents/forge-controller/forge-controller"
```

Pi-go launches the controller as an ACP subprocess in the current project
directory and streams the controller's status messages back to the caller.
