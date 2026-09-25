package usecase

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
	"gorm.io/gorm"
)

var errNF = gorm.ErrRecordNotFound

func claimFor(realm string) context.Context {
	claim := &candishared.TokenClaim{RegisteredClaims: jwt.RegisteredClaims{Subject: "1"}}
	claim.Additional = map[string]any{"realm": realm}
	return candishared.SetToContext(context.Background(), candishared.ContextKeyTokenClaim, claim)
}
