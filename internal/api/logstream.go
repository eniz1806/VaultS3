package api

import (
	"fmt"
	"net/http"
	"sync"
)

// LogBroadcaster distributes log lines to SSE subscribers.
type LogBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan string]struct{}
}

// NewLogBroadcaster creates a new log broadcaster.
func NewLogBroadcaster() *LogBroadcaster {
	return &LogBroadcaster{
		subscribers: make(map[chan string]struct{}),
	}
}

// Write implements io.Writer for slog integration.
func (lb *LogBroadcaster) Write(p []byte) (n int, err error) {
	line := string(p)
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	for ch := range lb.subscribers {
		select {
		case ch <- line:
		default:
		}
	}
	return len(p), nil
}

func (lb *LogBroadcaster) subscribe() chan string {
	ch := make(chan string, 128)
	lb.mu.Lock()
	lb.subscribers[ch] = struct{}{}
	lb.mu.Unlock()
	return ch
}

func (lb *LogBroadcaster) unsubscribe(ch chan string) {
	lb.mu.Lock()
	delete(lb.subscribers, ch)
	lb.mu.Unlock()
	close(ch)
}

// handleLogStream handles GET /api/v1/logs (SSE endpoint).
func (h *APIHandler) handleLogStream(w http.ResponseWriter, r *http.Request) {
	if h.logBroadcaster == nil {
		writeError(w, http.StatusServiceUnavailable, "log streaming not enabled")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.logBroadcaster.subscribe()
	defer h.logBroadcaster.unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case line := <-ch:
			fmt.Fprintf(w, "data: %s\n", line)
			flusher.Flush()
		}
	}
}
