//go:build windows

package wslhost

import "time"

// WindowsResumeSource bridges Windows power-resume notifications into ResumeSource.
// The service host feeds NotifyResume from the SCM/session power path; this task
// does not own SCM stop escalation or GUI toast policy.
type WindowsResumeSource struct {
	inner *FakeResumeSource
}

// NewWindowsResumeSource constructs a resume source.
func NewWindowsResumeSource() *WindowsResumeSource {
	return &WindowsResumeSource{inner: &FakeResumeSource{}}
}

// Subscribe implements ResumeSource.
func (w *WindowsResumeSource) Subscribe(fn func(at time.Time)) func() {
	return w.inner.Subscribe(fn)
}

// NotifyResume delivers a resume event from the Windows power callback path.
func (w *WindowsResumeSource) NotifyResume(at time.Time) {
	if at.IsZero() {
		at = time.Now()
	}
	w.inner.Resume(at)
}
