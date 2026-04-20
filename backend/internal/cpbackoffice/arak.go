package cpbackoffice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

// Upstream paths on the Mistra NG gateway. Anchored in docs/mistra-dist.yaml
// (§ /customers/v2/*, § /users/v2/*). Kept in one place so changes are
// localized and obvious.
const (
	upstreamCustomersPath      = "/customers/v2/customer"
	upstreamCustomerStatesPath = "/customers/v2/customer-state"
	upstreamUsersPath          = "/users/v2/user"
	upstreamCreateAdminPath    = "/users/v2/admin"
)

// disablePaginationQuery is pinned on every list upstream call so the
// caller always gets the full set (v1 locks out frontend pagination).
const disablePaginationQuery = "disable_pagination=true"

// upstreamItemsEnvelope is the shape returned by every Mistra NG list
// endpoint. We unwrap `items` and surface only that array to the SPA;
// the pagination envelope is a v1 locked no-op.
type upstreamItemsEnvelope struct {
	Items json.RawMessage `json:"items"`
}

// upstreamErrorBody preserves the business `message` that the SPA surfaces
// to the operator on a toast. Shape verified in S0 pre-code checks.
type upstreamErrorBody struct {
	Message string `json:"message"`
}

// decodeUpstreamMessage returns the business-facing message from an upstream
// error body, or an empty string if the body is not JSON with a `message`
// field. Safe on empty / garbage bodies.
func decodeUpstreamMessage(body []byte) string {
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}
	var e upstreamErrorBody
	if err := json.Unmarshal(body, &e); err != nil {
		return ""
	}
	return e.Message
}

// forwardUpstreamError writes the upstream status code back to the client
// together with a JSON body `{"error": "upstream_error", "message": "<msg>"}`.
// The SPA formats the toast from `status + message`. The real upstream body
// and any decoding failure stay in the server logs.
func forwardUpstreamError(w http.ResponseWriter, r *http.Request, op string, status int, body []byte) {
	msg := decodeUpstreamMessage(body)
	logging.FromContext(r.Context()).Warn("upstream returned business error",
		"component", "cpbackoffice",
		"operation", op,
		"upstream_status", status,
		"upstream_message", msg,
		"upstream_body", compactBody(body),
	)
	httputil.JSON(w, status, map[string]string{
		"error":   "upstream_error",
		"message": msg,
	})
}

// compactBody returns a short snippet of the upstream body suitable for
// structured logs; caps length to keep log lines small.
func compactBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 256 {
		return text[:256] + "..."
	}
	return text
}

// ═══ GET /cp-backoffice/v1/customers ═══
// Upstream: GET /customers/v2/customer?disable_pagination=true.

func handleListCustomers(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireArak(deps) {
			writeUpstreamUnavailable(w)
			return
		}
		const op = "list_customers"

		resp, err := deps.Arak.Do(http.MethodGet, upstreamCustomersPath, disablePaginationQuery, nil)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}

		if resp.StatusCode >= http.StatusBadRequest {
			forwardUpstreamError(w, r, op, resp.StatusCode, body)
			return
		}

		writeItemsPassthrough(w, r, op, body)
	}
}

// ═══ GET /cp-backoffice/v1/customer-states ═══
// Upstream: GET /customers/v2/customer-state?disable_pagination=true.

func handleListCustomerStates(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireArak(deps) {
			writeUpstreamUnavailable(w)
			return
		}
		const op = "list_customer_states"

		resp, err := deps.Arak.Do(http.MethodGet, upstreamCustomerStatesPath, disablePaginationQuery, nil)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}

		if resp.StatusCode >= http.StatusBadRequest {
			forwardUpstreamError(w, r, op, resp.StatusCode, body)
			return
		}

		writeItemsPassthrough(w, r, op, body)
	}
}

// writeItemsPassthrough unwraps the `items` array from the upstream envelope
// and forwards it to the SPA. Upstream wrapping is a gateway convention; the
// SPA never receives pagination fields because v1 locks them to disabled.
func writeItemsPassthrough(w http.ResponseWriter, r *http.Request, op string, body []byte) {
	var env upstreamItemsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		upstreamFailure(w, r, fmt.Errorf("decode upstream envelope: %w", err), op)
		return
	}
	items := env.Items
	if len(items) == 0 {
		items = json.RawMessage("[]")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(items)
}

// ═══ PUT /cp-backoffice/v1/customers/{id}/state ═══
// Upstream: PUT /customers/v2/customer/{customerId} with body { state_id: int64 }.

func handleUpdateCustomerState(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireArak(deps) {
			writeUpstreamUnavailable(w)
			return
		}
		const op = "update_customer_state"

		rawID := strings.TrimSpace(r.PathValue("id"))
		if rawID == "" {
			httputil.Error(w, http.StatusBadRequest, "customer_id_required")
			return
		}
		customerID, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || customerID <= 0 {
			httputil.Error(w, http.StatusBadRequest, "customer_id_invalid")
			return
		}

		var in UpdateStateRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
			return
		}
		if in.StateID <= 0 {
			httputil.Error(w, http.StatusBadRequest, "state_id_required")
			return
		}

		// Upstream edit DTO is `customer-edit` (group_id, state_id, variables).
		// We only set state_id; other fields are optional and omitted.
		payload, err := json.Marshal(map[string]int64{"state_id": in.StateID})
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}

		path := upstreamCustomersPath + "/" + strconv.FormatInt(customerID, 10)
		resp, err := deps.Arak.Do(http.MethodPut, path, "", bytes.NewReader(payload))
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}

		if resp.StatusCode >= http.StatusBadRequest {
			forwardUpstreamError(w, r, op, resp.StatusCode, body)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if len(bytes.TrimSpace(body)) == 0 {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		_, _ = w.Write(body)
	}
}

