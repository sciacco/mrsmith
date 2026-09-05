package training

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

// planningBadRequest deliberately differs from legacy validationError (422): planning input is 400.
func planningBadRequest(code, message string) error {
	return appError{status: http.StatusBadRequest, code: code, message: message}
}

// Parsing is kept separate from handlers so phase two can compose routes without
// relaxing the HTTP contract.
func parsePlanningListFilter(r *http.Request) (PlanningListFilter, error) {
	q := r.URL.Query()
	f := PlanningListFilter{View: strings.TrimSpace(q.Get("view")), Q: strings.TrimSpace(q.Get("q")), Tag: q.Get("tag"), EmployeeID: strings.TrimSpace(q.Get("employeeId")), TeamID: strings.TrimSpace(q.Get("teamId")), SkillAreaID: strings.TrimSpace(q.Get("skillAreaId")), Limit: 25}
	if f.View == "" {
		f.View = "operative"
	}
	if f.View != "operative" && f.View != "reminders" && f.View != "suspended" && f.View != "history" {
		return f, planningBadRequest("invalid_view", "vista non valida")
	}
	if utf8.RuneCountInString(f.Q) > 200 || utf8.RuneCountInString(f.Tag) > 200 {
		return f, planningBadRequest("invalid_filter", "filtro testo troppo lungo")
	}
	for _, v := range []string{f.EmployeeID, f.TeamID, f.SkillAreaID} {
		if v != "" {
			if _, err := uuid.Parse(v); err != nil {
				return f, planningBadRequest("invalid_id", "id filtro non valido")
			}
		}
	}
	var err error
	if q.Get("limit") != "" {
		f.Limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || (f.Limit != 25 && f.Limit != 50) {
			return f, planningBadRequest("invalid_limit", "limite non valido")
		}
	}
	if q.Get("offset") != "" {
		f.Offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil || f.Offset < 0 {
			return f, planningBadRequest("invalid_offset", "offset non valido")
		}
	}
	return f, nil
}
func parsePlanningItemsFilter(r *http.Request) (PlanningItemsFilter, error) {
	q := r.URL.Query()
	f := PlanningItemsFilter{Kind: strings.TrimSpace(q.Get("kind")), Q: strings.TrimSpace(q.Get("q")), TeamID: strings.TrimSpace(q.Get("teamId")), EventID: strings.TrimSpace(q.Get("eventId")), Status: strings.TrimSpace(q.Get("status")), Limit: 25}
	if f.Kind != "requests" && f.Kind != "enrollments" && f.Kind != "events" && f.Kind != "reminders" {
		return f, planningBadRequest("invalid_kind", "sezione non valida")
	}
	// Section filters are not silently ignored: this prevents URLs from
	// appearing to narrow a drawer while returning unrelated facts.
	if (f.Kind == "requests" && f.EventID != "") ||
		(f.Kind == "events" && (f.TeamID != "" || f.EventID != "")) ||
		(f.Kind == "reminders" && f.TeamID != "") {
		return f, planningBadRequest("inapplicable_filter", "filtro non applicabile alla sezione")
	}
	if utf8.RuneCountInString(f.Q) > 200 {
		return f, planningBadRequest("invalid_filter", "filtro testo troppo lungo")
	}
	for _, v := range []string{f.TeamID, f.EventID} {
		if v != "" {
			if _, err := uuid.Parse(v); err != nil {
				return f, planningBadRequest("invalid_id", "id filtro non valido")
			}
		}
	}
	valid := map[string]map[string]bool{"requests": {"open": true, "suspended": true, "accepted": true, "rejected": true, "withdrawn": true}, "enrollments": {"planned": true, "in_progress": true, "completed": true, "partially_completed": true, "not_attended": true, "cancelled": true}, "events": {"operational": true, "terminal": true, "cancelled": true}, "reminders": {"due": true, "future": true, "undated": true, "suspended": true}}
	if f.Status != "" && !valid[f.Kind][f.Status] {
		return f, planningBadRequest("invalid_status", "stato non valido per la sezione")
	}
	var err error
	if q.Get("limit") != "" {
		f.Limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || (f.Limit != 25 && f.Limit != 50) {
			return f, planningBadRequest("invalid_limit", "limite non valido")
		}
	}
	if q.Get("offset") != "" {
		f.Offset, err = strconv.Atoi(q.Get("offset"))
		if err != nil || f.Offset < 0 {
			return f, planningBadRequest("invalid_offset", "offset non valido")
		}
	}
	return f, nil
}
func parsePlanningUUID(raw string) (string, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", planningBadRequest("invalid_id", "id non valido")
	}
	return id.String(), nil
}
func (h *handler) registerPlanningRoutes(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	mux.Handle("GET /training/v1/planning", protect(h.requireStore(http.HandlerFunc(h.handlePlanningList))))
	mux.Handle("GET /training/v1/planning/filters", protect(h.requireStore(http.HandlerFunc(h.handlePlanningFilters))))
	mux.Handle("GET /training/v1/planning/courses/{id}", protect(h.requireStore(http.HandlerFunc(h.handlePlanningCourseDetail))))
	mux.Handle("GET /training/v1/planning/courses/{id}/items", protect(h.requireStore(http.HandlerFunc(h.handlePlanningItems))))
	mux.Handle("PUT /training/v1/reminders/{kind}/{id}", protect(h.requireStore(http.HandlerFunc(h.handleUpdatePlanningReminder))))
}

