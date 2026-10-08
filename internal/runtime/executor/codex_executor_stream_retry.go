package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/runtime/executor/helps"
)

// codexIncompleteRetryBackoff returns how long to wait before retry number attempt (1-based) of a
// stream that ended before a terminal event. Linear, 1s per attempt, so the full default-capped
// budget of 5 waits 15s in total; the request context cuts any wait short.
var (
	codexIncompleteRetryBackoffMu sync.RWMutex
	codexIncompleteRetryBackoff   = func(attempt int) time.Duration {
		if attempt < 1 {
			attempt = 1
		}
		return time.Duration(attempt) * time.Second
	}
)

func codexIncompleteRetryBackoffFor(attempt int) time.Duration {
	codexIncompleteRetryBackoffMu.RLock()
	fn := codexIncompleteRetryBackoff
	codexIncompleteRetryBackoffMu.RUnlock()
	return fn(attempt)
}

func setCodexIncompleteRetryBackoffForTest(fn func(int) time.Duration) func() {
	codexIncompleteRetryBackoffMu.Lock()
	orig := codexIncompleteRetryBackoff
	codexIncompleteRetryBackoff = fn
	codexIncompleteRetryBackoffMu.Unlock()
	return func() {
		codexIncompleteRetryBackoffMu.Lock()
		codexIncompleteRetryBackoff = orig
		codexIncompleteRetryBackoffMu.Unlock()
	}
}

// waitCodexIncompleteRetryBackoff sleeps for the attempt's backoff unless the context ends first,
// in which case it reports false.
func waitCodexIncompleteRetryBackoff(ctx context.Context, attempt int) bool {
	delay := codexIncompleteRetryBackoffFor(attempt)
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// codexStreamReissuer re-sends the upstream request that opened a Codex SSE stream. The template
// request keeps its headers (identity, routing hint, cache key); only the body reader is renewed,
// from the exact bytes the first attempt sent, so the upstream sees an identical request and the
// prompt cache key still matches.
type codexStreamReissuer struct {
	template *http.Request
	body     []byte
	client   *http.Client
	log      helps.UpstreamRequestLog
}

// reissue performs one more attempt and returns its response. A non-2xx answer is drained,
// closed and returned as an error carrying the upstream status and body summary; the caller
// treats any error as the end of the retry budget rather than as a new terminal error, because
// the client has already received the first attempt's handshake.
func (r *codexStreamReissuer) reissue(ctx context.Context, cfg *config.Config) (*http.Response, error) {
	if r == nil || r.template == nil || r.client == nil {
		return nil, fmt.Errorf("codex executor: stream reissue unavailable")
	}
	req := r.template.Clone(ctx)
	req.Body = io.NopCloser(bytes.NewReader(r.body))
	req.ContentLength = int64(len(r.body))
	body := r.body
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	helps.RecordAPIRequest(ctx, cfg, r.log)
	resp, err := r.client.Do(req)
	if err != nil {
		helps.RecordAPIResponseError(ctx, cfg, err)
		return nil, err
	}
	helps.RecordAPIResponseMetadata(ctx, cfg, resp.StatusCode, resp.Header.Clone())
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("codex executor: stream reissue status %d (body unreadable: %w)", resp.StatusCode, readErr)
		}
		helps.AppendAPIResponseChunk(ctx, cfg, data)
		return nil, fmt.Errorf("codex executor: stream reissue status %d: %s", resp.StatusCode, helps.SummarizeErrorBody(resp.Header.Get("Content-Type"), data))
	}
	return resp, nil
}