// ═══ GET /cp-backoffice/v1/users?customer_id=... ═══
// Upstream: GET /users/v2/user?customer_id={id}&disable_pagination=true.
// Hard backend guard: missing or empty customer_id rejected with 400 and never
// proxied. The FE also gates the fetch on selection, but the backend is the
// source of truth for this rule.

func handleListUsers(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireArak(deps) {
			writeUpstreamUnavailable(w)
			return
		}
		const op = "list_users"

		customerID := strings.TrimSpace(r.URL.Query().Get("customer_id"))
		if customerID == "" {
			httputil.Error(w, http.StatusBadRequest, "customer_id_required")
			return
		}
		// Normalizing via url.Values guarantees correct encoding and a stable
		// param order (customer_id before disable_pagination).
		q := url.Values{}
		q.Set("customer_id", customerID)
		q.Set("disable_pagination", "true")

		resp, err := deps.Arak.Do(http.MethodGet, upstreamUsersPath, q.Encode(), nil)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}

		if resp.StatusCode >= http.StatusBadRequest {
			forwardUpstreamError(w, r, op, resp.StatusCode, body)
			return
		}

		writeItemsPassthrough(w, r, op, body)
	}
}

// ═══ POST /cp-backoffice/v1/admins ═══
// Upstream: POST /users/v2/admin with the user-admin-new DTO. The request
// body assembled for upstream ALWAYS pins `skip_keycloak: false` regardless
// of any caller-provided value. The hidden source switch is intentionally
// not exposed in v1; re-enablement tracked in docs/TODO.md (§S6).

// adminUpstreamPayload is the exact wire shape accepted by upstream
// POST /users/v2/admin. We marshal it explicitly (rather than via a pass-
// through map) so `skip_keycloak` is pinned at the type level and the FE
// cannot accidentally flip it by sending extra JSON fields.
type adminUpstreamPayload struct {
	FirstName                 string `json:"first_name"`
	LastName                  string `json:"last_name"`
	Email                     string `json:"email"`
	CustomerID                int64  `json:"customer_id"`
	Phone                     string `json:"phone"`
	MaintenanceOnPrimaryEmail bool   `json:"maintenance_on_primary_email"`
	MarketingOnPrimaryEmail   bool   `json:"marketing_on_primary_email"`
	SkipKeycloak              bool   `json:"skip_keycloak"`
}

// adminInboundPayload mirrors CreateAdminRequest but tolerates an inbound
// `skip_keycloak` so we can explicitly discard it (the type system then
// guarantees we never forward the caller's value).
type adminInboundPayload struct {
	CustomerID                int64  `json:"customer_id"`
	Nome                      string `json:"nome"`
	Cognome                   string `json:"cognome"`
	Email                     string `json:"email"`
	Telefono                  string `json:"telefono"`
	MaintenanceOnPrimaryEmail bool   `json:"maintenance_on_primary_email"`
	MarketingOnPrimaryEmail   bool   `json:"marketing_on_primary_email"`
	// SkipKeycloak is read and discarded; the outgoing request always pins it
	// to false. The field is here only so a deliberate `true` from a client
	// does not slip through via a generic map decode.
	SkipKeycloak *bool `json:"skip_keycloak,omitempty"`
}

func handleCreateAdmin(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireArak(deps) {
			writeUpstreamUnavailable(w)
			return
		}
		const op = "create_admin"

		var in adminInboundPayload
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_request_body")
			return
		}
		if in.CustomerID <= 0 {
			httputil.Error(w, http.StatusBadRequest, "customer_id_required")
			return
		}
		if strings.TrimSpace(in.Email) == "" {
			httputil.Error(w, http.StatusBadRequest, "email_required")
			return
		}

		// Construct the upstream payload from scratch. skip_keycloak is pinned
		// to false — NEVER read from the caller. See v1 lock in FINAL.md §S3.
		out := adminUpstreamPayload{
			FirstName:                 in.Nome,
			LastName:                  in.Cognome,
			Email:                     in.Email,
			CustomerID:                in.CustomerID,
			Phone:                     in.Telefono,
			MaintenanceOnPrimaryEmail: in.MaintenanceOnPrimaryEmail,
			MarketingOnPrimaryEmail:   in.MarketingOnPrimaryEmail,
			SkipKeycloak:              false,
		}
		payload, err := json.Marshal(out)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}

		resp, err := deps.Arak.Do(http.MethodPost, upstreamCreateAdminPath, "", bytes.NewReader(payload))
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			upstreamFailure(w, r, err, op)
			return
		}

		if resp.StatusCode >= http.StatusBadRequest {
			forwardUpstreamError(w, r, op, resp.StatusCode, body)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		if len(bytes.TrimSpace(body)) == 0 {
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		_, _ = w.Write(body)
	}
}
