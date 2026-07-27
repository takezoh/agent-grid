package wslhost

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Health status values accepted from GET /healthz JSON body.
const (
	HealthStatusOK          = "ok"
	HealthStatusUnavailable = "unavailable"
	HealthStatusUnhealthy   = "unhealthy"
)

// HealthResult is one probe observation.
type HealthResult struct {
	Status     string // ok | unavailable | unhealthy | "" on transport failure
	HTTPStatus int
	Healthy    bool // true only for HTTP 200 + status ok
	Reason     string
	At         time.Time
}

// HealthProber is the HTTP health/TLS seam.
type HealthProber interface {
	Probe(ctx context.Context, healthURL string, timeout time.Duration) (HealthResult, error)
}

// Clock is the injectable time source.
type Clock interface {
	Now() time.Time
}

// Timer cancels a scheduled callback.
type Timer interface {
	Stop() bool
}

// TimerScheduler schedules one-shot callbacks (injected; fake-clock testable).
type TimerScheduler interface {
	AfterFunc(d time.Duration, fn func()) Timer
}

// ResumeSource delivers Windows sleep/hibernate resume notifications.
type ResumeSource interface {
	// Subscribe registers a callback invoked on resume. Returns unsubscribe.
	Subscribe(fn func(at time.Time)) (unsubscribe func())
}

// RealClock uses the system wall clock.
type RealClock struct{}

// Now implements Clock.
func (RealClock) Now() time.Time { return time.Now() }

// RealScheduler uses time.AfterFunc.
type RealScheduler struct{}

// AfterFunc implements TimerScheduler.
func (RealScheduler) AfterFunc(d time.Duration, fn func()) Timer {
	return realTimer{t: time.AfterFunc(d, fn)}
}

type realTimer struct{ t *time.Timer }

func (r realTimer) Stop() bool { return r.t.Stop() }

// ManualClock is the deterministic fake clock + timer seam.
type ManualClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*manualTimer
	// OnTimerFire is invoked synchronously when a timer fires via Advance.
}

type manualTimer struct {
	// deadline absolute
	deadline time.Time
	fn       func()
	stopped  bool
	clock    *ManualClock
}

// NewManualClock constructs a fake clock at now.
func NewManualClock(now time.Time) *ManualClock {
	return &ManualClock{now: now}
}

// Now implements Clock.
func (c *ManualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// AfterFunc implements TimerScheduler.
func (c *ManualClock) AfterFunc(d time.Duration, fn func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &manualTimer{deadline: c.now.Add(d), fn: fn, clock: c}
	c.timers = append(c.timers, t)
	return t
}

// Stop implements Timer.
func (t *manualTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}

// Advance moves time forward and fires due timers in order (at most one pass
// queue: newly scheduled timers during fire are considered on subsequent Advance).
func (c *ManualClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	var due []*manualTimer
	remaining := c.timers[:0]
	for _, t := range c.timers {
		if t.stopped {
			continue
		}
		if !t.deadline.After(now) {
			due = append(due, t)
			t.stopped = true
		} else {
			remaining = append(remaining, t)
		}
	}
	c.timers = remaining
	c.mu.Unlock()
	for _, t := range due {
		if t.fn != nil {
			t.fn()
		}
	}
}

// PendingTimers returns the number of non-stopped timers.
func (c *ManualClock) PendingTimers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, t := range c.timers {
		if !t.stopped {
			n++
		}
	}
	return n
}

// FakeResumeSource is the manual resume seam.
type FakeResumeSource struct {
	mu   sync.Mutex
	subs []func(time.Time)
}

// Subscribe implements ResumeSource.
func (f *FakeResumeSource) Subscribe(fn func(at time.Time)) func() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subs = append(f.subs, fn)
	idx := len(f.subs) - 1
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if idx >= 0 && idx < len(f.subs) {
			f.subs[idx] = nil
		}
	}
}

// Resume delivers a resume notification to all subscribers.
func (f *FakeResumeSource) Resume(at time.Time) {
	f.mu.Lock()
	subs := append([]func(time.Time){}, f.subs...)
	f.mu.Unlock()
	for _, fn := range subs {
		if fn != nil {
			fn(at)
		}
	}
}

// FakeHealthProber is the scripted health seam.
type FakeHealthProber struct {
	mu sync.Mutex

	ProbeFn func(ctx context.Context, healthURL string, timeout time.Duration) (HealthResult, error)

	// Next is returned when Queue is empty.
	Next  HealthResult
	Err   error
	Queue []HealthResult
	Calls []string
}

