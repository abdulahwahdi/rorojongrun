package usecase

import (
	"context"
	"testing"

	"monorepo/services/user/internal/modules/menu/domain"
	"monorepo/services/user/pkg/helper"
	mockclient "monorepo/services/user/pkg/mocks/modules/client/repository"
	mockmenu "monorepo/services/user/pkg/mocks/modules/menu/repository"
	mockrbac "monorepo/services/user/pkg/mocks/modules/rbac/repository"
	mockrealm "monorepo/services/user/pkg/mocks/modules/realm/repository"
	mocksharedrepo "monorepo/services/user/pkg/mocks/shared/repository"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func ip(i int) *int { return &i }

func adminCtx() context.Context {
	claim := &candishared.TokenClaim{RegisteredClaims: jwt.RegisteredClaims{Subject: "1"}}
	claim.Additional = map[string]any{"realm": common.MasterRealm}
	return candishared.SetToContext(context.Background(), candishared.ContextKeyTokenClaim, claim)
}

type menuHarness struct {
	uc    *menuUsecaseImpl
	menus *mockmenu.MenuRepository
	perms *mockrbac.PermissionRepository
}

func newMenuHarness(t *testing.T) *menuHarness {
	repo := &mocksharedrepo.RepoSQL{}
	realms, clients := &mockrealm.RealmRepository{}, &mockclient.ClientRepository{}
	h := &menuHarness{menus: &mockmenu.MenuRepository{}, perms: &mockrbac.PermissionRepository{}}
	repo.On("RealmRepo").Return(realms)
	repo.On("ClientRepo").Return(clients)
	repo.On("MenuRepo").Return(h.menus)
	repo.On("PermissionRepo").Return(h.perms)
	realms.On("FindByName", mock.Anything, "acme").Return(shareddomain.Realm{ID: 1, Name: "acme", Enabled: true}, nil)
	clients.On("FindByClientID", mock.Anything, 1, "web").Return(shareddomain.Client{ID: 7, RealmID: 1, ClientID: "web"}, nil)
	h.uc = &menuUsecaseImpl{repoSQL: repo}
	return h
}

func Test_buildTree_and_descendants(t *testing.T) {
	flat := []domain.ResponseMenu{
		{ID: 1, Key: "a"}, {ID: 2, Key: "b", ParentID: 1}, {ID: 3, Key: "c", ParentID: 2}, {ID: 4, Key: "d", ParentID: 1},
		{ID: 5, Key: "orphan", ParentID: 99}, // parent missing: becomes a root instead of vanishing
	}
	tree := buildTree(flat)
	assert.Len(t, tree, 2)
	assert.Equal(t, "a", tree[0].Key)
	assert.Len(t, tree[0].Children, 2)
	assert.Equal(t, "c", tree[0].Children[0].Children[0].Key)

	menus := []shareddomain.Menu{{ID: 1}, {ID: 2, ParentID: ip(1)}, {ID: 3, ParentID: ip(2)}, {ID: 4, ParentID: ip(1)}, {ID: 5}}
	assert.ElementsMatch(t, []int{2, 3, 4}, descendantIDs(menus, 1))
	assert.Empty(t, descendantIDs(menus, 5))
}

func Test_filterTree(t *testing.T) {
	perms := map[int]shareddomain.Permission{
		10: {ID: 10, Service: "ui", Code: "menu.orders"},
		11: {ID: 11, Service: "ui", Code: "menu.reports"},
	}
	tree := []domain.ResponseMenu{
		{ID: 1, Key: "sales", Children: []domain.ResponseMenu{
			{ID: 2, Key: "orders", Path: "/orders", PermissionID: 10},
			{ID: 3, Key: "reports", Path: "/reports", PermissionID: 11},
		}},
		{ID: 4, Key: "home", Path: "/"}, // no permission: everyone
		{ID: 5, Key: "admin", Children: []domain.ResponseMenu{{ID: 6, Key: "roles", Path: "/roles", PermissionID: 11}}},
	}

	got := filterTree(tree, perms, [][2]string{{"ui", "menu.orders"}})
	keys := func(nodes []domain.ResponseMenu) (k []string) {
		for _, n := range nodes {
			k = append(k, n.Key)
		}
		return
	}
	assert.Equal(t, []string{"sales", "home"}, keys(got), "empty group 'admin' is dropped")
	assert.Equal(t, []string{"orders"}, keys(got[0].Children))

	assert.Equal(t, []string{"sales", "home", "admin"}, keys(filterTree(tree, perms, [][2]string{{"*", "*"}})))
	assert.Equal(t, []string{"home"}, keys(filterTree(tree, perms, nil)))

	// a denied parent hides its whole subtree even if a child would be allowed
	tree2 := []domain.ResponseMenu{{ID: 1, Key: "p", PermissionID: 11, Children: []domain.ResponseMenu{{ID: 2, Key: "c", Path: "/c", PermissionID: 10}}}}
	assert.Empty(t, filterTree(tree2, perms, [][2]string{{"ui", "menu.orders"}}))
}

func Test_CreateMenu(t *testing.T) {
	t.Run("duplicate key", func(t *testing.T) {
		h := newMenuHarness(t)
		h.menus.On("FindByKey", mock.Anything, 7, "orders").Return(shareddomain.Menu{ID: 3}, nil)
		_, err := h.uc.CreateMenu(adminCtx(), "acme", "web", &domain.RequestMenu{Key: "orders", Label: "Orders"})
		assert.Equal(t, 409, helper.HTTPStatus(err))
	})

	t.Run("parent must belong to the client", func(t *testing.T) {
		h := newMenuHarness(t)
		h.menus.On("FindByKey", mock.Anything, 7, "orders").Return(shareddomain.Menu{}, errNF)
		h.menus.On("Find", mock.Anything, 7, 99).Return(shareddomain.Menu{}, errNF)
		_, err := h.uc.CreateMenu(adminCtx(), "acme", "web", &domain.RequestMenu{Key: "orders", Label: "Orders", ParentID: ip(99)})
		assert.Equal(t, 400, helper.HTTPStatus(err))
		h.menus.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("permission must exist in the realm", func(t *testing.T) {
		h := newMenuHarness(t)
		h.menus.On("FindByKey", mock.Anything, 7, "orders").Return(shareddomain.Menu{}, errNF)
		h.perms.On("Find", mock.Anything, 1, 5).Return(shareddomain.Permission{}, errNF)
		_, err := h.uc.CreateMenu(adminCtx(), "acme", "web", &domain.RequestMenu{Key: "orders", Label: "Orders", PermissionID: ip(5)})
		assert.Equal(t, 400, helper.HTTPStatus(err))
	})

	t.Run("a token of another realm cannot touch it", func(t *testing.T) {
		h := newMenuHarness(t)
		_, err := h.uc.CreateMenu(claimFor("globex"), "acme", "web", &domain.RequestMenu{Key: "x", Label: "x"})
		assert.Equal(t, 403, helper.HTTPStatus(err))
	})
}

func Test_UpdateMenu_cycle(t *testing.T) {
	h := newMenuHarness(t)
	all := []shareddomain.Menu{{ID: 1}, {ID: 2, ParentID: ip(1)}, {ID: 3, ParentID: ip(2)}}
	h.menus.On("Find", mock.Anything, 7, 1).Return(all[0], nil)
	h.menus.On("Find", mock.Anything, 7, 3).Return(all[2], nil)
	h.menus.On("FindByKey", mock.Anything, 7, "root").Return(all[0], nil)
	h.menus.On("FetchAllOfClient", mock.Anything, 7).Return(all, nil)

	// moving 1 under its own grandchild 3
	err := h.uc.UpdateMenu(adminCtx(), "acme", "web", 1, &domain.RequestMenu{Key: "root", Label: "root", ParentID: ip(3)})
	assert.Equal(t, 400, helper.HTTPStatus(err))
	// or under itself
	err = h.uc.UpdateMenu(adminCtx(), "acme", "web", 1, &domain.RequestMenu{Key: "root", Label: "root", ParentID: ip(1)})
	assert.Equal(t, 400, helper.HTTPStatus(err))
	h.menus.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func Test_DeleteMenu_cascades(t *testing.T) {
	h := newMenuHarness(t)
	all := []shareddomain.Menu{{ID: 1}, {ID: 2, ParentID: ip(1)}, {ID: 3, ParentID: ip(2)}, {ID: 4}}
	h.menus.On("Find", mock.Anything, 7, 1).Return(all[0], nil)
	h.menus.On("FetchAllOfClient", mock.Anything, 7).Return(all, nil)
	h.menus.On("DeleteMany", mock.Anything, mock.MatchedBy(func(ids []int) bool {
		return assert.ElementsMatch(t, []int{1, 2, 3}, ids)
	})).Return(nil)
	assert.NoError(t, h.uc.DeleteMenu(adminCtx(), "acme", "web", 1))
	h.menus.AssertNumberOfCalls(t, "DeleteMany", 1)
}

func Test_GetUserMenu(t *testing.T) {
	h := newMenuHarness(t)
	pOrders := 10
	h.menus.On("FetchAllOfClient", mock.Anything, 7).Return([]shareddomain.Menu{
		{ID: 1, Key: "sales"}, {ID: 2, Key: "orders", Path: "/orders", ParentID: ip(1), PermissionID: &pOrders},
	}, nil)
	h.perms.On("FetchByIDs", mock.Anything, 1, []int{10}).Return([]shareddomain.Permission{{ID: 10, Service: "ui", Code: "menu.orders"}}, nil)

	got, err := h.uc.GetUserMenu(context.Background(), 1, 7, []shareddomain.Permission{{Service: "ui", Code: "menu.orders"}})
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "ui:menu.orders", got[0].Children[0].Permission)

	got, err = h.uc.GetUserMenu(context.Background(), 1, 7, nil)
	assert.NoError(t, err)
	assert.Empty(t, got)
}
