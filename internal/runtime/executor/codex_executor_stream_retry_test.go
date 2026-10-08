package executor

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v8/sdk/translator"
)

const (
	codexRetryTestCreated    = "data: {\"type\":\"response.created\",\"sequence_number\":0,\"response\":{\"id\":\"resp_1\",\"model\":\"gpt-5.5\"}}\n\n"
	codexRetryTestInProgress = "data: {\"type\":\"response.in_progress\",\"sequence_number\":1,\"response\":{\"id\":\"resp_1\"}}\n\n"
	codexRetryTestKeepalive  = "data: {\"type\":\"keepalive\"}\n\n"
	codexRetryTestTextDelta  = "data: {\"type\":\"response.output_text.delta\",\"sequence_number\":2,\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"hello\"}\n\n"
	codexRetryTestCompleted  = "data: {\"type\":\"response.completed\",\"sequence_number\":3,\"response\":{\"id\":\"resp_1\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"msg_1\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n\n"
)

func codexRetryTestHandshake() string {
	return codexRetryTestCreated + codexRetryTestInProgress + codexRetryTestKeepalive
}

func codexRetryTestFullStream() string {
	return codexRetryTestHandshake() + codexRetryTestTextDelta + codexRetryTestCompleted
}

func withFastCodexIncompleteRetryBackoff(t *testing.T) {
	t.Helper()
	restore := setCodexIncompleteRetryBackoffForTest(func(int) time.Duration { return time.Millisecond })
	t.Cleanup(restore)
}

func runCodexRetryTestStream(t *testing.T, ctx context.Context, cfg *config.Config, baseURL string) ([][]byte, error) {
	t.Helper()
	executor := NewCodexExecutor(cfg)
	auth := &cliproxyauth.Auth{Attributes: map[string]string{
		"base_url": baseURL,
		"api_key":  "test",
	}}
	result, err := executor.ExecuteStream(ctx, auth, cliproxyexecutor.Request{
		Model:   "gpt-5.5",
		Payload: []byte(`{"model":"gpt-5.5","input":"hello"}`),
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai-response"),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("ExecuteStream error: %v", err)
	}
	var payloads [][]byte
	var streamErr error
	for chunk := range result.Chunks {
		if chunk.Err != nil {
			streamErr = chunk.Err
			continue
		}
		payloads = append(payloads, chunk.Payload)
	}
	return payloads, streamErr
}

func countCodexRetryTestEvents(payloads [][]byte, eventType string) int {
	needle := []byte("\"type\":\"" + eventType + "\"")
	count := 0
	for _, payload := range payloads {
		count += bytes.Count(payload, needle)
	}
	return count
}

func TestCodexExecutorStreamRetriesEarlyEndWithNothingGenerated(t *testing.T) {
	withFastCodexIncompleteRetryBackoff(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if requests.Add(1) == 1 {
			_, _ = io.WriteString(w, codexRetryTestHandshake())
			return
		}
		_, _ = io.WriteString(w, codexRetryTestFullStream())
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Codex.StreamIncompleteRetries = 2
	payloads, streamErr := runCodexRetryTestStream(t, context.Background(), cfg, server.URL)
	if streamErr != nil {
		t.Fatalf("expected the retried stream to complete, got error: %v", streamErr)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("upstream requests = %d, want 2", got)
	}
	if got := countCodexRetryTestEvents(payloads, "response.created"); got != 1 {
		t.Fatalf("response.created forwarded %d times, want exactly 1 (retry handshake must be suppressed)", got)
	}
	if got := countCodexRetryTestEvents(payloads, "response.in_progress"); got != 1 {
		t.Fatalf("response.in_progress forwarded %d times, want exactly 1", got)
	}
	if got := countCodexRetryTestEvents(payloads, "response.output_text.delta"); got != 1 {
		t.Fatalf("response.output_text.delta forwarded %d times, want 1", got)
	}
	if got := countCodexRetryTestEvents(payloads, "response.completed"); got != 1 {
		t.Fatalf("response.completed forwarded %d times, want 1", got)
	}
}

func TestCodexExecutorStreamDoesNotRetryAfterGeneratedContent(t *testing.T) {
	withFastCodexIncompleteRetryBackoff(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		requests.Add(1)
		_, _ = io.WriteString(w, codexRetryTestHandshake()+codexRetryTestTextDelta)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Codex.StreamIncompleteRetries = 2
	_, streamErr := runCodexRetryTestStream(t, context.Background(), cfg, server.URL)
	if streamErr == nil {
		t.Fatal("expected missing-completion stream error after generated content, got nil")
	}
	if got := statusCodeFromTestError(t, streamErr); got != http.StatusRequestTimeout {
		t.Fatalf("status code = %d, want %d; err=%v", got, http.StatusRequestTimeout, streamErr)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("upstream requests = %d, want 1 (no retry once content was forwarded)", got)
	}
}

func TestCodexExecutorStreamRetryBudgetExhausted(t *testing.T) {
	withFastCodexIncompleteRetryBackoff(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		requests.Add(1)
		_, _ = io.WriteString(w, codexRetryTestHandshake())
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Codex.StreamIncompleteRetries = 2
	payloads, streamErr := runCodexRetryTestStream(t, context.Background(), cfg, server.URL)
	if streamErr == nil {
		t.Fatal("expected missing-completion stream error after the retry budget, got nil")
	}
	if got := statusCodeFromTestError(t, streamErr); got != http.StatusRequestTimeout {
		t.Fatalf("status code = %d, want %d; err=%v", got, http.StatusRequestTimeout, streamErr)
	}
	assertRequestScopedTestError(t, streamErr)
	if got := requests.Load(); got != 3 {
		t.Fatalf("upstream requests = %d, want 3 (initial + 2 retries)", got)
	}
	if got := countCodexRetryTestEvents(payloads, "response.created"); got != 1 {
		t.Fatalf("response.created forwarded %d times, want exactly 1", got)
	}
}

func TestCodexExecutorStreamRetryDisabledByDefault(t *testing.T) {
	withFastCodexIncompleteRetryBackoff(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		requests.Add(1)
		_, _ = io.WriteString(w, codexRetryTestHandshake())
	}))
	defer server.Close()

	_, streamErr := runCodexRetryTestStream(t, context.Background(), &config.Config{}, server.URL)
	if streamErr == nil {
		t.Fatal("expected missing-completion stream error, got nil")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("upstream requests = %d, want 1 (retries default to 0)", got)
	}
}

func TestCodexExecutorStreamRetriesTransportErrorWithNothingGenerated(t *testing.T) {
	withFastCodexIncompleteRetryBackoff(t)
	var requests atomic.Int32
	handshake := []byte(codexRetryTestHandshake())
	ctx := context.WithValue(context.Background(), "cliproxy.roundtripper", roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var body io.Reader
		if requests.Add(1) == 1 {
			body = io.MultiReader(bytes.NewReader(handshake), unexpectedEOFReader{})
		} else {
			body = strings.NewReader(codexRetryTestFullStream())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/event-stream"}},
			Body:       io.NopCloser(body),
			Request:    req,
		}, nil
	}))

	cfg := &config.Config{}
	cfg.Codex.StreamIncompleteRetries = 1
	payloads, streamErr := runCodexRetryTestStream(t, ctx, cfg, "http://codex.test")
	if streamErr != nil {
		t.Fatalf("expected the retried stream to complete, got error: %v", streamErr)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("upstream requests = %d, want 2", got)
	}
	if got := countCodexRetryTestEvents(payloads, "response.created"); got != 1 {
		t.Fatalf("response.created forwarded %d times, want exactly 1", got)
	}
	if got := countCodexRetryTestEvents(payloads, "response.completed"); got != 1 {
		t.Fatalf("response.completed forwarded %d times, want 1", got)
	}
}