const planningRequestTimeout = 20 * time.Second

func (h *handler) planningContext(r *http.Request) (context.Context, context.CancelFunc, string, string, error) {
	ctx, cancel := context.WithTimeout(r.Context(), planningRequestTimeout)
	now := time.Now()
	today, err := planningToday(now)
	if err != nil {
		cancel()
		return nil, nil, "", "", fmt.Errorf("resolve planning date: %w", err)
	}
	return ctx, cancel, today, now.UTC().Format(time.RFC3339), nil
}

func (h *handler) planningLog(operation string, started time.Time, count int) {
	h.logger.Info("training planning operation", "operation", operation, "duration", time.Since(started), "count", count)
}

// hydratePlanningPOs does no work when there are no expenses, allowing fully
// local planning views to work without an Arak client. Each ID is fetched once.
func (h *handler) hydratePlanningPOs(ctx context.Context, email string, ids []int64) (map[int64]string, error) {
	states := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return states, nil
	}
	if h.arak == nil {
		return nil, serviceUnavailableError("arak_not_configured", "servizio PO temporaneamente non disponibile")
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan int64)
	var wg sync.WaitGroup
	var once sync.Once
	var mu sync.Mutex
	var firstErr error
	workers := 4
	if len(ids) < workers {
		workers = len(ids)
	}
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-workCtx.Done():
					return
				case id, ok := <-jobs:
					if !ok {
						return
					}
					po, err := h.getPurchaseOrder(workCtx, email, id)
					if err != nil {
						once.Do(func() { firstErr = fmt.Errorf("hydrate planning PO %d: %w", id, err); cancel() })
						return
					}
					mu.Lock()
					states[id] = string(classifyEconomicState(po.RawState))
					mu.Unlock()
				}
			}
		}()
	}
	for _, id := range ids {
		select {
		case <-workCtx.Done():
			break
		case jobs <- id:
		}
		if workCtx.Err() != nil {
			break
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("planning PO hydration: %w", err)
	}
	return states, nil
}

