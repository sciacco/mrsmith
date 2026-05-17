package rda

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const providerQualificationReferenceType = "QUALIFICATION_REF"

var providerReferencePhonePattern = regexp.MustCompile(`^\+[1-9][0-9]{4,19}$`)

var allowedRDAProviderReferenceTypes = map[string]struct{}{
	"OTHER_REF":          {},
	"ADMINISTRATIVE_REF": {},
	"TECHNICAL_REF":      {},
}

type country struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type providerReferencePayload struct {
	FirstName     *string
	LastName      *string
	Email         *string
	Phone         string
	ReferenceType string
}

type providerReferenceForwardOptions struct {
	includeReferenceType bool
	includeEmptyPhone    bool
}

func (h *Handler) handleProviders(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	values := r.URL.Query()
	values.Set("page_number", "1")
	values.Set("disable_pagination", "true")
	values.Set("usable", "true")
	h.forwardArak(w, r, http.MethodGet, arakProviderRoot+"/provider", values.Encode(), nil, nil)
}

func (h *Handler) handleProviderDetail(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	h.forwardArak(w, r, http.MethodGet, arakProviderRoot+"/provider/"+url.PathEscape(r.PathValue("id")), r.URL.RawQuery, nil, nil)
}

func (h *Handler) handleProviderCountries(w http.ResponseWriter, r *http.Request) {
	if !h.requireArakDB(w) {
		return
	}
	rows, err := h.arakDB.QueryContext(r.Context(), `
		SELECT code, name
		FROM provider_qualifications.country
		ORDER BY name ASC`)
	if err != nil {
		httputil.InternalError(w, r, err, "rda provider countries query failed")
		return
	}
	defer rows.Close()

	items := make([]country, 0)
	for rows.Next() {
		var item country
		if err := rows.Scan(&item.Code, &item.Name); err != nil {
			httputil.InternalError(w, r, err, "rda provider countries scan failed")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		httputil.InternalError(w, r, err, "rda provider countries rows failed")
		return
	}
	httputil.JSON(w, http.StatusOK, items)
}

func (h *Handler) handleProviderDraft(w http.ResponseWriter, r *http.Request) {
	if !h.requireArak(w) {
		return
	}
	h.forwardArak(w, r, http.MethodPost, arakProviderRoot+"/provider/draft", r.URL.RawQuery, r.Body, nil)
}

func (h *Handler) handleCreateProviderReference(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeProviderReferencePayload(w, r)
	if !ok {
		return
	}
	if payload.ReferenceType == "" {
		httputil.Error(w, http.StatusBadRequest, "Seleziona il tipo contatto")
		return
	}
	if payload.ReferenceType == providerQualificationReferenceType {
		httputil.Error(w, http.StatusBadRequest, "QUALIFICATION_REF non puo' essere gestito da /references")
		return
	}
	if !isAllowedRDAProviderReferenceType(payload.ReferenceType) {
		httputil.Error(w, http.StatusBadRequest, "Tipo contatto non valido")
		return
	}
	if payload.Email == nil {
		httputil.Error(w, http.StatusBadRequest, "Inserisci l'email del contatto")
		return
	}
	h.forwardProviderReference(
		w,
		r,
		"/provider/"+url.PathEscape(r.PathValue("id"))+"/reference",
		payload,
		providerReferenceForwardOptions{includeReferenceType: true},
	)
}

func (h *Handler) handleUpdateProviderReference(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeProviderReferencePayload(w, r)
	if !ok {
		return
	}
	if payload.ReferenceType == providerQualificationReferenceType {
		httputil.Error(w, http.StatusBadRequest, "QUALIFICATION_REF non puo' essere gestito da /references")
		return
	}
	if payload.ReferenceType != "" && !isAllowedRDAProviderReferenceType(payload.ReferenceType) {
		httputil.Error(w, http.StatusBadRequest, "Tipo contatto non valido")
		return
	}
	h.forwardProviderReference(
		w,
		r,
		"/provider/"+url.PathEscape(r.PathValue("id"))+"/reference/"+url.PathEscape(r.PathValue("refId")),
		payload,
		providerReferenceForwardOptions{includeEmptyPhone: true},
	)
}

func decodeProviderReferencePayload(w http.ResponseWriter, r *http.Request) (providerReferencePayload, bool) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return providerReferencePayload{}, false
	}
	var payload providerReferencePayload
	for _, field := range []struct {
		key string
		dst **string
	}{
		{key: "first_name", dst: &payload.FirstName},
		{key: "last_name", dst: &payload.LastName},
		{key: "email", dst: &payload.Email},
	} {
		value, ok := referencePayloadString(raw, field.key)
		if !ok {
			httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
			return providerReferencePayload{}, false
		}
		if value != "" {
			copied := value
			*field.dst = &copied
		}
	}
	phone, ok := referencePayloadString(raw, "phone")
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return providerReferencePayload{}, false
	}
	if !isValidRDAProviderReferencePhone(phone) {
		httputil.Error(w, http.StatusBadRequest, "Telefono contatto non valido")
		return providerReferencePayload{}, false
	}
	payload.Phone = phone
	refType, ok := referencePayloadString(raw, "reference_type")
	if !ok {
		httputil.Error(w, http.StatusBadRequest, "Richiesta non valida")
		return providerReferencePayload{}, false
	}
	payload.ReferenceType = strings.ToUpper(refType)
	return payload, true
}

func referencePayloadString(raw map[string]any, key string) (string, bool) {
	value, exists := raw[key]
	if !exists || value == nil {
		return "", true
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(text), true
}

func isAllowedRDAProviderReferenceType(value string) bool {
	_, ok := allowedRDAProviderReferenceTypes[value]
	return ok
}

func isValidRDAProviderReferencePhone(value string) bool {
	return value == "" || providerReferencePhonePattern.MatchString(value)
}

func (p providerReferencePayload) arakBody(opts providerReferenceForwardOptions) ([]byte, error) {
	body := map[string]any{}
	if p.Phone != "" || opts.includeEmptyPhone {
		body["phone"] = p.Phone
	}
	if p.FirstName != nil {
		body["first_name"] = *p.FirstName
	}
	if p.LastName != nil {
		body["last_name"] = *p.LastName
	}
	if p.Email != nil {
		body["email"] = *p.Email
	}
	if opts.includeReferenceType && p.ReferenceType != "" {
		body["reference_type"] = p.ReferenceType
	}
	return json.Marshal(body)
}

func (h *Handler) forwardProviderReference(w http.ResponseWriter, r *http.Request, path string, payload providerReferencePayload, opts providerReferenceForwardOptions) {
	if !h.requireArak(w) {
		return
	}
	body, err := payload.arakBody(opts)
	if err != nil {
		httputil.InternalError(w, r, err, "rda provider reference body encode failed")
		return
	}
	h.forwardArak(w, r, r.Method, arakProviderRoot+path, r.URL.RawQuery, bytes.NewReader(body), nil)
}
