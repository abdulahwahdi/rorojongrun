package usecase

import (
	"context"
	"testing"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/user/domain"
	"monorepo/services/user/pkg/helper"
	mockauthrepo "monorepo/services/user/pkg/mocks/modules/auth/repository"
	mockrbac "monorepo/services/user/pkg/mocks/modules/rbac/repository"
	mockrealm "monorepo/services/user/pkg/mocks/modules/realm/repository"
	mockuser "monorepo/services/user/pkg/mocks/modules/user/repository"
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

func masterCtx() context.Context { return realmCtx(common.MasterRealm) }

func realmCtx(realm string) context.Context {
	claim := &candishared.TokenClaim{RegisteredClaims: jwt.RegisteredClaims{Subject: "1"}}
	claim.Additional = map[string]any{"realm": realm}
	return candishared.SetToContext(context.Background(), candishared.ContextKeyTokenClaim, claim)
}

type harness struct {
	uc       *userUsecaseImpl
	users    *mockuser.UserRepository
	sessions *mockauthrepo.SessionRepository
	roles    *mockrbac.RoleRepository
}

func newHarness(t *testing.T) *harness {
	repo := &mocksharedrepo.RepoSQL{}
	realms := &mockrealm.RealmRepository{}
	h := &harness{users: &mockuser.UserRepository{}, sessions: &mockauthrepo.SessionRepository{}, roles: &mockrbac.RoleRepository{}}
	repo.On("RealmRepo").Return(realms)
	repo.On("UserRepo").Return(h.users)
	repo.On("SessionRepo").Return(h.sessions)
	repo.On("RoleRepo").Return(h.roles)
	repo.On("WithTransaction", mock.Anything, mock.Anything).Return(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
	realms.On("FindByName", mock.Anything, "acme").Return(shareddomain.Realm{ID: 1, Name: "acme", Enabled: true}, nil)
	h.uc = &userUsecaseImpl{repoSQL: repo}
	return h
}

func Test_CreateUser(t *testing.T) {
	req := func() *domain.RequestCreateUser {
		return &domain.RequestCreateUser{Username: " Alice ", Email: "Alice@Example.com", Phone: "0811", Password: "password1", RoleIDs: []int{2, 2}}
	}
	free := func(h *harness) {
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(shareddomain.User{}, errNF)
		h.users.On("FindByEmail", mock.Anything, 1, "alice@example.com").Return(shareddomain.User{}, errNF)
		h.users.On("FindByPhone", mock.Anything, 1, "0811").Return(shareddomain.User{}, errNF)
	}

	t.Run("normalizes, hashes the password and assigns de-duplicated roles", func(t *testing.T) {
		h := newHarness(t)
		free(h)
		h.users.On("CountRolesByIDs", mock.Anything, 1, []int{2}).Return(1)
		h.users.On("Save", mock.Anything, mock.MatchedBy(func(u *shareddomain.User) bool {
			return u.Username == "alice" && *u.Email == "alice@example.com" && u.PasswordHash != "password1" &&
				helper.CheckSecret(u.PasswordHash, "password1") && u.Status == shareddomain.UserStatusActive
		})).Return(nil)
		h.users.On("ReplaceRoles", mock.Anything, 0, []int{2}).Return(nil)
		res, err := h.uc.CreateUser(masterCtx(), "acme", req())
		assert.NoError(t, err)
		assert.Equal(t, "alice", res.Username)
	})

	t.Run("duplicates are 409, per realm", func(t *testing.T) {
		for _, taken := range []string{"username", "email", "phone"} {
			h := newHarness(t)
			other := shareddomain.User{ID: 77}
			for _, c := range []struct{ name, method string }{{"username", "FindByUsername"}, {"email", "FindByEmail"}, {"phone", "FindByPhone"}} {
				if c.name == taken {
					h.users.On(c.method, mock.Anything, 1, mock.Anything).Return(other, nil)
				} else {
					h.users.On(c.method, mock.Anything, 1, mock.Anything).Return(shareddomain.User{}, errNF)
				}
			}
			_, err := h.uc.CreateUser(masterCtx(), "acme", req())
			assert.Equal(t, 409, rest.HTTPStatus(err), taken)
			h.users.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
		}
	})

	t.Run("weak password, bad status, unknown role", func(t *testing.T) {
		h := newHarness(t)
		r := req()
		r.Password = "short"
		_, err := h.uc.CreateUser(masterCtx(), "acme", r)
		assert.Equal(t, 400, rest.HTTPStatus(err))

		r = req()
		r.Status = "locked"
		_, err = h.uc.CreateUser(masterCtx(), "acme", r)
		assert.Equal(t, 400, rest.HTTPStatus(err))

		free(h)
		h.users.On("CountRolesByIDs", mock.Anything, 1, []int{2}).Return(0)
		_, err = h.uc.CreateUser(masterCtx(), "acme", req())
		assert.Equal(t, 400, rest.HTTPStatus(err))
	})

	t.Run("cannot administer another realm", func(t *testing.T) {
		h := newHarness(t)
		_, err := h.uc.CreateUser(realmCtx("globex"), "acme", req())
		assert.Equal(t, 403, rest.HTTPStatus(err))
		_, err = h.uc.CreateUser(context.Background(), "acme", req())
		assert.Equal(t, 403, rest.HTTPStatus(err))
	})
}

func Test_UpdateUser(t *testing.T) {
	existing := shareddomain.User{ID: 5, RealmID: 1, Username: "alice", Status: shareddomain.UserStatusActive, FailedAttempts: 2}
	req := &domain.RequestUpdateUser{Username: "alice", FullName: "Alice", Status: shareddomain.UserStatusDisabled}

	t.Run("disabling revokes every session", func(t *testing.T) {
		h := newHarness(t)
		h.users.On("Find", mock.Anything, 1, 5).Return(existing, nil)
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(existing, nil) // itself: not a conflict
		h.users.On("Save", mock.Anything, mock.Anything).Return(nil)
		h.sessions.On("RevokeByUser", mock.Anything, 5).Return(nil)
		assert.NoError(t, h.uc.UpdateUser(masterCtx(), "acme", 5, req))
		h.sessions.AssertCalled(t, "RevokeByUser", mock.Anything, 5)
	})

	t.Run("re-activating clears the lockout", func(t *testing.T) {
		h := newHarness(t)
		h.users.On("Find", mock.Anything, 1, 5).Return(existing, nil)
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(existing, nil)
		h.users.On("Save", mock.Anything, mock.MatchedBy(func(u *shareddomain.User) bool { return u.FailedAttempts == 0 && u.LockedUntil == nil })).Return(nil)
		r := *req
		r.Status = shareddomain.UserStatusActive
		assert.NoError(t, h.uc.UpdateUser(masterCtx(), "acme", 5, &r))
		h.sessions.AssertNotCalled(t, "RevokeByUser", mock.Anything, mock.Anything)
	})

	t.Run("username taken by someone else", func(t *testing.T) {
		h := newHarness(t)
		h.users.On("Find", mock.Anything, 1, 5).Return(existing, nil)
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(shareddomain.User{ID: 6}, nil)
		assert.Equal(t, 409, rest.HTTPStatus(h.uc.UpdateUser(masterCtx(), "acme", 5, req)))
	})

	t.Run("service accounts are managed through their client", func(t *testing.T) {
		h := newHarness(t)
		svc := existing
		svc.IsServiceAccount = true
		h.users.On("Find", mock.Anything, 1, 5).Return(svc, nil)
		assert.Equal(t, 409, rest.HTTPStatus(h.uc.UpdateUser(masterCtx(), "acme", 5, req)))
		assert.Equal(t, 409, rest.HTTPStatus(h.uc.DeleteUser(masterCtx(), "acme", 5)))
		assert.Equal(t, 409, rest.HTTPStatus(h.uc.SetPassword(masterCtx(), "acme", 5, "password1")))
	})

	t.Run("not found", func(t *testing.T) {
		h := newHarness(t)
		h.users.On("Find", mock.Anything, 1, 5).Return(shareddomain.User{}, errNF)
		assert.Equal(t, 404, rest.HTTPStatus(h.uc.UpdateUser(masterCtx(), "acme", 5, req)))
	})
}

func Test_SetPassword_and_DeleteUser_revokeSessions(t *testing.T) {
	existing := shareddomain.User{ID: 5, RealmID: 1, Username: "alice", Status: shareddomain.UserStatusActive, FailedAttempts: 3}

	h := newHarness(t)
	h.users.On("Find", mock.Anything, 1, 5).Return(existing, nil)
	h.users.On("Save", mock.Anything, mock.MatchedBy(func(u *shareddomain.User) bool {
		return helper.CheckSecret(u.PasswordHash, "new-password") && u.FailedAttempts == 0
	})).Return(nil)
	h.sessions.On("RevokeByUser", mock.Anything, 5).Return(nil)
	assert.NoError(t, h.uc.SetPassword(masterCtx(), "acme", 5, "new-password"))
	h.sessions.AssertCalled(t, "RevokeByUser", mock.Anything, 5)
	assert.Equal(t, 400, rest.HTTPStatus(h.uc.SetPassword(masterCtx(), "acme", 5, "short")))

	h = newHarness(t)
	h.users.On("Find", mock.Anything, 1, 5).Return(existing, nil)
	h.sessions.On("RevokeByUser", mock.Anything, 5).Return(nil)
	h.users.On("Delete", mock.Anything, 5).Return(nil)
	assert.NoError(t, h.uc.DeleteUser(masterCtx(), "acme", 5))
	h.users.AssertCalled(t, "Delete", mock.Anything, 5)
}

func Test_UserRoles(t *testing.T) {
	h := newHarness(t)
	h.users.On("Find", mock.Anything, 1, 5).Return(shareddomain.User{ID: 5}, nil)
	h.roles.On("Find", mock.Anything, 1, 2).Return(shareddomain.Role{ID: 2}, nil)
	h.roles.On("Find", mock.Anything, 1, 3).Return(shareddomain.Role{}, errNF)
	h.users.On("AddRole", mock.Anything, 5, 2).Return(nil)
	assert.NoError(t, h.uc.AddUserRole(masterCtx(), "acme", 5, 2))
	assert.Equal(t, 404, rest.HTTPStatus(h.uc.AddUserRole(masterCtx(), "acme", 5, 3)), "role of another realm / missing")

	h.users.On("CountRolesByIDs", mock.Anything, 1, []int{2, 3}).Return(1)
	assert.Equal(t, 400, rest.HTTPStatus(h.uc.ReplaceUserRoles(masterCtx(), "acme", 5, []int{2, 3, 3})))
	h.users.On("CountRolesByIDs", mock.Anything, 1, []int{2}).Return(1)
	h.users.On("ReplaceRoles", mock.Anything, 5, []int{2}).Return(nil)
	assert.NoError(t, h.uc.ReplaceUserRoles(masterCtx(), "acme", 5, []int{2}))
	h.users.On("ReplaceRoles", mock.Anything, 5, []int(nil)).Return(nil).Maybe()

	h.users.On("RemoveRole", mock.Anything, 5, 2).Return(nil)
	assert.NoError(t, h.uc.RemoveUserRole(masterCtx(), "acme", 5, 2))
}
