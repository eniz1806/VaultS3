package api

import (
	"net/http"
	"runtime"
	"time"
)

type diagnosticsResponse struct {
	Timestamp string            `json:"timestamp"`
	Go        goDiagnostics     `json:"go"`
	System    systemDiagnostics `json:"system"`
}

type goDiagnostics struct {
	Version    string `json:"version"`
	Goroutines int    `json:"goroutines"`
	HeapAlloc  uint64 `json:"heapAllocBytes"`
	HeapSys    uint64 `json:"heapSysBytes"`
	NumGC      uint32 `json:"numGC"`
	NumCPU     int    `json:"numCPU"`
}

type systemDiagnostics struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// handleDiagnostics handles GET /api/v1/diagnostics.
func (h *APIHandler) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := diagnosticsResponse{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Go: goDiagnostics{
			Version:    runtime.Version(),
			Goroutines: runtime.NumGoroutine(),
			HeapAlloc:  m.HeapAlloc,
			HeapSys:    m.HeapSys,
			NumGC:      m.NumGC,
			NumCPU:     runtime.NumCPU(),
		},
		System: systemDiagnostics{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}
