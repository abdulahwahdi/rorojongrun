package helper

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
)

// AppError is a business error carrying the HTTP status it maps to
type AppError struct {
	Status  int
	Message string
}

func (e *AppError) Error() string { return e.Message }

// NewNotFound entity not found
func NewNotFound(msg string) error { return &AppError{Status: http.StatusNotFound, Message: msg} }

// NewConflict duplicate / state conflict
func NewConflict(msg string) error { return &AppError{Status: http.StatusConflict, Message: msg} }

// NewInvalid invalid input that passed schema validation
func NewInvalid(msg string) error { return &AppError{Status: http.StatusBadRequest, Message: msg} }

// NewForbidden authenticated but not allowed
func NewForbidden(msg string) error { return &AppError{Status: http.StatusForbidden, Message: msg} }

// NewUnauthorized bad credentials / token
func NewUnauthorized(msg string) error {
	return &AppError{Status: http.StatusUnauthorized, Message: msg}
}

// NewLocked account locked
func NewLocked(msg string) error { return &AppError{Status: http.StatusLocked, Message: msg} }

// HTTPStatus maps an error to an HTTP status, 500 for unknown errors
func HTTPStatus(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Status
	}
	return http.StatusInternalServerError
}

// MapDBError turns a postgres unique violation into a 409 AppError, other errors pass through
func MapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return NewConflict("duplicate value: " + pgErr.ConstraintName)
	}
	return err
}
