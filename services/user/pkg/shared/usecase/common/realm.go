package common

import (
	"context"
	"errors"

	"monorepo/globalshared/auth"
	"monorepo/globalshared/rest"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
	"gorm.io/gorm"
)

// MasterRealm is the realm that administers all other realms
const MasterRealm = "master"

// AdminService is the service name under which this service's own permissions are registered
const AdminService = "user"

// SystemContext marks ctx as an internal call (bootstrap, migrations) acting with master authority
func SystemContext(ctx context.Context) context.Context {
	claim := &candishared.TokenClaim{RegisteredClaims: jwt.RegisteredClaims{Subject: "system"}}
	claim.Additional = map[string]any{auth.ClaimRealm: MasterRealm, auth.ClaimType: auth.TypeService}
	return candishared.SetToContext(ctx, candishared.ContextKeyTokenClaim, claim)
}

// CallerRealm returns the realm of the authenticated caller, "" when unauthenticated
func CallerRealm(ctx context.Context) string {
	return auth.RealmFromClaim(auth.TokenClaimFromContext(ctx))
}

// RequireMaster only lets callers authenticated in the master realm through
func RequireMaster(ctx context.Context) error {
	if CallerRealm(ctx) != MasterRealm {
		return rest.NewForbidden("Forbidden: only the master realm can manage realms")
	}
	return nil
}

// AuthorizeRealm lets callers of the same realm, or of the master realm, manage a realm.
// Fine-grained permissions were already checked by the ACL middleware in the caller's own realm.
func AuthorizeRealm(ctx context.Context, realm string) error {
	caller := CallerRealm(ctx)
	if caller == "" || (caller != realm && caller != MasterRealm) {
		return rest.NewForbidden("Forbidden: token of this realm cannot manage realm " + realm)
	}
	return nil
}

// ResolveRealm authorizes the caller for the realm and loads it
func ResolveRealm(ctx context.Context, repo repository.RepoSQL, name string) (shareddomain.Realm, error) {
	if err := AuthorizeRealm(ctx, name); err != nil {
		return shareddomain.Realm{}, err
	}
	return LoadRealm(ctx, repo, name)
}

// LoadRealm loads a realm without authorizing the caller (public / login flows)
func LoadRealm(ctx context.Context, repo repository.RepoSQL, name string) (shareddomain.Realm, error) {
	realm, err := repo.RealmRepo().FindByName(ctx, name)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return realm, rest.NewNotFound("realm " + name + " not found")
	}
	return realm, err
}

// NotFound converts gorm's not found into a 404 AppError for entity
func NotFound(err error, entity string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return rest.NewNotFound(entity + " not found")
	}
	return err
}
