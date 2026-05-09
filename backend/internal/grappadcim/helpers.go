package grappadcim

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const maxJSONBodyBytes = 2 << 20

var errBadRequest = errors.New("bad request")

func decodeJSONBody(r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

func parsePathInt(r *http.Request, name string) (int, error) {
	value, err := strconv.Atoi(r.PathValue(name))
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

func parsePositiveString(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, errBadRequest
	}
	return value, nil
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(value.String)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullableInt(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func nullableFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func nullableTime(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format(time.RFC3339)
	return &formatted
}

func nullableDate(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.Format("2006-01-02")
	return &formatted
}

func requiredTrimmed(value string) string {
	return strings.TrimSpace(value)
}

func optionalTrimmed(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func hasDestructiveConfirmation(body DestructiveActionRequest) bool {
	return body.ConfirmPrimary && body.ConfirmSecondary
}

func decodeDestructiveBody(r *http.Request) (DestructiveActionRequest, error) {
	var body DestructiveActionRequest
	if err := decodeJSONBody(r, &body); err != nil {
		return body, err
	}
	if !hasDestructiveConfirmation(body) {
		return body, errBadRequest
	}
	return body, nil
}

func activeStateSQL(column string) string {
	return "(COALESCE(TRIM(" + column + "), '') = '' OR LOWER(TRIM(" + column + ")) NOT IN ('cessato', 'cessata', 'spento', 'chiuso'))"
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func withTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func invalidRequest(w http.ResponseWriter, code string) {
	httputil.Error(w, http.StatusBadRequest, code)
}
