package alerts

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

func errEntry() *domain.LogEntry {
	return &domain.LogEntry{Level: domain.ErrorLevel, Msg: "boom", Timestamp: time.Now()}
}

// roundTripFunc lets a test inject a transport-level failure whose error
// (like a real *url.Error) embeds the request URL.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestTelegram_TransportErrorDoesNotLeakToken: on a transport failure
// the bot token (embedded in the request URL) must not appear in the
// error returned from Write.
func TestTelegram_TransportErrorDoesNotLeakToken(t *testing.T) {
	s := NewTelegramSink("SECRET-TOKEN", "123").(*TelegramSink)
	s.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		// net/http wraps this in a *url.Error carrying r.URL (with token).
		return nil, errors.New("dial tcp: connection refused")
	})}
	err := s.Write(errEntry())
	if err == nil {
		t.Fatal("expected an error from a failing transport")
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Fatalf("bot token leaked in Write error: %q", err.Error())
	}
}

// TestSlack_Permanent4xxNotRetried: a revoked webhook (404) must be hit
// exactly once — retrying a permanent client error just hammers it.
func TestSlack_Permanent4xxNotRetried(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	s := NewSlackSink(srv.URL)
	start := time.Now()
	if err := s.Write(errEntry()); err == nil {
		t.Fatal("expected an error for a 404 webhook")
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("permanent 404 was retried: %d requests (want 1)", n)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Write blocked %v on a permanent 4xx (want ~instant, no retry sleeps)", elapsed)
	}
}

// TestSlack_5xxRetried: a transient 500 IS retried (retryable path still
// works after the classification change).
func TestSlack_5xxRetried(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := NewSlackSink(srv.URL)
	if err := s.Write(errEntry()); err == nil {
		t.Fatal("expected an error for a persistent 500")
	}
	if n := atomic.LoadInt32(&hits); n < 2 {
		t.Fatalf("5xx should be retried: only %d request(s)", n)
	}
}

// TestSlack_TotalBudgetBounded: even against a hard-failing endpoint the
// whole retry loop stays within the documented budget, so an alert
// outage can't stall the log path for tens of seconds.
func TestSlack_TotalBudgetBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := NewSlackSink(srv.URL)
	start := time.Now()
	_ = s.Write(errEntry())
	if elapsed := time.Since(start); elapsed > alertTotalBudget+2*time.Second {
		t.Fatalf("retry loop ran %v, exceeds total budget %v", elapsed, alertTotalBudget)
	}
}

// TestSlack_ConfigureWriteNoRace exercises concurrent Configure/Write;
// must be clean under -race.
func TestSlack_ConfigureWriteNoRace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSlackSink(srv.URL).(*SlackSink)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = s.Write(errEntry())
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = s.Configure(map[string]interface{}{"webhook_url": srv.URL})
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}
