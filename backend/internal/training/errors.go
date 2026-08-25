package training

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

type appError struct {
	status  int
	code    string
	message string
}

func (e appError) Error() string {
	if e.message != "" {
		return e.message
	}
	return e.code
}

func validationError(code, message string) error {
	return appError{status: http.StatusUnprocessableEntity, code: code, message: message}
}

func conflictError(code, message string) error {
	return appError{status: http.StatusConflict, code: code, message: message}
}

func forbiddenError(code, message string) error {
	return appError{status: http.StatusForbidden, code: code, message: message}
}

func notFoundError(code, message string) error {
	return appError{status: http.StatusNotFound, code: code, message: message}
}

func serviceUnavailableError(code, message string) error {
	return appError{status: http.StatusServiceUnavailable, code: code, message: message}
}

func asAppError(err error) (appError, bool) {
	var appErr appError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return appError{}, false
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && (constraint == "" || pgErr.ConstraintName == constraint)
}
