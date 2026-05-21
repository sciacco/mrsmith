package training

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *handler) writeActionError(w http.ResponseWriter, r *http.Request, err error, operation string) {
	if appErr, ok := asAppError(err); ok {
		httputil.JSON(w, appErr.status, map[string]string{
			"error":   appErr.code,
			"message": appErr.message,
		})
		return
	}
	httputil.InternalError(w, r, err, "training request failed", "operation", operation)
}

func decodeJSONBody[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var input T
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return input, false
	}
	return input, true
}

func (h *handler) principalOrUnauthorized(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	principal, ok := principalFromRequest(r)
	if !ok {
		httputil.Error(w, http.StatusUnauthorized, "missing_auth_claims")
		return Principal{}, false
	}
	return principal, true
}

func (h *handler) handleCreateEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EnrollmentInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateEnrollment(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_enrollment")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleUpdateEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EnrollmentInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateEnrollment(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_enrollment")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleTransitionEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[EnrollmentTransitionInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.TransitionEnrollment(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.transition_enrollment")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[TrainingRequestInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateTrainingRequest(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_request")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleTransitionRequest(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[TrainingRequestTransitionInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.TransitionTrainingRequest(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.transition_request")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleCreateAward(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[AwardInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.CreateAward(r.Context(), principal, input)
	if err != nil {
		h.writeActionError(w, r, err, "training.create_award")
		return
	}
	httputil.JSON(w, http.StatusCreated, response)
}

func (h *handler) handleUpdateAward(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	input, ok := decodeJSONBody[AwardUpdateInput](w, r)
	if !ok {
		return
	}
	response, err := h.store.UpdateAward(r.Context(), principal, r.PathValue("id"), input)
	if err != nil {
		h.writeActionError(w, r, err, "training.update_award")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleUpsertVendor(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[VendorInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertVendor(r.Context(), principal, id, input)
	})
}

func (h *handler) handleUpsertTeam(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[TeamInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertTeam(r.Context(), principal, id, input)
	})
}

func (h *handler) handleUpsertSkillArea(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[SkillAreaInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertSkillArea(r.Context(), principal, id, input)
	})
}

func (h *handler) handleUpsertCertification(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[CertificationInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertCertification(r.Context(), principal, id, input)
	})
}

func (h *handler) handleUpsertCourse(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[CourseInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertCourse(r.Context(), principal, id, input)
	})
}

func (h *handler) handleUpsertPlan(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[TrainingPlanInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertTrainingPlan(r.Context(), principal, id, input)
	})
}

func (h *handler) handleUpsertMandatoryRule(w http.ResponseWriter, r *http.Request) {
	h.handleUpsert(w, r, func(principal Principal, id string) (ActionResponse, error) {
		input, ok := decodeJSONBody[MandatoryRuleInput](w, r)
		if !ok {
			return ActionResponse{}, nil
		}
		return h.store.UpsertMandatoryRule(r.Context(), principal, id, input)
	})
}

func (h *handler) handleUpsert(w http.ResponseWriter, r *http.Request, fn func(Principal, string) (ActionResponse, error)) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := fn(principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.upsert_master_data")
		return
	}
	if !response.OK && response.ID == "" {
		return
	}
	status := http.StatusOK
	if r.Method == http.MethodPost {
		status = http.StatusCreated
	}
	httputil.JSON(w, status, response)
}

func (h *handler) handleUploadEnrollmentDocument(w http.ResponseWriter, r *http.Request) {
	h.handleUploadDocument(w, r, r.PathValue("id"), "")
}

func (h *handler) handleUploadAwardDocument(w http.ResponseWriter, r *http.Request) {
	h.handleUploadDocument(w, r, "", r.PathValue("id"))
}

func (h *handler) handleUploadDocument(w http.ResponseWriter, r *http.Request, enrollmentID string, awardID string) {
	if h.storage == nil {
		httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "training_storage_not_configured"})
		return
	}
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.storageMaxBytes+(1<<20))
	if err := r.ParseMultipartForm(h.storageMaxBytes); err != nil {
		httputil.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_upload"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httputil.JSON(w, http.StatusBadRequest, map[string]string{"error": "file_required"})
		return
	}
	defer file.Close()

	content, mimeType, err := readPDFUpload(file, header.Header.Get("Content-Type"), h.storageMaxBytes)
	if err != nil {
		h.writeActionError(w, r, err, "training.upload_document")
		return
	}
	key := documentStorageKey(enrollmentID, awardID, header.Filename)
	stored, err := h.storage.Put(r.Context(), key, bytes.NewReader(content), h.storageMaxBytes)
	if err != nil {
		h.writeActionError(w, r, err, "training.store_document")
		return
	}
	meta, err := h.store.InsertDocument(r.Context(), principal, enrollmentID, awardID, cleanFilename(header.Filename), mimeType, stored)
	if err != nil {
		_ = h.storage.Delete(r.Context(), stored.Key)
		h.writeActionError(w, r, err, "training.insert_document")
		return
	}
	httputil.JSON(w, http.StatusCreated, meta)
}