func TestCodexExecutorStreamRetryStopsWhenContextCancelledDuringBackoff(t *testing.T) {
	restore := setCodexIncompleteRetryBackoffForTest(func(int) time.Duration { return time.Hour })
	t.Cleanup(restore)
	var requests atomic.Int32
	firstServed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if requests.Add(1) == 1 {
			_, _ = io.WriteString(w, codexRetryTestHandshake())
			close(firstServed)
			return
		}
		_, _ = io.WriteString(w, codexRetryTestFullStream())
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-firstServed
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	cfg := &config.Config{}
	cfg.Codex.StreamIncompleteRetries = 1
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = runCodexRetryTestStream(t, ctx, cfg, server.URL)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not end after the request context was cancelled during retry backoff")
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("upstream requests = %d, want 1 (no retry after cancellation)", got)
	}
}

func TestCodexExecutorStreamRetryKeepsOneClaudeMessageStart(t *testing.T) {
	withFastCodexIncompleteRetryBackoff(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if requests.Add(1) == 1 {
			_, _ = io.WriteString(w, codexRetryTestHandshake())
			return
		}
		_, _ = io.WriteString(w, codexRetryTestFullStream())
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.Codex.StreamIncompleteRetries = 1
	executor := NewCodexExecutor(cfg)
	auth := &cliproxyauth.Auth{Attributes: map[string]string{"base_url": server.URL, "api_key": "test"}}
	result, err := executor.ExecuteStream(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "gpt-5.5",
		Payload: []byte(`{"model":"gpt-5.5","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hello"}]}`),
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("claude"),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("ExecuteStream error: %v", err)
	}
	var joined bytes.Buffer
	var streamErr error
	for chunk := range result.Chunks {
		if chunk.Err != nil {
			streamErr = chunk.Err
			continue
		}
		joined.Write(chunk.Payload)
	}
	if streamErr != nil {
		t.Fatalf("expected the retried stream to complete, got error: %v", streamErr)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("upstream requests = %d, want 2", got)
	}
	out := joined.String()
	if got := strings.Count(out, "event: message_start"); got != 1 {
		t.Fatalf("message_start emitted %d times, want exactly 1; stream:\n%s", got, out)
	}
	if got := strings.Count(out, "event: message_stop"); got != 1 {
		t.Fatalf("message_stop emitted %d times, want exactly 1; stream:\n%s", got, out)
	}
	if !strings.Contains(out, `"text":"hello"`) {
		t.Fatalf("expected the retried generation's text in the stream:\n%s", out)
	}
}
