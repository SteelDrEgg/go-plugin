package goplugin

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// ExitResult describes a backend termination, not its runtime health.
// ExitCode is -1 when no numeric exit code is available (including a signal).
type ExitResult struct {
	ExitCode int
	Err      error
	ExitedAt time.Time
}

func (r ExitResult) Failed() bool { return r.ExitCode != 0 || r.Err != nil }

type exitState struct {
	once   sync.Once
	done   chan struct{}
	result ExitResult
}

func newExitState() *exitState { return &exitState{done: make(chan struct{})} }
func (s *exitState) finish(result ExitResult) {
	s.once.Do(func() { result.ExitedAt = time.Now(); s.result = result; close(s.done) })
}
func (s *exitState) observeProcess(ctx context.Context, cmd *exec.Cmd) {
	// Hashicorp cancels this context after its Runner.Wait has completed.
	// This observes the existing reaper; it never calls cmd.Wait again.
	context.AfterFunc(ctx, func() {
		result := ExitResult{ExitCode: -1, Err: fmt.Errorf("backend exited without process status")}
		if cmd != nil && cmd.ProcessState != nil {
			result.ExitCode = cmd.ProcessState.ExitCode()
			if !cmd.ProcessState.Success() {
				result.Err = &exec.ExitError{ProcessState: cmd.ProcessState}
			} else {
				result.Err = nil
			}
		}
		s.finish(result)
	})
}

// Done closes once the backend has exited. Nil means the backend does not
// provide exit notifications. Cancellation of a successful load is not an exit.
func (h *Handle) Done() <-chan struct{} {
	if h == nil || h.exit == nil {
		return nil
	}
	return h.exit.done
}

// ExitResult returns an immutable snapshot after Done closes.
func (h *Handle) ExitResult() (ExitResult, bool) {
	if h == nil || h.exit == nil {
		return ExitResult{}, false
	}
	select {
	case <-h.exit.done:
		return h.exit.result, true
	default:
		return ExitResult{}, false
	}
}

// ReportExit is supplied to WASM loaders for actual module termination.
// Ordinary invocation errors must not be reported as exits.
type ReportExit func(ExitResult)
