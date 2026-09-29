// Package auth holds the realm-aware token validation and permission checking
// shared by every service. The "user" service issues the tokens; all other
// services only verify them (via JWKS) and ask "user" whether a permission is
// granted.
package auth

import (
	"strings"

	"github.com/golangid/candi/candishared"
)

// Keys inside candishared.TokenClaim.Additional
const (
	ClaimRealm     = "realm"
	ClaimSessionID = "sid"
	ClaimClientID  = "cid"
	ClaimType      = "typ"

	TypeUser    = "user"
	TypeService = "service"
)

// IssuerFor returns the "iss" value of tokens issued for a realm
func IssuerFor(issuerBaseURL, realm string) string {
	return strings.TrimRight(issuerBaseURL, "/") + "/realms/" + realm
}

// RealmFromIssuer extracts the realm name from an issuer produced by IssuerFor
func RealmFromIssuer(issuerBaseURL, issuer string) (realm string, ok bool) {
	prefix := strings.TrimRight(issuerBaseURL, "/") + "/realms/"
	if !strings.HasPrefix(issuer, prefix) {
		return "", false
	}
	realm = strings.TrimPrefix(issuer, prefix)
	return realm, realm != "" && !strings.Contains(realm, "/")
}

func additional(c *candishared.TokenClaim) map[string]any {
	if c == nil {
		return nil
	}
	m, _ := c.Additional.(map[string]any)
	return m
}

// RealmFromClaim returns the realm the token was issued for
func RealmFromClaim(c *candishared.TokenClaim) string {
	s, _ := additional(c)[ClaimRealm].(string)
	return s
}

// ClientIDFromClaim returns the client_id (audience client) of the token
func ClientIDFromClaim(c *candishared.TokenClaim) string {
	s, _ := additional(c)[ClaimClientID].(string)
	return s
}

// TypeFromClaim returns TypeUser or TypeService
func TypeFromClaim(c *candishared.TokenClaim) string {
	s, _ := additional(c)[ClaimType].(string)
	return s
}

// SessionIDFromClaim returns the session (refresh-token family) id, 0 if the token has none
func SessionIDFromClaim(c *candishared.TokenClaim) int {
	switch v := additional(c)[ClaimSessionID].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// TokenClaimFromContext returns the claim set by candi's bearer middleware, nil if none
func TokenClaimFromContext(ctx interface{ Value(any) any }) *candishared.TokenClaim {
	c, _ := ctx.Value(candishared.ContextKeyTokenClaim).(*candishared.TokenClaim)
	return c
}
