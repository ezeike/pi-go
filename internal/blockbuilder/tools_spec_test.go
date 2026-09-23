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
// Task 5 (impl-specs/toolset.md): 4 plain-unary spec tools — accept_spec,
// validate_spec, get_spec, list_primitives. Each wraps one sdk.SpecClient
// method: build the request from the tool's typed input, call the client,
// return protojson.Marshal(resp) as the tool's output. RPC errors propagate
// unwrapped (spec.md "Error handling": no bespoke formatting).
// ---------------------------------------------------------------------------

func TestAcceptSpecTool_RoundTrip(t *testing.T) {
	var gotReq *pluginv1.AcceptSpecRequest
	spec := &sdktest.FakeSpecClient{
		AcceptSpecFunc: func(_ context.Context, req *pluginv1.AcceptSpecRequest) (*pluginv1.AcceptSpecResponse, error) {
			gotReq = req
			return &pluginv1.AcceptSpecResponse{}, nil
		},
	}
	tl, err := acceptSpecTool(spec)
	if err != nil {
		t.Fatalf("acceptSpecTool: %v", err)
	}

	out, callErr := runTool[AcceptSpecInput, map[string]any](t, tl, AcceptSpecInput{
		PluginID:       "plug-1",
		ConversationID: "conv-1",
	})
	if callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if gotReq == nil || gotReq.PluginId != "plug-1" || gotReq.ConversationId != "conv-1" {
		t.Fatalf("request seen by SpecClient.AcceptSpec = %+v, want PluginId=plug-1 ConversationId=conv-1", gotReq)
	}
	// Tool output is a plain map[string]any (see protoResult's doc comment in tool.go for why —
	// json.RawMessage's inferred output schema made functiontool.New wrap every response under
	// an extra "result" key instead of returning the proto response's fields directly). Re-encode
	// it to bytes to reuse protojson for the decode-shape assertion.
	outBytes, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal tool output: %v", err)
	}
	var resp pluginv1.AcceptSpecResponse
	if err := protojson.Unmarshal(outBytes, &resp); err != nil {
		t.Fatalf("tool output did not protojson-decode as AcceptSpecResponse: %v", err)
	}
}

func TestAcceptSpecTool_PropagatesRPCError(t *testing.T) {
	wantErr := errors.New("boom")
	spec := &sdktest.FakeSpecClient{
		AcceptSpecFunc: func(context.Context, *pluginv1.AcceptSpecRequest) (*pluginv1.AcceptSpecResponse, error) {
			return nil, wantErr
		},
	}
	tl, err := acceptSpecTool(spec)
	if err != nil {
		t.Fatalf("acceptSpecTool: %v", err)
	}
	_, callErr := runTool[AcceptSpecInput, json.RawMessage](t, tl, AcceptSpecInput{PluginID: "plug-1"})
	if callErr == nil {
		t.Fatal("runTool: got nil error, want the RPC error to propagate")
	}
	if !errors.Is(callErr, wantErr) {
		t.Fatalf("callErr = %v, want it to wrap %v", callErr, wantErr)
	}
}

func TestValidateSpecTool_RoundTrip(t *testing.T) {
	var gotReq *pluginv1.ValidateSpecRequest
	spec := &sdktest.FakeSpecClient{
		ValidateSpecFunc: func(_ context.Context, req *pluginv1.ValidateSpecRequest) (*pluginv1.ValidateSpecResponse, error) {
			gotReq = req
			return &pluginv1.ValidateSpecResponse{}, nil
		},
	}
	tl, err := validateSpecTool(spec)
	if err != nil {
		t.Fatalf("validateSpecTool: %v", err)
	}

	_, callErr := runTool[ValidateSpecInput, json.RawMessage](t, tl, ValidateSpecInput{PluginID: "plug-1"})
	if callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if gotReq == nil || gotReq.PluginId != "plug-1" {
		t.Fatalf("request seen by SpecClient.ValidateSpec = %+v, want PluginId=plug-1", gotReq)
	}
}

func TestValidateSpecTool_UnsetSpecValidatesCommitted(t *testing.T) {
	// ValidateSpecRequest.Spec is optional (*PluginSpec): unset validates the
	// currently-committed spec (spec.pb.go doc comment on the field). Confirm
	// an ValidateSpecInput with no Spec produces a request with a nil Spec,
	// not a zero-value empty message that would be misread server-side as
	// "validate this (empty) draft."
	var gotReq *pluginv1.ValidateSpecRequest
	spec := &sdktest.FakeSpecClient{
		ValidateSpecFunc: func(_ context.Context, req *pluginv1.ValidateSpecRequest) (*pluginv1.ValidateSpecResponse, error) {
			gotReq = req
			return &pluginv1.ValidateSpecResponse{}, nil
		},
	}
	tl, err := validateSpecTool(spec)
	if err != nil {
		t.Fatalf("validateSpecTool: %v", err)
	}
	if _, callErr := runTool[ValidateSpecInput, json.RawMessage](t, tl, ValidateSpecInput{PluginID: "plug-1"}); callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if gotReq.Spec != nil {
		t.Fatalf("request.Spec = %+v, want nil when ValidateSpecInput.Spec is omitted", gotReq.Spec)
	}
}

