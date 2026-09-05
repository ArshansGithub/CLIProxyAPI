package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
)

// overloadTestExecutor fails the first failures calls with a 529 and records
// which credential each call used.
type overloadTestExecutor struct {
	failures int
	calls    []string
}

func (e *overloadTestExecutor) Identifier() string { return "overload-test-provider" }

func (e *overloadTestExecutor) attempt(auth *Auth) error {
	e.calls = append(e.calls, auth.ID)
	if len(e.calls) <= e.failures {
		return compactTestStatusError{code: 529, msg: "overloaded"}
	}
	return nil
}

func (e *overloadTestExecutor) Execute(_ context.Context, auth *Auth, _ cliproxyexecutor.Request, _ cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	if err := e.attempt(auth); err != nil {
		return cliproxyexecutor.Response{}, err
	}
	return cliproxyexecutor.Response{Payload: []byte(`{"status":"ok"}`)}, nil
}

func (e *overloadTestExecutor) ExecuteStream(_ context.Context, auth *Auth, _ cliproxyexecutor.Request, _ cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	if err := e.attempt(auth); err != nil {
		return nil, err
	}
	ch := make(chan cliproxyexecutor.StreamChunk, 1)
	ch <- cliproxyexecutor.StreamChunk{Payload: []byte(`{"status":"ok"}`)}
	close(ch)
	return &cliproxyexecutor.StreamResult{Chunks: ch}, nil
}

func (e *overloadTestExecutor) Refresh(_ context.Context, auth *Auth) (*Auth, error) {
	return auth, nil
}
func (e *overloadTestExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, errors.New("not supported")
}
func (e *overloadTestExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, errors.New("not supported")
}

func newOverloadManager(t *testing.T, executor *overloadTestExecutor, retry internalconfig.OverloadRetryConfig) (*Manager, string) {
	t.Helper()
	m := NewManager(nil, nil, nil)
	m.RegisterExecutor(executor)
	m.SetConfig(&internalconfig.Config{OverloadRetry: retry})
	model := "test-model"
	for _, id := range []string{"auth1", "auth2"} {
		a := &Auth{ID: id, Provider: executor.Identifier(), Status: StatusActive}
		if _, err := m.Register(context.Background(), a); err != nil {
			t.Fatalf("Register %s: %v", id, err)
		}
		registry.GetGlobalRegistry().RegisterClient(a.ID, a.Provider, []*registry.ModelInfo{{ID: model}})
		t.Cleanup(func() { registry.GetGlobalRegistry().UnregisterClient(a.ID) })
	}
	return m, model
}

func TestOverloadRetryStaysOnSameCredential(t *testing.T) {
	executor := &overloadTestExecutor{failures: 2}
	m, model := newOverloadManager(t, executor, internalconfig.OverloadRetryConfig{Enabled: true, Attempts: 3, Backoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond})
	req := cliproxyexecutor.Request{Model: model, Payload: []byte(`{}`)}

	if _, err := m.Execute(context.Background(), []string{executor.Identifier()}, req, cliproxyexecutor.Options{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(executor.calls) != 3 {
		t.Fatalf("calls = %v, want three attempts", executor.calls)
	}
	for i, id := range executor.calls {
		if id != executor.calls[0] {
			t.Fatalf("call %d used %s, want every attempt on %s", i, id, executor.calls[0])
		}
	}

	executor.calls = nil
	executor.failures = 1
	if _, err := m.ExecuteStream(context.Background(), []string{executor.Identifier()}, req, cliproxyexecutor.Options{}); err != nil {
		t.Fatalf("ExecuteStream: %v", err)
	}
	if len(executor.calls) != 2 || executor.calls[0] != executor.calls[1] {
		t.Fatalf("stream calls = %v, want one retry on the same credential", executor.calls)
	}
}

func TestOverloadRetryFallsOverAfterBudget(t *testing.T) {
	executor := &overloadTestExecutor{failures: 3}
	m, model := newOverloadManager(t, executor, internalconfig.OverloadRetryConfig{Enabled: true, Attempts: 1, Backoff: time.Millisecond})
	req := cliproxyexecutor.Request{Model: model, Payload: []byte(`{}`)}
	if _, err := m.Execute(context.Background(), []string{executor.Identifier()}, req, cliproxyexecutor.Options{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// Two tries on the first credential, then failover to the second.
	if len(executor.calls) < 3 || executor.calls[0] != executor.calls[1] || executor.calls[2] == executor.calls[0] {
		t.Fatalf("calls = %v, want same credential twice then failover", executor.calls)
	}
}

func TestOverloadRetryDisabledFailsOverImmediately(t *testing.T) {
	executor := &overloadTestExecutor{failures: 1}
	m, model := newOverloadManager(t, executor, internalconfig.OverloadRetryConfig{Enabled: false})
	req := cliproxyexecutor.Request{Model: model, Payload: []byte(`{}`)}
	if _, err := m.Execute(context.Background(), []string{executor.Identifier()}, req, cliproxyexecutor.Options{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(executor.calls) != 2 || executor.calls[0] == executor.calls[1] {
		t.Fatalf("calls = %v, want immediate failover to the other credential", executor.calls)
	}
}
