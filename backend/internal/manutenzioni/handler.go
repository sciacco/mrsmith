package manutenzioni

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/acl"
	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/authz"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/openrouter"
)

type Deps struct {
	Maintenance *sql.DB
	Mistra      *sql.DB
	AI          *openrouter.Client
	Logger      *slog.Logger
}

type Handler struct {
	maintenance *sql.DB
	mistra      *sql.DB
	ai          *openrouter.Client
	logger      *slog.Logger
}

var (
	errBadRequest = errors.New("bad request")
	codePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

func RegisterRoutes(mux *http.ServeMux, deps Deps) {
	h := &Handler{
		maintenance: deps.Maintenance,
		mistra:      deps.Mistra,
		ai:          deps.AI,
		logger:      deps.Logger,
	}

	accessProtect := acl.RequireRole(applaunch.ManutenzioniAccessRoles()...)
	managerProtect := acl.RequireRole(applaunch.ManutenzioniManagerRoles()...)
	actionProtect := acl.RequireRole(combineRoles(
		applaunch.ManutenzioniManagerRoles(),
		applaunch.ManutenzioniApproverRoles(),
	)...)

	access := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, accessProtect(http.HandlerFunc(handler)))
	}
	manager := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, managerProtect(http.HandlerFunc(handler)))
	}
	action := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, actionProtect(http.HandlerFunc(handler)))
	}

	access("GET /manutenzioni/v1/maintenances", h.handleListMaintenances)
	access("GET /manutenzioni/v1/maintenances/{id}", h.handleGetMaintenance)
	access("GET /manutenzioni/v1/maintenances/{id}/events", h.handleGetEvents)
	access("GET /manutenzioni/v1/reference-data", h.handleReferenceData)
	access("GET /manutenzioni/v1/service-dependencies", h.handleListServiceDependencies)

	manager("GET /manutenzioni/v1/customers", h.handleSearchCustomers)
	manager("POST /manutenzioni/v1/maintenances", h.handleCreateMaintenance)
	manager("PATCH /manutenzioni/v1/maintenances/{id}", h.handleUpdateMaintenance)
	manager("POST /manutenzioni/v1/maintenances/{id}/assistance/draft", h.handleDraftAssistance)
	manager("POST /manutenzioni/v1/maintenances/assistance/preview", h.handlePreviewAssistance)
	action("POST /manutenzioni/v1/maintenances/{id}/status", h.handleMaintenanceStatus)

	manager("POST /manutenzioni/v1/maintenances/{id}/windows", h.handleCreateWindow)
	manager("PATCH /manutenzioni/v1/maintenances/{id}/windows/{windowId}", h.handleUpdateWindow)
	manager("POST /manutenzioni/v1/maintenances/{id}/windows/{windowId}/cancel", h.handleCancelWindow)
	manager("POST /manutenzioni/v1/maintenances/{id}/windows/reschedule", h.handleRescheduleWindow)

	manager("PUT /manutenzioni/v1/maintenances/{id}/service-taxonomy", h.handleReplaceServiceTaxonomy)
	manager("PUT /manutenzioni/v1/maintenances/{id}/reason-classes", h.handleReplaceReasonClasses)
	manager("PUT /manutenzioni/v1/maintenances/{id}/impact-effects", h.handleReplaceImpactEffects)
	manager("PUT /manutenzioni/v1/maintenances/{id}/quality-flags", h.handleReplaceQualityFlags)

	manager("POST /manutenzioni/v1/maintenances/{id}/targets", h.handleCreateTarget)
	manager("PATCH /manutenzioni/v1/maintenances/{id}/targets/{targetId}", h.handleUpdateTarget)
	manager("DELETE /manutenzioni/v1/maintenances/{id}/targets/{targetId}", h.handleDeleteTarget)

	manager("POST /manutenzioni/v1/maintenances/{id}/impacted-customers", h.handleCreateImpactedCustomer)
	manager("PATCH /manutenzioni/v1/maintenances/{id}/impacted-customers/{customerImpactId}", h.handleUpdateImpactedCustomer)
	manager("DELETE /manutenzioni/v1/maintenances/{id}/impacted-customers/{customerImpactId}", h.handleDeleteImpactedCustomer)

	manager("POST /manutenzioni/v1/maintenances/{id}/notices", h.handleCreateNotice)
	manager("PATCH /manutenzioni/v1/maintenances/{id}/notices/{noticeId}", h.handleUpdateNotice)
	manager("PUT /manutenzioni/v1/maintenances/{id}/notices/{noticeId}/locales/{locale}", h.handleUpsertNoticeLocale)
	manager("POST /manutenzioni/v1/maintenances/{id}/notices/{noticeId}/status", h.handleNoticeStatus)
	manager("PUT /manutenzioni/v1/maintenances/{id}/notices/{noticeId}/quality-flags", h.handleReplaceNoticeQualityFlags)

	action("GET /manutenzioni/v1/llm-models", h.handleListLLMModels)
	action("POST /manutenzioni/v1/llm-models", h.handleCreateLLMModel)
	action("PATCH /manutenzioni/v1/llm-models/{scope}", h.handleUpdateLLMModel)

	action("GET /manutenzioni/v1/service-dependencies/{id}", h.handleGetServiceDependency)
	action("POST /manutenzioni/v1/service-dependencies", h.handleCreateServiceDependency)
	action("PATCH /manutenzioni/v1/service-dependencies/{id}", h.handleUpdateServiceDependency)
	action("POST /manutenzioni/v1/service-dependencies/{id}/deactivate", h.handleDeactivateServiceDependency)
	action("POST /manutenzioni/v1/service-dependencies/{id}/reactivate", h.handleReactivateServiceDependency)

	action("GET /manutenzioni/v1/config/summary", h.handleConfigSummary)
	action("GET /manutenzioni/v1/config/{resource}", h.handleListConfig)
	action("POST /manutenzioni/v1/config/{resource}", h.handleCreateConfig)
	action("POST /manutenzioni/v1/config/{resource}/reorder", h.handleReorderConfig)
	action("PATCH /manutenzioni/v1/config/{resource}/{id}", h.handleUpdateConfig)
	action("POST /manutenzioni/v1/config/{resource}/{id}/deactivate", h.handleDeactivateConfig)
	action("POST /manutenzioni/v1/config/{resource}/{id}/reactivate", h.handleReactivateConfig)
	action("GET /manutenzioni/v1/config/{resource}/{id}/usage", h.handleConfigUsage)
}

