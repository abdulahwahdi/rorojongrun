package usecase

import (
	"context"
	"testing"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/rbac/domain"
	mockrbac "monorepo/services/user/pkg/mocks/modules/rbac/repository"
	mockrealm "monorepo/services/user/pkg/mocks/modules/realm/repository"
	mocksharedrepo "monorepo/services/user/pkg/mocks/shared/repository"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

var errNF = gorm.ErrRecordNotFound

func ctxOf(realm string) context.Context {
	claim := &candishared.TokenClaim{RegisteredClaims: jwt.RegisteredClaims{Subject: "1"}}
	claim.Additional = map[string]any{"realm": realm}
	return candishared.SetToContext(context.Background(), candishared.ContextKeyTokenClaim, claim)
}

type harness struct {
	uc    *rbacUsecaseImpl
	roles *mockrbac.RoleRepository
	perms *mockrbac.PermissionRepository
}

func newHarness(t *testing.T) *harness {
	repo := &mocksharedrepo.RepoSQL{}
	realms := &mockrealm.RealmRepository{}
	h := &harness{roles: &mockrbac.RoleRepository{}, perms: &mockrbac.PermissionRepository{}}
	repo.On("RealmRepo").Return(realms)
	repo.On("RoleRepo").Return(h.roles)
	repo.On("PermissionRepo").Return(h.perms)
	repo.On("WithTransaction", mock.Anything, mock.Anything).Return(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	realms.On("FindByName", mock.Anything, "acme").Return(shareddomain.Realm{ID: 1, Name: "acme", Enabled: true}, nil)
	h.uc = &rbacUsecaseImpl{repoSQL: repo}
	return h
}

func Test_Roles(t *testing.T) {
	ctx := ctxOf("acme")

	t.Run("create rejects a duplicate name inside the realm", func(t *testing.T) {
		h := newHarness(t)
		h.roles.On("FindByName", mock.Anything, 1, "ops").Return(shareddomain.Role{ID: 9}, nil)
		_, err := h.uc.CreateRole(ctx, "acme", &domain.RequestRole{Name: "ops"})
		assert.Equal(t, 409, rest.HTTPStatus(err))
	})

	t.Run("create", func(t *testing.T) {
		h := newHarness(t)
		h.roles.On("FindByName", mock.Anything, 1, "ops").Return(shareddomain.Role{}, errNF)
		h.roles.On("Save", mock.Anything, mock.MatchedBy(func(r *shareddomain.Role) bool { return r.RealmID == 1 && r.Name == "ops" })).Return(nil)
		res, err := h.uc.CreateRole(ctx, "acme", &domain.RequestRole{Name: "ops", Description: "d"})
		assert.NoError(t, err)
		assert.Equal(t, "ops", res.Name)
	})

	t.Run("rename onto an existing role is a conflict, keeping the own name is fine", func(t *testing.T) {
		h := newHarness(t)
		h.roles.On("Find", mock.Anything, 1, 5).Return(shareddomain.Role{ID: 5, Name: "ops"}, nil)
		h.roles.On("FindByName", mock.Anything, 1, "other").Return(shareddomain.Role{ID: 6}, nil)
		h.roles.On("FindByName", mock.Anything, 1, "ops").Return(shareddomain.Role{ID: 5}, nil)
		h.roles.On("Save", mock.Anything, mock.Anything).Return(nil)
		assert.Equal(t, 409, rest.HTTPStatus(h.uc.UpdateRole(ctx, "acme", 5, &domain.RequestRole{Name: "other"})))
		assert.NoError(t, h.uc.UpdateRole(ctx, "acme", 5, &domain.RequestRole{Name: "ops", Description: "new"}))
	})

	t.Run("delete is blocked while assigned unless forced", func(t *testing.T) {
		h := newHarness(t)
		h.roles.On("Find", mock.Anything, 1, 5).Return(shareddomain.Role{ID: 5}, nil)
		h.roles.On("CountUsers", mock.Anything, 5).Return(2)
		assert.Equal(t, 409, rest.HTTPStatus(h.uc.DeleteRole(ctx, "acme", 5, false)))
		h.roles.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)

		h.roles.On("Delete", mock.Anything, 5).Return(nil)
		assert.NoError(t, h.uc.DeleteRole(ctx, "acme", 5, true))
	})

	t.Run("delete unassigned", func(t *testing.T) {
		h := newHarness(t)
		h.roles.On("Find", mock.Anything, 1, 5).Return(shareddomain.Role{ID: 5}, nil)
		h.roles.On("CountUsers", mock.Anything, 5).Return(0)
		h.roles.On("Delete", mock.Anything, 5).Return(nil)
		assert.NoError(t, h.uc.DeleteRole(ctx, "acme", 5, false))
	})

	t.Run("detail includes permissions, missing role is 404", func(t *testing.T) {
		h := newHarness(t)
		h.roles.On("Find", mock.Anything, 1, 5).Return(shareddomain.Role{ID: 5, Name: "ops"}, nil)
		h.roles.On("Find", mock.Anything, 1, 6).Return(shareddomain.Role{}, errNF)
		h.roles.On("Permissions", mock.Anything, 5).Return([]shareddomain.Permission{{ID: 1, Service: "s", Code: "c"}}, nil)
		res, err := h.uc.GetDetailRole(ctx, "acme", 5)
		assert.NoError(t, err)
		assert.Len(t, res.Permissions, 1)
		_, err = h.uc.GetDetailRole(ctx, "acme", 6)
		assert.Equal(t, 404, rest.HTTPStatus(err))
	})
}

func Test_RolePermissions(t *testing.T) {
	ctx := ctxOf("acme")
	h := newHarness(t)
	h.roles.On("Find", mock.Anything, 1, 5).Return(shareddomain.Role{ID: 5}, nil)

	// replace: every id must exist in this realm, duplicates collapse
	h.perms.On("CountByIDs", mock.Anything, 1, []int{1, 2}).Return(2)
	h.roles.On("ReplacePermissions", mock.Anything, 5, []int{1, 2}).Return(nil)
	assert.NoError(t, h.uc.ReplaceRolePermissions(ctx, "acme", 5, []int{1, 2, 2}))
	h.perms.On("CountByIDs", mock.Anything, 1, []int{1, 999}).Return(1)
	assert.Equal(t, 400, rest.HTTPStatus(h.uc.ReplaceRolePermissions(ctx, "acme", 5, []int{1, 999})))

	// add: the permission must be of this realm
	h.perms.On("Find", mock.Anything, 1, 1).Return(shareddomain.Permission{ID: 1}, nil)
	h.perms.On("Find", mock.Anything, 1, 999).Return(shareddomain.Permission{}, errNF)
	h.roles.On("AddPermission", mock.Anything, 5, 1).Return(nil)
	assert.NoError(t, h.uc.AddRolePermission(ctx, "acme", 5, 1))
	assert.Equal(t, 404, rest.HTTPStatus(h.uc.AddRolePermission(ctx, "acme", 5, 999)))

	h.roles.On("RemovePermission", mock.Anything, 5, 1).Return(nil)
	assert.NoError(t, h.uc.RemoveRolePermission(ctx, "acme", 5, 1))
}

func Test_Permissions(t *testing.T) {
	ctx := ctxOf("acme")

	t.Run("create defaults the type to api and rejects duplicates", func(t *testing.T) {
		h := newHarness(t)
		h.perms.On("FindByServiceCode", mock.Anything, 1, "order", "cancel").Return(shareddomain.Permission{}, errNF)
		h.perms.On("Save", mock.Anything, mock.MatchedBy(func(p *shareddomain.Permission) bool { return p.Type == "api" && p.RealmID == 1 })).Return(nil)
		_, err := h.uc.CreatePermission(ctx, "acme", &domain.RequestPermission{Service: "order", Code: "cancel"})
		assert.NoError(t, err)

		h = newHarness(t)
		h.perms.On("FindByServiceCode", mock.Anything, 1, "order", "cancel").Return(shareddomain.Permission{ID: 3}, nil)
		_, err = h.uc.CreatePermission(ctx, "acme", &domain.RequestPermission{Service: "order", Code: "cancel"})
		assert.Equal(t, 409, rest.HTTPStatus(err))
	})

	t.Run("bulk upsert updates existing and creates new", func(t *testing.T) {
		h := newHarness(t)
		h.perms.On("FindByServiceCode", mock.Anything, 1, "order", "cancel").Return(shareddomain.Permission{ID: 3, RealmID: 1, Service: "order", Code: "cancel"}, nil)
		h.perms.On("FindByServiceCode", mock.Anything, 1, "order", "refund").Return(shareddomain.Permission{}, errNF)
		h.perms.On("Save", mock.Anything, mock.Anything).Return(nil)
		res, err := h.uc.UpsertPermissions(ctx, "acme", []domain.RequestPermission{
			{Service: "order", Code: "cancel", Description: "new text"}, {Service: "order", Code: "refund", Type: "ui"},
		})
		assert.NoError(t, err)
		assert.Len(t, res, 2)
		assert.Equal(t, 3, res[0].ID, "existing permission keeps its id")
		assert.Equal(t, "ui", res[1].Type)
		h.perms.AssertNumberOfCalls(t, "Save", 2)
	})

	t.Run("delete goes through the repository cleanup", func(t *testing.T) {
		h := newHarness(t)
		h.perms.On("Find", mock.Anything, 1, 3).Return(shareddomain.Permission{ID: 3}, nil)
		h.perms.On("Delete", mock.Anything, 3).Return(nil)
		assert.NoError(t, h.uc.DeletePermission(ctx, "acme", 3))
	})

	t.Run("another realm's admin is refused", func(t *testing.T) {
		h := newHarness(t)
		_, err := h.uc.GetAllPermission(ctxOf("globex"), "acme", &domain.FilterPermission{})
		assert.Equal(t, 403, rest.HTTPStatus(err))
		h.perms.On("FetchAll", mock.Anything, 1, mock.Anything).Return([]shareddomain.Permission{}, nil)
		h.perms.On("Count", mock.Anything, 1, mock.Anything).Return(0)
		_, err = h.uc.GetAllPermission(ctxOf(common.MasterRealm), "acme", &domain.FilterPermission{})
		assert.NoError(t, err, "master may administer any realm")
	})
}
