package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TraceBroadcaster distributes API trace events to SSE subscribers.
type TraceBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

// NewTraceBroadcaster creates a new trace broadcaster.
func NewTraceBroadcaster() *TraceBroadcaster {
	return &TraceBroadcaster{
		subscribers: make(map[chan []byte]struct{}),
	}
}

// TraceEvent represents a single API call trace.
type TraceEvent struct {
	Time     string `json:"time"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Status   int    `json:"status"`
	Duration string `json:"duration"`
	ClientIP string `json:"clientIP"`
}

// Record publishes a trace event.
func (tb *TraceBroadcaster) Record(method, path string, status int, duration time.Duration, clientIP string) {
	data, _ := json.Marshal(TraceEvent{
		Time:     time.Now().UTC().Format(time.RFC3339Nano),
		Method:   method,
		Path:     path,
		Status:   status,
		Duration: duration.String(),
		ClientIP: clientIP,
	})

	tb.mu.RLock()
	defer tb.mu.RUnlock()
	for ch := range tb.subscribers {
		select {
		case ch <- data:
		default:
		}
	}
}

func (tb *TraceBroadcaster) subscribe() chan []byte {
	ch := make(chan []byte, 128)
	tb.mu.Lock()
	tb.subscribers[ch] = struct{}{}
	tb.mu.Unlock()
	return ch
}

func (tb *TraceBroadcaster) unsubscribe(ch chan []byte) {
	tb.mu.Lock()
	delete(tb.subscribers, ch)
	tb.mu.Unlock()
	close(ch)
}

// handleTrace handles GET /api/v1/trace (SSE endpoint).
func (h *APIHandler) handleTrace(w http.ResponseWriter, r *http.Request) {
	if h.traceBroadcaster == nil {
		writeError(w, http.StatusServiceUnavailable, "tracing not enabled")
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

	ch := h.traceBroadcaster.subscribe()
	defer h.traceBroadcaster.unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
