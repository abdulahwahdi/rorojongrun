// Package bootstrap seeds what a fresh installation needs to be administrable: the master realm with
// its signing key, a "superadmin" role holding the wildcard permission, the first superadmin user
// and the "admin-cli" client to log in with. Everything is idempotent and safe to run on every start.
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	clientdomain "monorepo/services/user/internal/modules/client/domain"
	realmdomain "monorepo/services/user/internal/modules/realm/domain"
	userdomain "monorepo/services/user/internal/modules/user/domain"
	"monorepo/services/user/pkg/shared"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/repository"
	"monorepo/services/user/pkg/shared/usecase"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/logger"
	"gorm.io/gorm"
)

const (
	// SuperadminRole is the role of the master realm that may do everything, in every realm
	SuperadminRole = "superadmin"
	// AdminClientID is the public client used to log in to the master realm
	AdminClientID = "admin-cli"
)

// Run seeds the database. It needs the migrations to have been applied.
func Run(ctx context.Context) error {
	env := shared.GetEnv()
	if env.KeyEncryptionSecret == "" || env.IssuerBaseURL == "" {
		return errors.New("KEY_ENCRYPTION_SECRET and ISSUER_BASE_URL must be set")
	}
	ctx = common.SystemContext(ctx)
	repo := repository.GetSharedRepoSQL()
	uc := usecase.GetSharedUsecase()

	// 1. master realm (creating a realm also creates its first key and its realm-admin role)
	if _, err := repo.RealmRepo().FindByName(ctx, common.MasterRealm); errors.Is(err, gorm.ErrRecordNotFound) {
		enabled := true
		name := "Master"
		if _, err := uc.Realm().CreateRealm(ctx, &realmdomain.RequestRealm{Name: common.MasterRealm, DisplayName: &name, Enabled: &enabled}); err != nil {
			return fmt.Errorf("create master realm: %w", err)
		}
		logger.LogI("bootstrap: created realm " + common.MasterRealm)
	} else if err != nil {
		return fmt.Errorf("look up master realm: %w", err)
	}
	master, err := repo.RealmRepo().FindByName(ctx, common.MasterRealm)
	if err != nil {
		return err
	}
	if keys, err := repo.RealmKeyRepo().FetchActive(ctx, master.ID); err != nil {
		return err
	} else if len(keys) == 0 { // every key was deactivated by hand: the realm could not sign anything
		if _, err := uc.Realm().RotateRealmKey(ctx, common.MasterRealm); err != nil {
			return fmt.Errorf("create master signing key: %w", err)
		}
	}

	// 2. superadmin role: the wildcard permission (any service, any code)
	role, err := ensureSuperadminRole(ctx, master, repo)
	if err != nil {
		return err
	}

	// 3. first superadmin user, only when the master realm has no user yet
	if err := ensureFirstAdmin(ctx, master, role, repo, uc); err != nil {
		return err
	}

	// 4. the public client to log in with
	if _, err := repo.ClientRepo().FindByClientID(ctx, master.ID, AdminClientID); errors.Is(err, gorm.ErrRecordNotFound) {
		if _, err := uc.Client().CreateClient(ctx, common.MasterRealm, &clientdomain.RequestCreateClient{
			ClientID: AdminClientID, Name: "Admin CLI", Description: "Built-in client to administer the master realm",
			Type: shareddomain.ClientTypePublic, GrantTypes: []string{"password", "refresh_token"},
		}); err != nil {
			return fmt.Errorf("create %s client: %w", AdminClientID, err)
		}
		logger.LogI("bootstrap: created client " + AdminClientID)
	} else if err != nil {
		return err
	}
	return nil
}

func ensureSuperadminRole(ctx context.Context, master shareddomain.Realm, repo repository.RepoSQL) (shareddomain.Role, error) {
	perm, err := repo.PermissionRepo().FindByServiceCode(ctx, master.ID, shareddomain.WildcardPermission, shareddomain.WildcardPermission)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		perm = shareddomain.Permission{
			RealmID: master.ID, Service: shareddomain.WildcardPermission, Code: shareddomain.WildcardPermission,
			Type: shareddomain.PermissionTypeAPI, Description: "Every permission of every service",
		}
		err = repo.PermissionRepo().Save(ctx, &perm)
	}
	if err != nil {
		return shareddomain.Role{}, fmt.Errorf("ensure wildcard permission: %w", err)
	}

	role, err := repo.RoleRepo().FindByName(ctx, master.ID, SuperadminRole)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		role = shareddomain.Role{RealmID: master.ID, Name: SuperadminRole, Description: "Full control of every realm and service"}
		err = repo.RoleRepo().Save(ctx, &role)
	}
	if err != nil {
		return role, fmt.Errorf("ensure %s role: %w", SuperadminRole, err)
	}
	// repair the grant if someone removed it; adding is a no-op when it is there
	if err := repo.RoleRepo().AddPermission(ctx, role.ID, perm.ID); err != nil {
		return role, fmt.Errorf("grant wildcard permission: %w", err)
	}
	return role, nil
}

func ensureFirstAdmin(ctx context.Context, master shareddomain.Realm, role shareddomain.Role, repo repository.RepoSQL, uc usecase.Usecase) error {
	env := shared.GetEnv()
	if repo.UserRepo().Count(ctx, master.ID, &userdomain.FilterUser{IncludeServiceAccounts: true}) > 0 {
		return nil
	}
	if env.BootstrapAdminUsername == "" || env.BootstrapAdminPassword == "" {
		logger.LogI("bootstrap: the master realm has no user; set BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD and restart to create the first superadmin")
		return nil
	}
	if _, err := uc.User().CreateUser(ctx, common.MasterRealm, &userdomain.RequestCreateUser{
		Username: env.BootstrapAdminUsername, Password: env.BootstrapAdminPassword, FullName: "Superadmin",
		Status: shareddomain.UserStatusActive, RoleIDs: []int{role.ID},
	}); err != nil {
		return fmt.Errorf("create first superadmin: %w", err)
	}
	logger.LogI("bootstrap: created superadmin user " + env.BootstrapAdminUsername + " in realm " + common.MasterRealm)
	return nil
}
