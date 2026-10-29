package host

// PauseAfterStep requests a pause at the next engine boundary. The active agent
// keeps its context and can finish its tools and persist its completed work.
// start (including Resume) clears the request before dispatching another step.
func (h *Host) PauseAfterStep() bool {
	h.mu.Lock()
	running := h.lifecycle == lifecycleRunning
	h.mu.Unlock()
	if !running || h.engine == nil {
		return false
	}
	h.engine.mu.Lock()
	defer h.engine.mu.Unlock()
	if !h.engine.running {
		return false
	}
	h.engine.pauseAfterStep = true
	return true
}

func (e *engine) pauseAtBoundary() bool {
	e.mu.Lock()
	pause := e.pauseAfterStep
	summary := e.pauseSummary
	e.mu.Unlock()
	if !pause {
		return false
	}
	if summary == "" {
		summary = "Đã tạm dừng sau bước hiện tại"
	}
	e.onPause(summary)
	return true
}
