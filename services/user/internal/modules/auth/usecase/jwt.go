package usecase

import (
	"context"
	"crypto/rsa"
	"strconv"
	"strings"
	"time"

	"monorepo/globalshared/auth"
	"monorepo/services/user/pkg/helper"
	"monorepo/services/user/pkg/shared"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
)

type cachedKey struct {
	kid  string
	priv *rsa.PrivateKey
}

// signingKey returns the newest active key of the realm, decrypting (and caching) its private half
func (uc *authUsecaseImpl) signingKey(ctx context.Context, realmID int) (*cachedKey, error) {
	key, err := uc.repoSQL.RealmKeyRepo().FindActive(ctx, realmID)
	if err != nil {
		return nil, helper.NewInvalid("realm has no active signing key")
	}
	uc.keyMu.RLock()
	c, ok := uc.keyCache[key.KID]
	uc.keyMu.RUnlock()
	if ok {
		return c, nil
	}
	pem, err := helper.Decrypt(shared.GetEnv().KeyEncryptionSecret, key.PrivateKeyEnc)
	if err != nil {
		return nil, err
	}
	priv, err := helper.PrivateKeyFromPEM(pem)
	if err != nil {
		return nil, err
	}
	c = &cachedKey{kid: key.KID, priv: priv}
	uc.keyMu.Lock()
	uc.keyCache[key.KID] = c
	uc.keyMu.Unlock()
	return c, nil
}

// signAccessToken issues an RS256 JWT for the user. sid is the refresh-token family (0 for service tokens).
func (uc *authUsecaseImpl) signAccessToken(ctx context.Context, realm shareddomain.Realm, clientID string, user shareddomain.User, sid int) (token string, expiresIn int, err error) {
	key, err := uc.signingKey(ctx, realm.ID)
	if err != nil {
		return "", 0, err
	}
	roles, err := uc.repoSQL.UserRepo().Roles(ctx, user.ID)
	if err != nil {
		return "", 0, err
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	typ := auth.TypeUser
	if user.IsServiceAccount {
		typ = auth.TypeService
	}

	now := time.Now()
	claim := candishared.TokenClaim{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    auth.IssuerFor(shared.GetEnv().IssuerBaseURL, realm.Name),
			Subject:   strconv.Itoa(user.ID),
			Audience:  jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(realm.AccessTokenTTLSec) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        helper.RandomToken(12),
		},
		Role: strings.Join(names, ","),
		Additional: map[string]any{
			auth.ClaimRealm: realm.Name, auth.ClaimSessionID: sid, auth.ClaimClientID: clientID, auth.ClaimType: typ,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &claim)
	tok.Header["kid"] = key.kid
	token, err = tok.SignedString(key.priv)
	return token, realm.AccessTokenTTLSec, err
}
