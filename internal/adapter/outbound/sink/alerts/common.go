package alerts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Shared HTTP delivery for the alert sinks (Slack / Telegram / Opsgenie).
//
// The three sinks previously each called a naive retryBackoff that
// (1) leaked the request URL — which carries the bot token / webhook
// secret — into the returned error, (2) retried every non-2xx status
// including permanent 4xx, hammering a revoked webhook 5×/entry, and
// (3) blocked the calling goroutine for up to ~28s per entry with no
// overall deadline. This helper fixes all three: it classifies which
// failures are worth retrying, bounds total wall-clock, and never lets
// a transport error (which embeds the secret URL) escape verbatim.

const (
	alertMaxAttempts = 4
	alertTotalBudget = 8 * time.Second
	alertMaxRetryGap = 2 * time.Second
)

// postConfig is an immutable snapshot of a sink's HTTP parameters,
// taken under the sink lock so delivery never races Configure.
type postConfig struct {
	name    string // sink name, used for error messages (never the URL)
	url     string
	headers map[string]string
	body    []byte
}

// deliver posts cfg.body to cfg.url with bounded retries. It returns a
// sanitized error: transport failures are reported by category only, so
// the secret-bearing URL is never surfaced to callers that log sink
// errors.
func deliver(client *http.Client, cfg postConfig) error {
	deadline := time.Now().Add(alertTotalBudget)
	delay := 200 * time.Millisecond
	var lastErr error

	for attempt := 0; attempt < alertMaxAttempts; attempt++ {
		retryable, err := postOnce(client, cfg)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryable || attempt == alertMaxAttempts-1 || time.Now().Add(delay).After(deadline) {
			return lastErr
		}
		time.Sleep(delay)
		if delay < alertMaxRetryGap {
			delay *= 2
		}
	}
	return lastErr
}

// postOnce performs a single POST and reports whether the failure (if
// any) is worth retrying. Transport errors and 5xx/429 are retryable;
// permanent 4xx are not. The returned error is always sanitized.
func postOnce(client *http.Client, cfg postConfig) (retryable bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.url, bytes.NewReader(cfg.body))
	if err != nil {
		return false, fmt.Errorf("%s: build request failed", cfg.name)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		// A *url.Error embeds the full request URL (bot token / webhook
		// secret). Report the category only — never the URL.
		return true, fmt.Errorf("%s: request failed: %s", cfg.name, transportErrorKind(err))
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode < 300:
		return false, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return true, fmt.Errorf("%s: status %s", cfg.name, resp.Status)
	default:
		// Permanent client error (bad/revoked webhook, auth failure):
		// retrying just hammers the endpoint, so give up immediately.
		return false, fmt.Errorf("%s: status %s (not retried)", cfg.name, resp.Status)
	}
}

// transportErrorKind reduces a transport error to a safe category,
// stripping the URL that *url.Error would otherwise expose.
func transportErrorKind(err error) string {
	var uerr *url.Error
	if errors.As(err, &uerr) && uerr.Timeout() {
		return "timeout"
	}
	return "connection error"
}
