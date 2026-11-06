package a2atool

import (
	"strings"
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestCogBAppendPartsText(t *testing.T) {
	var sb strings.Builder
	appendPartsText(&sb, a2a.ContentParts{
		a2a.NewTextPart("one"),
		a2a.NewTextPart(""),
		a2a.NewTextPart("two"),
	})
	if sb.String() != "onetwo" {
		t.Errorf("got %q, want %q", sb.String(), "onetwo")
	}

	var empty strings.Builder
	appendPartsText(&empty, nil)
	if empty.String() != "" {
		t.Errorf("nil parts wrote %q", empty.String())
	}
}

func TestCogBAppendStreamEvent(t *testing.T) {
	cases := []struct {
		name     string
		event    a2a.Event
		wantText string
		wantEnd  bool
	}{
		{
			name:     "message is terminal",
			event:    a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("hello"), a2a.NewTextPart("")),
			wantText: "hello",
			wantEnd:  true,
		},
		{
			name: "task is terminal",
			event: &a2a.Task{History: []*a2a.Message{
				a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("done")),
			}},
			wantText: "done",
			wantEnd:  true,
		},
		{
			name:     "empty task is terminal and silent",
			event:    &a2a.Task{},
			wantText: "",
			wantEnd:  true,
		},
		{
			name:     "terminal status ends the stream",
			event:    &a2a.TaskStatusUpdateEvent{Status: a2a.TaskStatus{State: a2a.TaskStateCompleted}},
			wantText: "",
			wantEnd:  true,
		},
		{
			name:     "working status continues the stream",
			event:    &a2a.TaskStatusUpdateEvent{Status: a2a.TaskStatus{State: a2a.TaskStateWorking}},
			wantText: "",
			wantEnd:  false,
		},
		{
			name: "artifact accumulates and continues",
			event: &a2a.TaskArtifactUpdateEvent{Artifact: &a2a.Artifact{
				Parts: a2a.ContentParts{a2a.NewTextPart("chunk"), a2a.NewTextPart("")},
			}},
			wantText: "chunk",
			wantEnd:  false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := newStreamTextCollector()
			end := c.appendStreamEvent(tt.event)
			if end != tt.wantEnd {
				t.Errorf("terminal = %v, want %v", end, tt.wantEnd)
			}
			if c.String() != tt.wantText {
				t.Errorf("text = %q, want %q", c.String(), tt.wantText)
			}
		})
	}
}

func TestCogBAppendStreamEventAccumulates(t *testing.T) {
	// Two artifact chunks then a terminal status: the builder keeps both.
	c := newStreamTextCollector()
	for _, chunk := range []string{"a", "b"} {
		e := &a2a.TaskArtifactUpdateEvent{Artifact: &a2a.Artifact{
			Parts: a2a.ContentParts{a2a.NewTextPart(chunk)},
		}}
		if c.appendStreamEvent(e) {
			t.Fatal("artifact event reported the stream as ended")
		}
	}
	if !c.appendStreamEvent(&a2a.TaskStatusUpdateEvent{
		Status: a2a.TaskStatus{State: a2a.TaskStateFailed},
	}) {
		t.Fatal("failed status did not end the stream")
	}
	if c.String() != "ab" {
		t.Errorf("accumulated %q, want %q", c.String(), "ab")
	}
}