func combineRoles(groups ...[]string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0)
	for _, group := range groups {
		for _, role := range group {
			if _, ok := seen[role]; ok {
				continue
			}
			seen[role] = struct{}{}
			result = append(result, role)
		}
	}
	return result
}

func (h *Handler) requireMaintenanceDB(w http.ResponseWriter) bool {
	if h.maintenance == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "manutenzioni_database_not_configured")
		return false
	}
	return true
}

func (h *Handler) requireMistraDB(w http.ResponseWriter) bool {
	if h.mistra == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "customer_lookup_not_configured")
		return false
	}
	return true
}

func (h *Handler) dbFailure(w http.ResponseWriter, r *http.Request, operation string, err error, attrs ...any) {
	args := []any{"component", "manutenzioni", "operation", operation}
	args = append(args, attrs...)
	httputil.InternalError(w, r, err, "database operation failed", args...)
}

func decodeBody(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

func pathInt64(r *http.Request, name string) (int64, error) {
	value, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || value <= 0 {
		return 0, errBadRequest
	}
	return value, nil
}

func queryPositiveInt(r *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func placeholder(args *[]any, value any) string {
	*args = append(*args, value)
	return "$" + strconv.Itoa(len(*args))
}

func nullIfEmpty(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func nullStringPtr(value *string) any {
	if value == nil {
		return nil
	}
	return nullIfEmpty(*value)
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func intPtr(value int) *int {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func floatPtr(value float64) *float64 {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func nullStringValue(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return stringPtr(value.String)
}

func nullIntValue(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullInt64Value(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func nullFloatValue(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func nullTimeValue(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func rawJSONOrDefault(raw json.RawMessage) string {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" {
		return "{}"
	}
	if !json.Valid(raw) {
		return "{}"
	}
	return string(raw)
}

func rawJSONFromBytes(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(raw)
}

func parseOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}
	t, err := parseTimeValue(trimmed)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseRequiredTime(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, errBadRequest
	}
	return parseTimeValue(trimmed)
}

func parseTimeValue(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04", raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	return time.Time{}, errBadRequest
}

func validateWindowRequest(body windowRequest) (time.Time, time.Time, *time.Time, *time.Time, *time.Time, *time.Time, error) {
	start, err := parseRequiredTime(body.ScheduledStartAt)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, err
	}
	end, err := parseRequiredTime(body.ScheduledEndAt)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, err
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, errBadRequest
	}
	actualStart, err := parseOptionalTime(body.ActualStartAt)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, err
	}
	actualEnd, err := parseOptionalTime(body.ActualEndAt)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, err
	}
	if actualStart != nil && actualEnd != nil && !actualEnd.After(*actualStart) {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, errBadRequest
	}
	announcedAt, err := parseOptionalTime(body.AnnouncedAt)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, err
	}
	lastNoticeAt, err := parseOptionalTime(body.LastNoticeAt)
	if err != nil {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, err
	}
	if body.ExpectedDowntimeMinutes != nil && *body.ExpectedDowntimeMinutes < 0 {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, errBadRequest
	}
	if body.ActualDowntimeMinutes != nil && *body.ActualDowntimeMinutes < 0 {
		return time.Time{}, time.Time{}, nil, nil, nil, nil, errBadRequest
	}
	return start, end, actualStart, actualEnd, announcedAt, lastNoticeAt, nil
}

func claimsActor(r *http.Request) map[string]any {
	claims, _ := auth.GetClaims(r.Context())
	return map[string]any{
		"subject": claims.Subject,
		"email":   claims.Email,
		"name":    claims.Name,
	}
}

func canManage(r *http.Request) bool {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		return false
	}
	return authz.HasAnyRole(claims.Roles, applaunch.ManutenzioniManagerRoles()...)
}

func canApprove(r *http.Request) bool {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		return false
	}
	return authz.HasAnyRole(claims.Roles, applaunch.ManutenzioniApproverRoles()...)
}

type eventWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func writeEvent(ctx context.Context, exec eventWriter, maintenanceID int64, windowID *int64, eventType string, summary string, actor map[string]any, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	if actor != nil {
		payload["actor"] = actor
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(
		ctx,
		`INSERT INTO maintenance.maintenance_event (
			maintenance_id,
			maintenance_window_id,
			event_type,
			actor_type,
			summary,
			payload
		) VALUES ($1, $2, $3, 'user', $4, $5::jsonb)`,
		maintenanceID,
		windowID,
		eventType,
		nullIfEmpty(summary),
		string(payloadBytes),
	)
	return err
}

func ensureMaintenanceExists(ctx context.Context, q queryer, maintenanceID int64) error {
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM maintenance.maintenance WHERE maintenance_id = $1)`, maintenanceID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return sql.ErrNoRows
	}
	return nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validateCode(code string, allowSiteCode bool) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	if allowSiteCode {
		return true
	}
	return codePattern.MatchString(code)
}

func respondMutationDetail(h *Handler, w http.ResponseWriter, r *http.Request, maintenanceID int64, status int) {
	detail, err := h.loadMaintenanceDetail(r.Context(), maintenanceID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "maintenance_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "load_mutation_detail", err, "maintenance_id", maintenanceID)
		return
	}
	httputil.JSON(w, status, detail)
}

func appError(w http.ResponseWriter, status int, code string) {
	httputil.Error(w, status, code)
}

func normalizeSource(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "manual"
	}
	return value
}

func defaultIfEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func formatMaintenanceCode(year int, id int64) string {
	return fmt.Sprintf("MNT-%04d-%06d", year, id)
}
