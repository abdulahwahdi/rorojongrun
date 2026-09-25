package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golangid/candi/candishared"
)

// PermissionRequest asks whether a principal may use a permission code of a service
type PermissionRequest struct {
	Realm     string
	UserID    string
	SessionID int
	Service   string
	Code      string
}

// PermissionClient is implemented by the user service client
type PermissionClient interface {
	CheckPermission(ctx context.Context, req PermissionRequest) (allowed bool, role string, err error)
}

type aclEntry struct {
	allowed bool
	role    string
	expires time.Time
}

// ACLChecker implements candi's interfaces.ACLPermissionChecker for a service:
// every code checked by the service's handlers is qualified with the service name
// and resolved by the user service, with a short in-memory cache.
type ACLChecker struct {
	service string
	client  PermissionClient
	ttl     time.Duration

	mu    sync.Mutex
	cache map[string]aclEntry
}

// NewACLChecker constructor. ttl <= 0 disables caching.
func NewACLChecker(service string, client PermissionClient, ttl time.Duration) *ACLChecker {
	return &ACLChecker{service: service, client: client, ttl: ttl, cache: map[string]aclEntry{}}
}

// CheckPermission implement interfaces.ACLPermissionChecker
func (c *ACLChecker) CheckPermission(ctx context.Context, userID string, permissionCode string) (role string, err error) {
	claim, _ := ctx.Value(candishared.ContextKeyTokenClaim).(*candishared.TokenClaim)
	if claim == nil {
		return "", errors.New("Forbidden: missing token claim")
	}
	req := PermissionRequest{
		Realm: RealmFromClaim(claim), UserID: userID, SessionID: SessionIDFromClaim(claim),
		Service: c.service, Code: permissionCode,
	}
	key := fmt.Sprintf("%s|%s|%d|%s", req.Realm, req.UserID, req.SessionID, req.Code)

	if c.ttl > 0 {
		c.mu.Lock()
		e, ok := c.cache[key]
		c.mu.Unlock()
		if ok && time.Now().Before(e.expires) {
			return c.result(e.allowed, e.role, permissionCode)
		}
	}

	allowed, role, err := c.client.CheckPermission(ctx, req)
	if err != nil {
		return "", fmt.Errorf("Forbidden: permission check failed: %w", err)
	}
	if c.ttl > 0 {
		c.mu.Lock()
		if len(c.cache) > 10000 { // bound memory
			c.cache = map[string]aclEntry{}
		}
		c.cache[key] = aclEntry{allowed: allowed, role: role, expires: time.Now().Add(c.ttl)}
		c.mu.Unlock()
	}
	return c.result(allowed, role, permissionCode)
}

func (c *ACLChecker) result(allowed bool, role, code string) (string, error) {
	if !allowed {
		return "", fmt.Errorf("Forbidden: missing permission %q", code)
	}
	return role, nil
}