func readPDFUpload(file io.Reader, rawContentType string, maxBytes int64) ([]byte, string, error) {
	content, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(content)) > maxBytes {
		return nil, "", validationError("document_too_large", "documento troppo grande")
	}
	if len(content) == 0 {
		return nil, "", validationError("empty_document", "documento vuoto")
	}
	detected := http.DetectContentType(content)
	mediaType := strings.TrimSpace(rawContentType)
	if mediaType != "" {
		if parsed, _, err := mime.ParseMediaType(mediaType); err == nil {
			mediaType = parsed
		}
	}
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = detected
	}
	if detected != "application/pdf" {
		return nil, "", validationError("unsupported_document_type", "sono ammessi solo PDF")
	}
	return content, "application/pdf", nil
}

func documentStorageKey(enrollmentID string, awardID string, filename string) string {
	scope := "documents"
	id := strings.TrimSpace(enrollmentID)
	if id != "" {
		scope = "enrollments"
	} else {
		id = strings.TrimSpace(awardID)
		scope = "awards"
	}
	return fmt.Sprintf("%s/%s/%d-%s", scope, id, time.Now().UTC().UnixNano(), cleanFilename(filename))
}

func cleanFilename(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "/" || filename == "" {
		return "attestato.pdf"
	}
	return strings.ReplaceAll(filename, `"`, "")
}