// Probe implements HealthProber.
func (f *FakeHealthProber) Probe(ctx context.Context, healthURL string, timeout time.Duration) (HealthResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, healthURL)
	if f.ProbeFn != nil {
		return f.ProbeFn(ctx, healthURL, timeout)
	}
	if f.Err != nil {
		return HealthResult{}, f.Err
	}
	if len(f.Queue) > 0 {
		r := f.Queue[0]
		f.Queue = f.Queue[1:]
		return r, nil
	}
	return f.Next, nil
}

// SetOK queues/sets a healthy result.
func (f *FakeHealthProber) SetOK() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Next = HealthResult{Status: HealthStatusOK, HTTPStatus: 200, Healthy: true, Reason: "status_ok"}
}

// SetUnavailable sets daemon-unavailable.
func (f *FakeHealthProber) SetUnavailable() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Next = HealthResult{Status: HealthStatusUnavailable, HTTPStatus: 200, Healthy: false, Reason: "daemon_unavailable"}
}

// CallCount returns probe call count.
func (f *FakeHealthProber) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Calls)
}

// ClassifyHealthBody maps HTTP status + JSON body onto HealthResult.
// Only HTTP 200 with {"status":"ok"} is healthy. daemon-unavailable stays non-Ready.
func ClassifyHealthBody(httpStatus int, body []byte) HealthResult {
	r := HealthResult{HTTPStatus: httpStatus}
	if httpStatus != http.StatusOK {
		r.Reason = fmt.Sprintf("http_%d", httpStatus)
		return r
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		// transport success alone is not Ready
		r.Reason = "body_not_json"
		return r
	}
	r.Status = strings.TrimSpace(payload.Status)
	switch r.Status {
	case HealthStatusOK:
		r.Healthy = true
		r.Reason = "status_ok"
	case HealthStatusUnavailable:
		r.Reason = "daemon_unavailable"
	case HealthStatusUnhealthy:
		r.Reason = "status_unhealthy"
	default:
		r.Reason = "status_not_ok"
	}
	return r
}

// HTTPHealthProber is the production HealthProber using net/http.
// TLS mode is derived from the URL scheme; insecure HTTPS is opt-in via InsecureSkipVerify.
type HTTPHealthProber struct {
	// Client, when non-nil, is used as-is.
	Client *http.Client
	// InsecureSkipVerify allows self-signed certs for https health targets.
	InsecureSkipVerify bool
}

// Probe implements HealthProber.
func (p *HTTPHealthProber) Probe(ctx context.Context, healthURL string, timeout time.Duration) (HealthResult, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	client := p.Client
	if client == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		if p.InsecureSkipVerify {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // health to bound local endpoint
		}
		client = &http.Client{Transport: tr, Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return HealthResult{Reason: "bad_url"}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return HealthResult{Reason: "transport: " + err.Error()}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	r := ClassifyHealthBody(resp.StatusCode, body)
	return r, nil
}

// ChannelResumeSource is a production-shaped ResumeSource driven by a channel
// (Windows adapter converts power notifications into channel sends).
type ChannelResumeSource struct {
	mu   sync.Mutex
	subs []func(time.Time)
	ch   <-chan time.Time
	done chan struct{}
}

// NewChannelResumeSource starts a fan-out loop on ch.
func NewChannelResumeSource(ch <-chan time.Time) *ChannelResumeSource {
	c := &ChannelResumeSource{ch: ch, done: make(chan struct{})}
	go c.loop()
	return c
}

func (c *ChannelResumeSource) loop() {
	for {
		select {
		case <-c.done:
			return
		case at, ok := <-c.ch:
			if !ok {
				return
			}
			c.mu.Lock()
			subs := append([]func(time.Time){}, c.subs...)
			c.mu.Unlock()
			for _, fn := range subs {
				if fn != nil {
					fn(at)
				}
			}
		}
	}
}

// Subscribe implements ResumeSource.
func (c *ChannelResumeSource) Subscribe(fn func(at time.Time)) func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs = append(c.subs, fn)
	idx := len(c.subs) - 1
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if idx >= 0 && idx < len(c.subs) {
			c.subs[idx] = nil
		}
	}
}

// Close stops the source loop.
func (c *ChannelResumeSource) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}