func (h *handler) handlePlanningList(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	count := 0
	defer func() { h.planningLog("training.planning.list", started, count) }()
	ctx, cancel, today, generatedAt, err := h.planningContext(r)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.list")
		return
	}
	defer cancel()
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	filter, err := parsePlanningListFilter(r)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.list")
		return
	}
	snapshot, err := h.store.loadPlanningSnapshot(ctx, "")
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.list")
		return
	}
	candidate, err := selectPlanningCandidates(ctx, snapshot, filter)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.list")
		return
	}
	states, err := h.hydratePlanningPOs(ctx, principal.Email, candidate.POIDs)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.list")
		return
	}
	response, err := projectPlanningList(ctx, candidate, states, today, generatedAt, filter)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.list")
		return
	}
	count = response.Total
	if err := ctx.Err(); err != nil {
		h.writeActionError(w, r, err, "training.planning.list")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handlePlanningFilters(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	defer func() { h.planningLog("training.planning.filters", started, 0) }()
	ctx, cancel, _, _, err := h.planningContext(r)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.filters")
		return
	}
	defer cancel()
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	snapshot, err := h.store.loadPlanningSnapshot(ctx, "")
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.filters")
		return
	}
	response, err := projectPlanningFilters(ctx, snapshot)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.filters")
		return
	}
	if err := ctx.Err(); err != nil {
		h.writeActionError(w, r, err, "training.planning.filters")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handlePlanningCourseDetail(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	count := 0
	defer func() { h.planningLog("training.planning.detail", started, count) }()
	ctx, cancel, today, _, err := h.planningContext(r)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.detail")
		return
	}
	defer cancel()
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	courseID, err := parsePlanningUUID(r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.detail")
		return
	}
	snapshot, err := h.store.loadPlanningSnapshot(ctx, courseID)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.detail")
		return
	}
	candidate, err := restrictPlanningSnapshot(ctx, snapshot, map[string]bool{courseID: true})
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.detail")
		return
	}
	states, err := h.hydratePlanningPOs(ctx, principal.Email, candidate.POIDs)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.detail")
		return
	}
	response, err := projectPlanningCourseDetail(ctx, candidate.Snapshot, states, today, courseID)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.detail")
		return
	}
	count = 1
	if err := ctx.Err(); err != nil {
		h.writeActionError(w, r, err, "training.planning.detail")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handlePlanningItems(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	count := 0
	defer func() { h.planningLog("training.planning.items", started, count) }()
	ctx, cancel, today, _, err := h.planningContext(r)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.items")
		return
	}
	defer cancel()
	if _, ok := h.principalOrUnauthorized(w, r); !ok {
		return
	}
	courseID, err := parsePlanningUUID(r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.items")
		return
	}
	filter, err := parsePlanningItemsFilter(r)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.items")
		return
	}
	snapshot, err := h.store.loadPlanningSnapshot(ctx, courseID)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.items")
		return
	}
	response, err := projectPlanningItems(ctx, snapshot, today, courseID, filter)
	if err != nil {
		h.writeActionError(w, r, err, "training.planning.items")
		return
	}
	count = response.Total
	if err := ctx.Err(); err != nil {
		h.writeActionError(w, r, err, "training.planning.items")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleUpdatePlanningReminder(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	count := 0
	defer func() { h.planningLog("training.reminder.update", started, count) }()
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	kind := strings.TrimSpace(r.PathValue("kind"))
	if kind != "course" && kind != "request" && kind != "event" {
		h.writeActionError(w, r, planningBadRequest("invalid_kind", "tipo promemoria non valido"), "training.reminder.update")
		return
	}
	id, err := parsePlanningUUID(r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.reminder.update")
		return
	}
	input, err := decodeReminderUpdate(r)
	if err != nil {
		h.writeActionError(w, r, err, "training.reminder.update")
		return
	}
	response, err := h.store.UpdatePlanningReminder(r.Context(), principal, kind, id, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.reminder.update")
		return
	}
	count = 1
	httputil.JSON(w, http.StatusOK, response)
}

func decodeReminderUpdate(r *http.Request) (ReminderUpdateInput, error) {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil || raw == nil {
		return ReminderUpdateInput{}, planningBadRequest("invalid_json", "JSON non valido")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return ReminderUpdateInput{}, planningBadRequest("invalid_json", "JSON non valido")
	}
	if len(raw) != 2 || raw["text"] == nil || raw["date"] == nil {
		return ReminderUpdateInput{}, planningBadRequest("invalid_reminder", "text e date sono obbligatori")
	}
	var textValue *string
	if err := json.Unmarshal(raw["text"], &textValue); err != nil || textValue == nil {
		return ReminderUpdateInput{}, planningBadRequest("invalid_reminder", "testo non valido")
	}
	text := strings.TrimSpace(*textValue)
	if utf8.RuneCountInString(text) > 2000 {
		return ReminderUpdateInput{}, planningBadRequest("invalid_reminder", "testo troppo lungo")
	}
	in := ReminderUpdateInput{Text: &text}
	var dateValue *string
	if err := json.Unmarshal(raw["date"], &dateValue); err != nil {
		return in, planningBadRequest("invalid_reminder_date", "data non valida")
	}
	if dateValue != nil {
		if _, err := time.Parse("2006-01-02", *dateValue); err != nil {
			return in, planningBadRequest("invalid_reminder_date", "data non valida")
		}
		in.Date = dateValue
	}
	if text == "" {
		in.Date = nil
	}
	return in, nil
}
