// Package httpretry provides bounded retries with exponential backoff for
// transient HTTP failures, shared by the GitLab, LLM, and Slack clients.
package httpretry

import (
	"io"
	"net/http"
	"strconv"
	"time"
)

// sleep is a package-level indirection so tests can stub out real waiting.
var sleep = time.Sleep

// Policy controls bounded retries for transient HTTP failures: network
// errors, 5xx responses, and 429 responses that carry a Retry-After header.
type Policy struct {
	MaxAttempts int           // total attempts including the first (min 1)
	BaseDelay   time.Duration // backoff before the first retry; doubles each retry
	MaxDelay    time.Duration // cap on any single backoff; <=0 means no cap
}

// DefaultPolicy returns sensible defaults for a CI-time tool: 3 attempts
// (two retries) with exponential backoff starting at 500ms, capped at 30s.
func DefaultPolicy() Policy {
	return Policy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}
}

// Do executes the request produced by newRequest, retrying transient
// failures per the policy. newRequest must return a fresh *http.Request on
// each call, since an attempt consumes the request and its body.
//
// On success — or on the final attempt regardless of outcome — Do returns
// the response with its body unread, for the caller to process and close.
// When every attempt fails at the network level, Do returns that error and
// no response.
//
// Note: because Do retries network-level failures, callers using it for
// non-idempotent requests (POST) accept a small risk of duplicate delivery
// when a request succeeds server-side but its response is lost in transit.
func (p Policy) Do(client *http.Client, newRequest func() (*http.Request, error)) (*http.Response, error) {
	attempts := p.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var resp *http.Response
	var err error

	for attempt := 1; attempt <= attempts; attempt++ {
		var req *http.Request
		req, err = newRequest()
		if err != nil {
			// Building the request failed — not a transient condition.
			return nil, err
		}

		resp, err = client.Do(req)
		last := attempt == attempts

		if err != nil {
			// Network-level failure: retry unless this was the last attempt.
			if last {
				return nil, err
			}
			sleep(p.backoff(attempt, 0))
			continue
		}

		if last || !retryable(resp) {
			return resp, nil
		}

		// Retryable response with attempts remaining: drain the body so the
		// connection can be reused, then back off and try again.
		wait := p.backoff(attempt, retryAfter(resp))
		drain(resp.Body)
		sleep(wait)
	}

	return resp, err
}

// retryable reports whether a response represents a transient failure worth
// retrying: any 5xx, or a 429 that tells us (via Retry-After) when to retry.
// A 429 without Retry-After is treated as permanent (e.g. exhausted quota).
func retryable(resp *http.Response) bool {
	if resp.StatusCode >= 500 {
		return true
	}
	if resp.StatusCode == http.StatusTooManyRequests && resp.Header.Get("Retry-After") != "" {
		return true
	}
	return false
}

// backoff returns how long to wait before the next attempt. A positive
// retryAfter (from a Retry-After header) takes precedence over exponential
// backoff. The result is capped at MaxDelay when that is set.
func (p Policy) backoff(attempt int, retryAfter time.Duration) time.Duration {
	var d time.Duration
	if retryAfter > 0 {
		d = retryAfter
	} else {
		d = p.BaseDelay << (attempt - 1)
	}
	if p.MaxDelay > 0 && d > p.MaxDelay {
		d = p.MaxDelay
	}
	return d
}

// retryAfter parses a Retry-After header expressed as a number of seconds.
// The HTTP-date form is rare for these APIs and is treated as absent, which
// falls back to exponential backoff. Returns 0 when absent or unparseable.
func retryAfter(resp *http.Response) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

// drain reads and closes a response body so the underlying connection can be
// reused rather than leaked.
func drain(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}
