package rest

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func Test_Errors(t *testing.T) {
	cases := map[int]error{404: NewNotFound("x"), 409: NewConflict("x"), 400: NewInvalid("x"), 403: NewForbidden("x"),
		401: NewUnauthorized("x"), 423: NewLocked("x"), 502: NewBadGateway("x"), 503: NewUnavailable("x")}
	for status, err := range cases {
		assert.Equal(t, status, HTTPStatus(err))
		assert.Equal(t, status, HTTPStatus(fmt.Errorf("wrapped: %w", err)), "survives wrapping")
	}
	assert.Equal(t, http.StatusInternalServerError, HTTPStatus(errors.New("boom")))
	assert.Equal(t, "x", NewInvalid("x").Error())
}

func Test_MapDBError(t *testing.T) {
	// candi's postgres driver is lib/pq: a unique violation must become a 409, not leak as a 500
	err := MapDBError(&pq.Error{Code: "23505", Constraint: "idx_payment_methods_code"})
	assert.Equal(t, http.StatusConflict, HTTPStatus(err))
	assert.Contains(t, err.Error(), "idx_payment_methods_code")
	assert.NotContains(t, err.Error(), "pq:", "driver text is not shown to API clients")

	assert.Equal(t, http.StatusConflict, HTTPStatus(MapDBError(fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "23505", ConstraintName: "c"}))))

	other := &pq.Error{Code: "23503"}
	assert.Equal(t, other, MapDBError(other), "other errors pass through")
	assert.NoError(t, MapDBError(nil))
}
