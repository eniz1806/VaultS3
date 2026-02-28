package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// EventBus distributes real-time bucket events to SSE subscribers.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[chan []byte]struct{}),
	}
}

// Publish sends an event to all subscribers (non-blocking).
func (eb *EventBus) Publish(eventType, bucket, key string, size int64, etag string) {
	data, _ := json.Marshal(map[string]interface{}{
		"eventType": eventType,
		"bucket":    bucket,
		"key":       key,
		"size":      size,
		"etag":      etag,
	})

	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for ch := range eb.subscribers {
		select {
		case ch <- data:
		default:
			// skip slow subscribers
		}
	}
}

func (eb *EventBus) subscribe() chan []byte {
	ch := make(chan []byte, 64)
	eb.mu.Lock()
	eb.subscribers[ch] = struct{}{}
	eb.mu.Unlock()
	return ch
}

func (eb *EventBus) unsubscribe(ch chan []byte) {
	eb.mu.Lock()
	delete(eb.subscribers, ch)
	eb.mu.Unlock()
	close(ch)
}

// handleEvents handles GET /api/v1/events (SSE endpoint).
func (h *APIHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventBus == nil {
		writeError(w, http.StatusServiceUnavailable, "event streaming not enabled")
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

	ch := h.eventBus.subscribe()
	defer h.eventBus.unsubscribe(ch)

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
