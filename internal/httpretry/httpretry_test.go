package httpretry

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fastPolicy is a policy with no real waiting, for tests that exercise the
// retry loop without caring about exact backoff durations.
func fastPolicy() Policy {
	return Policy{MaxAttempts: 3, BaseDelay: 0, MaxDelay: 0}
}

// stubSleep replaces the package sleep function for the duration of a test,
// recording the durations it was asked to wait.
func stubSleep(t *testing.T) *[]time.Duration {
	t.Helper()
	orig := sleep
	var waits []time.Duration
	sleep = func(d time.Duration) { waits = append(waits, d) }
	t.Cleanup(func() { sleep = orig })
	return &waits
}

func getReq(t *testing.T, url string) func() (*http.Request, error) {
	t.Helper()
	return func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, url, nil)
	}
}

func TestDo_RetriesThenSucceeds(t *testing.T) {
	stubSleep(t)

	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	resp, err := fastPolicy().Do(server.Client(), getReq(t, server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected final status 200, got %d", resp.StatusCode)
	}
	if got := count.Load(); got != 3 {
		t.Errorf("expected server hit 3 times (2 failures + 1 success), got %d", got)
	}
}

func TestDo_PermanentFailureReturnsResponseAfterMaxAttempts(t *testing.T) {
	stubSleep(t)

	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	resp, err := fastPolicy().Do(server.Client(), getReq(t, server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected final status 503 returned to caller, got %d", resp.StatusCode)
	}
	if got := count.Load(); got != 3 {
		t.Errorf("expected exactly 3 attempts, got %d", got)
	}
}

// errTransport always fails at the network level, counting invocations.
type errTransport struct{ n atomic.Int64 }

func (t *errTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.n.Add(1)
	return nil, errors.New("simulated network failure")
}

func TestDo_NetworkErrorRetriesThenReturnsError(t *testing.T) {
	stubSleep(t)

	tr := &errTransport{}
	client := &http.Client{Transport: tr}

	req := func() (*http.Request, error) {
		return http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	}

	resp, err := fastPolicy().Do(client, req)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on network failure, got %+v", resp)
	}
	if got := tr.n.Load(); got != 3 {
		t.Errorf("expected 3 attempts on persistent network error, got %d", got)
	}
}

func TestDo_SucceedsFirstAttemptNoRetry(t *testing.T) {
	waits := stubSleep(t)

	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := fastPolicy().Do(server.Client(), getReq(t, server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if got := count.Load(); got != 1 {
		t.Errorf("expected exactly 1 attempt on immediate success, got %d", got)
	}
	if len(*waits) != 0 {
		t.Errorf("expected no backoff sleeps on immediate success, got %v", *waits)
	}
}

func TestDo_DoesNotRetryClientError(t *testing.T) {
	stubSleep(t)

	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	resp, err := fastPolicy().Do(server.Client(), getReq(t, server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 returned, got %d", resp.StatusCode)
	}
	if got := count.Load(); got != 1 {
		t.Errorf("expected 400 to not be retried (1 attempt), got %d", got)
	}
}

func TestDo_Retries429WithRetryAfter(t *testing.T) {
	stubSleep(t)

	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := fastPolicy().Do(server.Client(), getReq(t, server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected retry after 429-with-Retry-After to succeed, got %d", resp.StatusCode)
	}
	if got := count.Load(); got != 2 {
		t.Errorf("expected 2 attempts, got %d", got)
	}
}

func TestDo_DoesNotRetry429WithoutRetryAfter(t *testing.T) {
	stubSleep(t)

	var count atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	resp, err := fastPolicy().Do(server.Client(), getReq(t, server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 returned, got %d", resp.StatusCode)
	}
	if got := count.Load(); got != 1 {
		t.Errorf("expected 429 without Retry-After to not be retried (1 attempt), got %d", got)
	}
}

func TestDo_NewRequestErrorReturnsImmediately(t *testing.T) {
	stubSleep(t)

	wantErr := errors.New("cannot build request")
	var count atomic.Int64

	_, err := fastPolicy().Do(http.DefaultClient, func() (*http.Request, error) {
		count.Add(1)
		return nil, wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the newRequest error to be returned, got %v", err)
	}
	if got := count.Load(); got != 1 {
		t.Errorf("expected a request-build error to not be retried (1 call), got %d", got)
	}
}

func TestBackoff_Exponential(t *testing.T) {
	p := Policy{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}

	if got := p.backoff(1, 0); got != 100*time.Millisecond {
		t.Errorf("attempt 1 backoff = %v, want 100ms", got)
	}
	if got := p.backoff(2, 0); got != 200*time.Millisecond {
		t.Errorf("attempt 2 backoff = %v, want 200ms", got)
	}
	if got := p.backoff(3, 0); got != 400*time.Millisecond {
		t.Errorf("attempt 3 backoff = %v, want 400ms", got)
	}
}

func TestBackoff_CapsAtMaxDelay(t *testing.T) {
	p := Policy{MaxAttempts: 10, BaseDelay: time.Second, MaxDelay: 3 * time.Second}

	if got := p.backoff(5, 0); got != 3*time.Second {
		t.Errorf("expected backoff capped at MaxDelay 3s, got %v", got)
	}
}

func TestBackoff_RetryAfterTakesPrecedenceAndIsCapped(t *testing.T) {
	p := Policy{MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: 5 * time.Second}

	if got := p.backoff(1, 2*time.Second); got != 2*time.Second {
		t.Errorf("expected Retry-After 2s to win over base delay, got %v", got)
	}
	if got := p.backoff(1, 60*time.Second); got != 5*time.Second {
		t.Errorf("expected Retry-After capped at MaxDelay 5s, got %v", got)
	}
}
