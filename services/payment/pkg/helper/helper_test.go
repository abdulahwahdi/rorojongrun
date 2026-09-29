package helper

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"

	"github.com/stretchr/testify/assert"
)

func Test_FormatIDR(t *testing.T) {
	cases := map[int64]string{0: "Rp0", 999: "Rp999", 1000: "Rp1.000", 101000: "Rp101.000", 1250000: "Rp1.250.000", 1000000000: "Rp1.000.000.000", -5000: "-Rp5.000"}
	for in, want := range cases {
		assert.Equal(t, want, FormatIDR(in))
	}
}

func Test_Errors(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, HTTPStatus(NewNotFound("x")))
	assert.Equal(t, http.StatusBadGateway, HTTPStatus(NewBadGateway("x")))
	assert.Equal(t, http.StatusServiceUnavailable, HTTPStatus(NewUnavailable("x")))
	assert.Equal(t, http.StatusInternalServerError, HTTPStatus(errors.New("boom")))
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
