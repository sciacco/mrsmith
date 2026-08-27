package training

import "net/http"

// Letture delle run del sync formativo Factorial persistite dalla
// migrazione 133 (#154, task 6.3 di #151; parent #141). Nessuna route di
// avvio qui: RunFactorialSync resta cablata solo su job runner e CLI
// (cmd/training-factorial-sync).

func (h *handler) handleListFactorialSyncRuns(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.list_factorial_sync_runs", func() (any, error) {
		runs, err := h.store.ListFactorialSyncRuns(r.Context(), 20)
		return FactorialSyncRunListResponse{Runs: runs}, err
	})
}

func (h *handler) handleGetFactorialSyncRun(w http.ResponseWriter, r *http.Request) {
	h.handleRead(w, r, "training.get_factorial_sync_run", func() (any, error) {
		return h.store.GetFactorialSyncRun(r.Context(), r.PathValue("id"))
	})
}