func TestValidateSpecTool_PropagatesRPCError(t *testing.T) {
	wantErr := errors.New("boom")
	spec := &sdktest.FakeSpecClient{
		ValidateSpecFunc: func(context.Context, *pluginv1.ValidateSpecRequest) (*pluginv1.ValidateSpecResponse, error) {
			return nil, wantErr
		},
	}
	tl, err := validateSpecTool(spec)
	if err != nil {
		t.Fatalf("validateSpecTool: %v", err)
	}
	_, callErr := runTool[ValidateSpecInput, json.RawMessage](t, tl, ValidateSpecInput{PluginID: "plug-1"})
	if !errors.Is(callErr, wantErr) {
		t.Fatalf("callErr = %v, want it to wrap %v", callErr, wantErr)
	}
}

func TestGetSpecTool_RoundTrip(t *testing.T) {
	var gotReq *pluginv1.GetSpecRequest
	spec := &sdktest.FakeSpecClient{
		GetSpecFunc: func(_ context.Context, req *pluginv1.GetSpecRequest) (*pluginv1.GetSpecResponse, error) {
			gotReq = req
			return &pluginv1.GetSpecResponse{}, nil
		},
	}
	tl, err := getSpecTool(spec)
	if err != nil {
		t.Fatalf("getSpecTool: %v", err)
	}
	_, callErr := runTool[GetSpecInput, json.RawMessage](t, tl, GetSpecInput{PluginID: "plug-1"})
	if callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if gotReq == nil || gotReq.PluginId != "plug-1" {
		t.Fatalf("request seen by SpecClient.GetSpec = %+v, want PluginId=plug-1", gotReq)
	}
}

func TestGetSpecTool_PropagatesRPCError(t *testing.T) {
	wantErr := errors.New("boom")
	spec := &sdktest.FakeSpecClient{
		GetSpecFunc: func(context.Context, *pluginv1.GetSpecRequest) (*pluginv1.GetSpecResponse, error) {
			return nil, wantErr
		},
	}
	tl, err := getSpecTool(spec)
	if err != nil {
		t.Fatalf("getSpecTool: %v", err)
	}
	_, callErr := runTool[GetSpecInput, json.RawMessage](t, tl, GetSpecInput{PluginID: "plug-1"})
	if !errors.Is(callErr, wantErr) {
		t.Fatalf("callErr = %v, want it to wrap %v", callErr, wantErr)
	}
}

// ListPrimitivesRequest is confirmed empty (sdk/gen/block-builder/plugin/v1/spec.pb.go:
// `type ListPrimitivesRequest struct{ state ...; unknownFields ...; sizeCache ... }`, no
// data fields) — resolves toolset.md OQ-A for this tool: ListPrimitivesInput is `struct{}`.
func TestListPrimitivesTool_RoundTrip(t *testing.T) {
	called := false
	spec := &sdktest.FakeSpecClient{
		ListPrimitivesFunc: func(context.Context, *pluginv1.ListPrimitivesRequest) (*pluginv1.ListPrimitivesResponse, error) {
			called = true
			return &pluginv1.ListPrimitivesResponse{}, nil
		},
	}
	tl, err := listPrimitivesTool(spec)
	if err != nil {
		t.Fatalf("listPrimitivesTool: %v", err)
	}
	_, callErr := runTool[ListPrimitivesInput, json.RawMessage](t, tl, ListPrimitivesInput{})
	if callErr != nil {
		t.Fatalf("runTool: %v", callErr)
	}
	if !called {
		t.Fatal("SpecClient.ListPrimitives was not called")
	}
}

func TestListPrimitivesTool_PropagatesRPCError(t *testing.T) {
	wantErr := errors.New("boom")
	spec := &sdktest.FakeSpecClient{
		ListPrimitivesFunc: func(context.Context, *pluginv1.ListPrimitivesRequest) (*pluginv1.ListPrimitivesResponse, error) {
			return nil, wantErr
		},
	}
	tl, err := listPrimitivesTool(spec)
	if err != nil {
		t.Fatalf("listPrimitivesTool: %v", err)
	}
	_, callErr := runTool[ListPrimitivesInput, json.RawMessage](t, tl, ListPrimitivesInput{})
	if !errors.Is(callErr, wantErr) {
		t.Fatalf("callErr = %v, want it to wrap %v", callErr, wantErr)
	}
}
