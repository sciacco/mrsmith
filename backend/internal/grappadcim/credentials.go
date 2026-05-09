package grappadcim

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

func (h *Handler) handleGetServerCredentials(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_server_id")
		return
	}
	var credentials ServerCredentials
	var iloAddress, iloUsername, customerRootAccess, customerUsername, cdlanUsername sql.NullString
	var iloPassword, rootPassword, customerPassword, cdlanPassword sql.NullString
	err = h.grappa.QueryRowContext(r.Context(), `
		SELECT id_server, ilo_idrac, user_ilo, accesso_root_administrator_cliente, utenza_cliente,
		       utenza_cdlan, pwd_ilo, root_administrator_password, pwd_utenza_cliente, pwd_utenza_cdlan
		FROM server
		WHERE id_server = ?`, id).Scan(
		&credentials.ServerID, &iloAddress, &iloUsername, &customerRootAccess, &customerUsername,
		&cdlanUsername, &iloPassword, &rootPassword, &customerPassword, &cdlanPassword,
	)
	if err == sql.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "server_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_server_credentials", err, "server_id", id)
		return
	}
	credentials.IloAddress = nullableString(iloAddress)
	credentials.IloUsername = nullableString(iloUsername)
	credentials.CustomerRootAccess = nullableString(customerRootAccess)
	credentials.CustomerUsername = nullableString(customerUsername)
	credentials.CdlanUsername = nullableString(cdlanUsername)
	credentials.IloPasswordStored = strings.TrimSpace(iloPassword.String) != ""
	credentials.RootAdministratorStored = strings.TrimSpace(rootPassword.String) != ""
	credentials.CustomerPasswordStored = strings.TrimSpace(customerPassword.String) != ""
	credentials.CdlanPasswordStored = strings.TrimSpace(cdlanPassword.String) != ""
	credentials.PasswordValueAccessEnabled = false
	credentials.PasswordWriteAccessEnabled = false
	httputil.JSON(w, http.StatusOK, credentials)
}

func (h *Handler) handleUpdateServerCredentials(w http.ResponseWriter, r *http.Request) {
	if !h.requireDB(w) {
		return
	}
	id, err := parsePathInt(r, "id")
	if err != nil {
		invalidRequest(w, "invalid_server_id")
		return
	}
	var body ServerCredentialsPatch
	if err := decodeJSONBody(r, &body); err != nil {
		invalidRequest(w, "invalid_credentials_payload")
		return
	}
	sets := []string{}
	args := []any{}
	fields := []struct {
		column string
		value  *string
	}{
		{"ilo_idrac", body.IloAddress},
		{"user_ilo", body.IloUsername},
		{"accesso_root_administrator_cliente", body.CustomerRootAccess},
		{"utenza_cliente", body.CustomerUsername},
		{"utenza_cdlan", body.CdlanUsername},
	}
	for _, field := range fields {
		if field.value != nil {
			sets = append(sets, field.column+" = ?")
			args = append(args, optionalTrimmed(field.value))
		}
	}
	if len(sets) == 0 {
		httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Nessuna modifica."})
		return
	}
	args = append(args, id)
	result, err := h.grappa.ExecContext(r.Context(), `UPDATE server SET `+strings.Join(sets, ", ")+` WHERE id_server = ?`, args...)
	if err != nil {
		h.dbFailure(w, r, "update_server_credentials", err, "server_id", id)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httputil.Error(w, http.StatusNotFound, "server_not_found")
		return
	}
	httputil.JSON(w, http.StatusOK, MutationResponse{ID: id, Message: "Credenziali aggiornate."})
}