func (h *handler) handleDownloadDocument(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	doc, err := h.store.DocumentForAccess(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.download_document")
		return
	}
	if h.storage == nil {
		httputil.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": "training_storage_not_configured"})
		return
	}
	body, err := h.storage.Get(r.Context(), doc.StorageKey)
	if err != nil {
		h.writeActionError(w, r, err, "training.get_document")
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", doc.MIME)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, doc.Filename))
	w.Header().Set("X-Content-SHA256", doc.SHA256)
	_, _ = io.Copy(w, body)
}

func (h *handler) handleValidateDocument(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	response, err := h.store.ValidateDocument(r.Context(), principal, r.PathValue("id"))
	if err != nil {
		h.writeActionError(w, r, err, "training.validate_document")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleRunJobs(w http.ResponseWriter, r *http.Request) {
	runner := NewJobRunner(h.store, h.notifier, h.logger, h.trainingAppURL)
	response, err := runner.RunOnce(r.Context())
	if err != nil {
		h.writeActionError(w, r, err, "training.jobs")
		return
	}
	httputil.JSON(w, http.StatusOK, response)
}

func (h *handler) handleExport(w http.ResponseWriter, r *http.Request) {
	principal, ok := h.principalOrUnauthorized(w, r)
	if !ok {
		return
	}
	query := map[string]string{
		"q":      r.URL.Query().Get("q"),
		"team":   r.URL.Query().Get("team"),
		"status": r.URL.Query().Get("status"),
		"year":   r.URL.Query().Get("year"),
	}
	kind := strings.TrimSuffix(strings.TrimSpace(r.PathValue("kind")), ".xlsx")
	var headers []string
	var rows [][]string
	switch kind {
	case "plan":
		items, err := h.store.ListPlanEnrollments(r.Context(), principal)
		if err != nil {
			h.writeActionError(w, r, err, "training.export_plan")
			return
		}
		headers = []string{"Persona", "Email", "Team", "Corso", "Fornitore", "Area", "Stato", "Anno", "Inizio", "Fine", "Ore", "Costo", "Obbligatoria", "Documento", "Validato"}
		for _, item := range filterPlanRows(items, query) {
			rows = append(rows, []string{item.EmployeeName, item.EmployeeEmail, item.TeamCode, item.CourseTitle, item.VendorName, item.SkillAreaName, item.Status, fmt.Sprint(item.Year), item.PlannedStart, item.PlannedEnd, intString(item.HoursPlanned), floatString(item.CostPlanned), fmt.Sprint(item.Mandatory), item.DocumentFilename, fmt.Sprint(item.DocumentValidated)})
		}
	case "requests":
		items, err := h.store.ListRequests(r.Context(), principal)
		if err != nil {
			h.writeActionError(w, r, err, "training.export_requests")
			return
		}
		headers = []string{"Persona", "Email", "Corso", "Titolo libero", "Area", "Motivazione", "Anno", "Stato", "Creata"}
		for _, item := range filterRequestRows(items, query) {
			rows = append(rows, []string{item.EmployeeName, item.EmployeeEmail, item.CourseTitle, item.FreeTextTitle, item.SkillAreaName, item.Motivation, intString(item.DesiredYear), item.Status, item.CreatedAt})
		}
	case "catalog":
		items, err := h.store.ListCatalogCourses(r.Context())
		if err != nil {
			h.writeActionError(w, r, err, "training.export_catalog")
			return
		}
		headers = []string{"Corso", "Fornitore", "Area", "Certificazione", "Modalita", "Origine", "Ore", "Costo", "URL", "Descrizione", "Obbligatoria", "Ricorrenza mesi", "Ambito", "Attiva"}
		for _, item := range filterCatalogRows(items, query) {
			rows = append(rows, []string{item.Title, item.VendorName, item.SkillAreaName, item.CertificationName, item.DeliveryMode, item.ProviderKind, intString(item.DefaultHours), floatString(item.DefaultCost), item.CourseURL, item.Description, fmt.Sprint(item.Mandatory), intString(item.RecurrenceMonths), item.ComplianceFramework, fmt.Sprint(item.Active)})
		}
	case "certifications":
		items, err := h.store.ListCertifications(r.Context(), principal)
		if err != nil {
			h.writeActionError(w, r, err, "training.export_certifications")
			return
		}
		headers = []string{"Persona", "Email", "Codice", "Certificazione", "Esito", "Data", "Scadenza", "Stato", "Fonte", "Attestato", "Validato"}
		for _, item := range filterCertificationRows(items, query) {
			rows = append(rows, []string{item.EmployeeName, item.EmployeeEmail, item.CertificationCode, item.CertificationName, item.Outcome, item.AwardedOn, item.ExpiresOn, item.CurrentStatus, item.ValidationSource, item.DocumentFilename, fmt.Sprint(item.DocumentValidated)})
		}
	case "plan-budget":
		if !principal.IsPeopleAdmin {
			h.writeActionError(w, r, forbiddenError("people_role_required", "azione riservata a People"), "training.export_plan_budget")
			return
		}
		items, err := h.store.ListPlanBudget(r.Context())
		if err != nil {
			h.writeActionError(w, r, err, "training.export_plan_budget")
			return
		}
		headers = []string{"Anno", "Team", "Iscrizioni", "Costo", "Ore"}
		for _, item := range items {
			rows = append(rows, []string{fmt.Sprint(item.Year), item.TeamCode, fmt.Sprint(item.EnrollmentsCount), floatString(item.CostTotal), floatString(item.HoursTotal)})
		}
	case "expiring-certifications":
		items, err := h.store.ListExpiringCertifications(r.Context(), principal)
		if err != nil {
			h.writeActionError(w, r, err, "training.export_expiring")
			return
		}
		headers = []string{"Persona", "Email", "Codice", "Certificazione", "Scadenza", "Giorni"}
		for _, item := range items {
			rows = append(rows, []string{item.EmployeeName, item.EmployeeEmail, item.CertificationCode, item.CertificationName, item.ExpiresOn, fmt.Sprint(item.DaysToExpiry)})
		}
	case "compliance-gaps":
		items, err := h.store.ListComplianceGaps(r.Context(), principal)
		if err != nil {
			h.writeActionError(w, r, err, "training.export_gaps")
			return
		}
		headers = []string{"Persona", "Corso", "Ambito", "Ultimo superamento", "Stato"}
		for _, item := range items {
			rows = append(rows, []string{item.EmployeeName, item.CourseTitle, item.ComplianceFramework, item.LastValidAwardedOn, item.ComplianceStatus})
		}
	default:
		httputil.JSON(w, http.StatusNotFound, map[string]string{"error": "export_not_found"})
		return
	}
	if err := writeTrainingXLSX(w, "formazione-"+kind+".xlsx", headers, rows); err != nil {
		h.writeActionError(w, r, err, "training.write_export")
	}
}
