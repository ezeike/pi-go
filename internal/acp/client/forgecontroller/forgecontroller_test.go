package forgecontroller

import (
	"context"
	"reflect"
	"testing"

	shared "github.com/dimetron/pi-go/internal/acp"
)

func TestResolveCommand(t *testing.T) {
	original := DefaultCommand
	DefaultCommand = []string{"forge-controller"}
	t.Cleanup(func() { DefaultCommand = original })

	t.Run("default", func(t *testing.T) {
		t.Setenv(EnvCommand, "")
		got, err := resolveCommand()
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"forge-controller"}; !reflect.DeepEqual(got, want) {
			t.Errorf("command = %q, want %q", got, want)
		}
	})

	t.Run("environment override", func(t *testing.T) {
		t.Setenv(EnvCommand, "/opt/forge/forge-controller --log-json")
		got, err := resolveCommand()
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"/opt/forge/forge-controller", "--log-json"}; !reflect.DeepEqual(got, want) {
			t.Errorf("command = %q, want %q", got, want)
		}
	})
}

func TestRunnerRejectsEmptyPrompt(t *testing.T) {
	_, err := (Runner{}).Start(context.Background(), shared.RunRequest{Command: []string{"ignored"}})
	if err == nil {
		t.Fatal("Start succeeded with an empty prompt")
	}
}
