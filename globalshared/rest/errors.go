// Package rest holds the HTTP helpers every REST service of the monorepo shares: business errors
// mapped to status codes, JSON-schema validated body and query decoding, and response writers.
package rest

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
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

// MapDBError turns a postgres unique violation into a 409 AppError, other errors pass through.
// The SQL layer runs on lib/pq (candi's postgres driver); pgx errors are recognised too.
func MapDBError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return NewConflict("duplicate value: " + pqErr.Constraint)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return NewConflict("duplicate value: " + pgErr.ConstraintName)
	}
	return err
}

// NewBadGateway an upstream (payment gateway) failure
func NewBadGateway(msg string) error { return &AppError{Status: http.StatusBadGateway, Message: msg} }

// NewUnavailable a feature that is currently switched off / not usable
func NewUnavailable(msg string) error {
	return &AppError{Status: http.StatusServiceUnavailable, Message: msg}
}
