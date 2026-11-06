package piagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimetron/pi-go/internal/config"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// hostileLayout plants a sentinel everywhere a non-hermetic agent would look:
// the home directory, a .pi-go folder in the workspace, and an AGENTS.md in the
// workspace's parent. It returns the home, the workspace and the pinned config
// directory.
func hostileLayout(t *testing.T) (home, workDir, configDir string) {
	t.Helper()
	home = isolate(t)
	parent := t.TempDir()
	workDir = filepath.Join(parent, "work")
	configDir = t.TempDir()

	mcp := `{"mcpServers":{"%s":{"command":"/bin/true"}}}`
	writeFile(t, filepath.Join(home, ".pi-go", "mcp.json"), strings.Replace(mcp, "%s", "home-sentinel", 1))
	writeFile(t, filepath.Join(home, ".pi-go", "AGENTS.md"), "HOME-AGENTS-SENTINEL")
	writeFile(t, filepath.Join(home, ".pi-go", "skills", "home-skill", "SKILL.md"),
		"---\nname: home-skill\ndescription: HOME-SKILL-SENTINEL\n---\nbody\n")
	writeFile(t, filepath.Join(workDir, ".pi-go", "mcp.json"), strings.Replace(mcp, "%s", "workspace-sentinel", 1))
	writeFile(t, filepath.Join(parent, "AGENTS.md"), "PARENT-AGENTS-SENTINEL")
	writeFile(t, filepath.Join(workDir, "AGENTS.md"), "WORKSPACE-AGENTS")

	writeFile(t, filepath.Join(configDir, "mcp.json"), strings.Replace(mcp, "%s", "pinned", 1))
	writeFile(t, filepath.Join(configDir, "skills", "pinned-skill", "SKILL.md"),
		"---\nname: pinned-skill\ndescription: PINNED-SKILL\n---\nbody\n")
	return home, workDir, configDir
}

func TestConfigDirLoadsOnlyThePinnedConfig(t *testing.T) {
	_, workDir, configDir := hostileLayout(t)

	cfg, err := config.LoadDir(configDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	var names []string
	if cfg.MCP != nil {
		for _, s := range cfg.MCP.Servers {
			names = append(names, s.Name)
		}
	}
	if len(names) != 1 || names[0] != "pinned" {
		t.Errorf("MCP servers = %v, want only the pinned one (home and workspace configs must not load)", names)
	}

	// The unpinned loader on the same layout does see the planted servers,
	// which is what makes the assertion above meaningful.
	loose, err := config.LoadFrom(workDir)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if loose.MCP == nil || len(loose.MCP.Servers) == 0 || loose.MCP.Servers[0].Name != "workspace-sentinel" {
		t.Fatalf("test layout is not hostile: LoadFrom saw %+v", loose.MCP)
	}
}

func TestConfigDirInstructionIgnoresHostAndAncestors(t *testing.T) {
	_, workDir, configDir := hostileLayout(t)

	o := optionsFor(t, WithWorkingDir(workDir), WithConfigDir(configDir))
	got := buildInstruction(o, workDir)

	for _, leaked := range []string{"HOME-AGENTS-SENTINEL", "PARENT-AGENTS-SENTINEL", "HOME-SKILL-SENTINEL"} {
		if strings.Contains(got, leaked) {
			t.Errorf("instruction leaked %s from outside the config dir and workspace", leaked)
		}
	}
	for _, want := range []string{"WORKSPACE-AGENTS", "PINNED-SKILL"} {
		if !strings.Contains(got, want) {
			t.Errorf("instruction lacks %s", want)
		}
	}
}

func TestConfigDirSandboxExcludesHomeState(t *testing.T) {
	home, workDir, _ := hostileLayout(t)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sb, err := newSandbox(workDir, nil, false)
	if err != nil {
		t.Fatalf("newSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Close() })

	if _, err := sb.ReadFile(filepath.Join(home, ".pi-go", "AGENTS.md")); err == nil {
		t.Error("a hermetic sandbox could read ~/.pi-go")
	}

	open, err := newSandbox(workDir, nil, true)
	if err != nil {
		t.Fatalf("newSandbox: %v", err)
	}
	t.Cleanup(func() { _ = open.Close() })
	if _, err := open.ReadFile(filepath.Join(home, ".pi-go", "AGENTS.md")); err != nil {
		t.Errorf("control: the default sandbox cannot read ~/.pi-go: %v", err)
	}
}

func TestConfigDirRootsDefaultState(t *testing.T) {
	configDir := t.TempDir()

	got, err := resolveSessionDirIn("", configDir)
	if err != nil || got != filepath.Join(configDir, "sessions") {
		t.Errorf("session dir = %q, %v", got, err)
	}
	if got, _ := resolveSessionDirIn("/explicit", configDir); got != "/explicit" {
		t.Errorf("explicit session dir overridden: %q", got)
	}

	if got := buildToolsetsIn(config.Config{}, false); len(got) != 0 {
		t.Errorf("env-configured toolsets present in hermetic mode: %d", len(got))
	}
}
