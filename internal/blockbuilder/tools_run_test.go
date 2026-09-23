package blockbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	pluginv1 "github.com/machina/block-builder/sdk/gen/block-builder/plugin/v1"
	"github.com/machina/block-builder/sdk/sdktest"
	"google.golang.org/protobuf/encoding/protojson"
)

// ---------------------------------------------------------------------------
// Task 7a (impl-specs/toolset.md): start_run, get_run, get_manifest — 3
// plain-unary run tools, same round-trip + error-propagation shape as
// tools_spec_test.go's task 5 tools. Field sets confirmed directly against
// sdk/gen/block-builder/plugin/v1/run.pb.go (StartRunRequest, GetRunRequest,
// GetManifestRequest) — no OQ-A ambiguity remained for this trio.
// ---------------------------------------------------------------------------

func TestStartRunTool_RoundTrip(t *testing.T) {
	var gotReq *pluginv1.StartRunRequest
	run := &sdktest.FakeRunClient{
		StartRunFunc: func(_ context.Context, req *pluginv1.StartRunRequest) (*pluginv1.StartRunResponse, error) {
			gotReq = req
			return &pluginv1.StartRunResponse{}, nil
		},
	}
	tl, err := startRunTool(run)
	if err != nil {
		t.Fatalf("startRunTool: %v", err)
	}

	in := StartRunInput{
		PluginID:            "plug-1",
		Only:                []string{"swap"},
		DeterministicOnly:   true,
		SynthModel:          "strong-model",
		ReviewModel:         "review-model",
		VerifyOnly:          false,
		SamplingProfile:     "thinking-precise-coding",
		ImplSamplingProfile: "thinking-precise-coding",
	}
	_, callErr := runTool[StartRunInput, json.RawMessage](t, tl, in)
	if callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if gotReq == nil {
		t.Fatal("RunClient.StartRun was not called")
	}
	if gotReq.PluginId != in.PluginID ||
		len(gotReq.Only) != 1 || gotReq.Only[0] != "swap" ||
		gotReq.DeterministicOnly != in.DeterministicOnly ||
		gotReq.SynthModel != in.SynthModel ||
		gotReq.ReviewModel != in.ReviewModel ||
		gotReq.VerifyOnly != in.VerifyOnly ||
		gotReq.SamplingProfile != in.SamplingProfile ||
		gotReq.ImplSamplingProfile != in.ImplSamplingProfile {
		t.Fatalf("request seen by RunClient.StartRun = %+v, did not round-trip StartRunInput %+v", gotReq, in)
	}
}

func TestStartRunTool_PropagatesRPCError(t *testing.T) {
	wantErr := errors.New("boom")
	run := &sdktest.FakeRunClient{
		StartRunFunc: func(context.Context, *pluginv1.StartRunRequest) (*pluginv1.StartRunResponse, error) {
			return nil, wantErr
		},
	}
	tl, err := startRunTool(run)
	if err != nil {
		t.Fatalf("startRunTool: %v", err)
	}
	_, callErr := runTool[StartRunInput, json.RawMessage](t, tl, StartRunInput{PluginID: "plug-1"})
	if !errors.Is(callErr, wantErr) {
		t.Fatalf("callErr = %v, want it to wrap %v", callErr, wantErr)
	}
}

func TestGetRunTool_RoundTrip(t *testing.T) {
	var gotReq *pluginv1.GetRunRequest
	run := &sdktest.FakeRunClient{
		GetRunFunc: func(_ context.Context, req *pluginv1.GetRunRequest) (*pluginv1.GetRunResponse, error) {
			gotReq = req
			return &pluginv1.GetRunResponse{}, nil
		},
	}
	tl, err := getRunTool(run)
	if err != nil {
		t.Fatalf("getRunTool: %v", err)
	}
	out, callErr := runTool[GetRunInput, map[string]any](t, tl, GetRunInput{PluginID: "plug-1", RunID: "run-1"})
	if callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if gotReq == nil || gotReq.PluginId != "plug-1" || gotReq.RunId != "run-1" {
		t.Fatalf("request seen by RunClient.GetRun = %+v, want PluginId=plug-1 RunId=run-1", gotReq)
	}
	// Tool output is a plain map[string]any (see protoResult's doc comment in tool.go); re-encode
	// to bytes to reuse protojson for the decode-shape assertion.
	outBytes, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal tool output: %v", err)
	}
	var resp pluginv1.GetRunResponse
	if err := protojson.Unmarshal(outBytes, &resp); err != nil {
		t.Fatalf("tool output did not protojson-decode as GetRunResponse: %v", err)
	}
}

func TestGetRunTool_PropagatesRPCError(t *testing.T) {
	wantErr := errors.New("boom")
	run := &sdktest.FakeRunClient{
		GetRunFunc: func(context.Context, *pluginv1.GetRunRequest) (*pluginv1.GetRunResponse, error) {
			return nil, wantErr
		},
	}
	tl, err := getRunTool(run)
	if err != nil {
		t.Fatalf("getRunTool: %v", err)
	}
	_, callErr := runTool[GetRunInput, json.RawMessage](t, tl, GetRunInput{PluginID: "plug-1", RunID: "run-1"})
	if !errors.Is(callErr, wantErr) {
		t.Fatalf("callErr = %v, want it to wrap %v", callErr, wantErr)
	}
}

func TestGetManifestTool_RoundTrip(t *testing.T) {
	var gotReq *pluginv1.GetManifestRequest
	run := &sdktest.FakeRunClient{
		GetManifestFunc: func(_ context.Context, req *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
			gotReq = req
			return &pluginv1.GetManifestResponse{}, nil
		},
	}
	tl, err := getManifestTool(run)
	if err != nil {
		t.Fatalf("getManifestTool: %v", err)
	}
	_, callErr := runTool[GetManifestInput, json.RawMessage](t, tl, GetManifestInput{PluginID: "plug-1"})
	if callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if gotReq == nil || gotReq.PluginId != "plug-1" {
		t.Fatalf("request seen by RunClient.GetManifest = %+v, want PluginId=plug-1", gotReq)
	}
}

func TestGetManifestTool_PropagatesRPCError(t *testing.T) {
	wantErr := errors.New("boom")
	run := &sdktest.FakeRunClient{
		GetManifestFunc: func(context.Context, *pluginv1.GetManifestRequest) (*pluginv1.GetManifestResponse, error) {
			return nil, wantErr
		},
	}
	tl, err := getManifestTool(run)
	if err != nil {
		t.Fatalf("getManifestTool: %v", err)
	}
	_, callErr := runTool[GetManifestInput, json.RawMessage](t, tl, GetManifestInput{PluginID: "plug-1"})
	if !errors.Is(callErr, wantErr) {
		t.Fatalf("callErr = %v, want it to wrap %v", callErr, wantErr)
	}
}
