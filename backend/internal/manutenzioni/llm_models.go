package manutenzioni

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const (
	llmModelScopeDefault         = "default"
	llmModelScopeAssistanceDraft = "assistance_draft"
)

var errLLMModelUnavailable = errors.New("llm model unavailable")

func (h *Handler) handleListLLMModels(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	rows, err := h.maintenance.QueryContext(
		r.Context(),
		`SELECT scope, model FROM maintenance.llm_model ORDER BY scope`,
	)
	if err != nil {
		h.dbFailure(w, r, "llm_models_list", err)
		return
	}
	defer rows.Close()

	models := []LLMModel{}
	for rows.Next() {
		model, err := scanLLMModel(rows)
		if err != nil {
			h.dbFailure(w, r, "llm_models_list_scan", err)
			return
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		h.dbFailure(w, r, "llm_models_list_rows", err)
		return
	}
	httputil.JSON(w, http.StatusOK, models)
}

func (h *Handler) handleCreateLLMModel(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	var body llmModelRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	scope := strings.TrimSpace(body.Scope)
	model := strings.TrimSpace(body.Model)
	if !codePattern.MatchString(scope) {
		appError(w, http.StatusBadRequest, "invalid_llm_model_scope")
		return
	}
	if model == "" {
		appError(w, http.StatusBadRequest, "llm_model_model_required")
		return
	}

	item, err := h.queryLLMModel(
		r,
		`INSERT INTO maintenance.llm_model (scope, model)
		VALUES ($1, $2)
		RETURNING scope, model`,
		scope,
		model,
	)
	if isUniqueViolation(err) {
		appError(w, http.StatusConflict, "llm_model_already_exists")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "llm_model_create", err, "scope", scope)
		return
	}
	httputil.JSON(w, http.StatusCreated, item)
}

func (h *Handler) handleUpdateLLMModel(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	scope := strings.TrimSpace(r.PathValue("scope"))
	if !codePattern.MatchString(scope) {
		appError(w, http.StatusBadRequest, "invalid_llm_model_scope")
		return
	}
	var body llmModelRequest
	if err := decodeBody(r, &body); err != nil {
		appError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	bodyScope := strings.TrimSpace(body.Scope)
	if bodyScope != "" && bodyScope != scope {
		appError(w, http.StatusBadRequest, "llm_model_scope_immutable")
		return
	}
	model := strings.TrimSpace(body.Model)
	if model == "" {
		appError(w, http.StatusBadRequest, "llm_model_model_required")
		return
	}

	item, err := h.queryLLMModel(
		r,
		`UPDATE maintenance.llm_model
		SET model = $1
		WHERE scope = $2
		RETURNING scope, model`,
		model,
		scope,
	)
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "llm_model_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "llm_model_update", err, "scope", scope)
		return
	}
	httputil.JSON(w, http.StatusOK, item)
}

func (h *Handler) queryLLMModel(r *http.Request, query string, args ...any) (LLMModel, error) {
	return scanLLMModel(h.maintenance.QueryRowContext(r.Context(), query, args...))
}

func scanLLMModel(scanner interface {
	Scan(dest ...any) error
}) (LLMModel, error) {
	var item LLMModel
	if err := scanner.Scan(&item.Scope, &item.Model); err != nil {
		return item, err
	}
	item.Scope = strings.TrimSpace(item.Scope)
	item.Model = strings.TrimSpace(item.Model)
	return item, nil
}

func (h *Handler) resolveLLMModel(ctx context.Context, scope string) (string, error) {
	scope = strings.TrimSpace(scope)
	if !codePattern.MatchString(scope) {
		return "", errBadRequest
	}
	if h.maintenance == nil {
		return "", errLLMModelUnavailable
	}
	model, err := h.loadLLMModel(ctx, scope)
	if err == nil {
		return model, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if scope == llmModelScopeDefault {
		return "", errLLMModelUnavailable
	}
	model, err = h.loadLLMModel(ctx, llmModelScopeDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errLLMModelUnavailable
	}
	if err != nil {
		return "", err
	}
	return model, nil
}

func (h *Handler) loadLLMModel(ctx context.Context, scope string) (string, error) {
	var model string
	if err := h.maintenance.QueryRowContext(ctx, `SELECT model FROM maintenance.llm_model WHERE scope = $1`, scope).Scan(&model); err != nil {
		return "", err
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", errLLMModelUnavailable
	}
	return model, nil
}
