// Package admingc replaces /admin/gc from the SvelteKit app.
//
// In Go, GC is automatic; this endpoint exposes runtime.ReadMemStats so an
// operator can spot RSS growth without shelling into the host. Calling GET
// also triggers runtime.GC() - matches the old endpoint's behaviour.
//
// Not auth-gated. If you ever expose this to the internet, wrap it with a
// token check or hide it behind your reverse proxy.
package admingc

import (
	"encoding/json"
	"net/http"
	"runtime"
)

type Stats struct {
	Goroutines int    `json:"goroutines"`
	GoVersion  string `json:"go_version"`

	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes   uint64 `json:"heap_sys_bytes"`
	HeapInuseBytes uint64 `json:"heap_inuse_bytes"`
	HeapObjects    uint64 `json:"heap_objects"`
	NumGC          uint32 `json:"num_gc"`
	PauseTotalNs   uint64 `json:"pause_total_ns"`
	NextGCBytes    uint64 `json:"next_gc_bytes"`
	LastGCNs       uint64 `json:"last_gc_ns"`

	BeforeAllocBytes uint64 `json:"before_alloc_bytes"`
	AfterAllocBytes  uint64 `json:"after_alloc_bytes"`
}

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		runtime.GC()
		runtime.ReadMemStats(&after)

		s := Stats{
			Goroutines:       runtime.NumGoroutine(),
			GoVersion:        runtime.Version(),
			HeapAllocBytes:   after.HeapAlloc,
			HeapSysBytes:     after.HeapSys,
			HeapInuseBytes:   after.HeapInuse,
			HeapObjects:      after.HeapObjects,
			NumGC:            after.NumGC,
			PauseTotalNs:     after.PauseTotalNs,
			NextGCBytes:      after.NextGC,
			LastGCNs:         after.LastGC,
			BeforeAllocBytes: before.HeapAlloc,
			AfterAllocBytes:  after.HeapAlloc,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s)
	}
}

func Mount(mux *http.ServeMux) {
	mux.Handle("GET /admin/gc", Handler())
}
