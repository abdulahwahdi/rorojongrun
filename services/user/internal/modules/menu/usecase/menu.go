package usecase

import (
	"context"
	"errors"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/menu/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

// resolve authorizes the realm and loads the client the menus belong to
func (uc *menuUsecaseImpl) resolve(ctx context.Context, realm, clientID string) (shareddomain.Realm, shareddomain.Client, error) {
	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return r, shareddomain.Client{}, err
	}
	client, err := uc.repoSQL.ClientRepo().FindByClientID(ctx, r.ID, clientID)
	if err != nil {
		return r, client, common.NotFound(err, "client")
	}
	return r, client, nil
}

// permissionIndex loads the permissions referenced by menus so responses can show "<service>:<code>"
func (uc *menuUsecaseImpl) permissionIndex(ctx context.Context, realmID int, menus []shareddomain.Menu) (map[int]shareddomain.Permission, error) {
	var ids []int
	for _, m := range menus {
		if m.PermissionID != nil {
			ids = append(ids, *m.PermissionID)
		}
	}
	perms, err := uc.repoSQL.PermissionRepo().FetchByIDs(ctx, realmID, ids)
	if err != nil {
		return nil, err
	}
	idx := make(map[int]shareddomain.Permission, len(perms))
	for _, p := range perms {
		idx[p.ID] = p
	}
	return idx, nil
}

func (uc *menuUsecaseImpl) GetAllMenu(ctx context.Context, realm, clientID string, filter *domain.FilterMenu) (data domain.ResponseMenuList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuUsecase:GetAllMenu")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, client, err := uc.resolve(ctx, realm, clientID)
	if err != nil {
		return data, err
	}
	var menus []shareddomain.Menu
	if filter.Tree {
		menus, err = uc.repoSQL.MenuRepo().FetchAllOfClient(ctx, client.ID)
	} else {
		menus, err = uc.repoSQL.MenuRepo().FetchAll(ctx, client.ID, filter)
	}
	if err != nil {
		return data, err
	}
	perms, err := uc.permissionIndex(ctx, r.ID, menus)
	if err != nil {
		return data, err
	}
	nodes := make([]domain.ResponseMenu, 0, len(menus))
	for i := range menus {
		var n domain.ResponseMenu
		n.Serialize(&menus[i], perms)
		nodes = append(nodes, n)
	}
	if filter.Tree {
		data.Data = buildTree(nodes)
		data.Meta = candishared.NewMeta(1, len(nodes), len(nodes))
		return data, nil
	}
	data.Data = nodes
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.MenuRepo().Count(ctx, client.ID, filter))
	return
}

func (uc *menuUsecaseImpl) GetDetailMenu(ctx context.Context, realm, clientID string, id int) (data domain.ResponseMenu, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuUsecase:GetDetailMenu")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, client, err := uc.resolve(ctx, realm, clientID)
	if err != nil {
		return data, err
	}
	menu, err := uc.repoSQL.MenuRepo().Find(ctx, client.ID, id)
	if err != nil {
		return data, common.NotFound(err, "menu")
	}
	perms, err := uc.permissionIndex(ctx, r.ID, []shareddomain.Menu{menu})
	if err != nil {
		return data, err
	}
	data.Serialize(&menu, perms)
	return
}

// validate checks the parent and permission references of a menu request
func (uc *menuUsecaseImpl) validate(ctx context.Context, realmID int, client shareddomain.Client, selfID int, req *domain.RequestMenu) error {
	if req.ParentID != nil {
		if *req.ParentID == selfID && selfID != 0 {
			return rest.NewInvalid("a menu cannot be its own parent")
		}
		if _, err := uc.repoSQL.MenuRepo().Find(ctx, client.ID, *req.ParentID); err != nil {
			return rest.NewInvalid("parent menu not found in this client")
		}
		if selfID != 0 { // moving under a descendant would create a cycle
			all, err := uc.repoSQL.MenuRepo().FetchAllOfClient(ctx, client.ID)
			if err != nil {
				return err
			}
			for _, id := range descendantIDs(all, selfID) {
				if id == *req.ParentID {
					return rest.NewInvalid("a menu cannot be moved under its own descendant")
				}
			}
		}
	}
	if req.PermissionID != nil {
		if _, err := uc.repoSQL.PermissionRepo().Find(ctx, realmID, *req.PermissionID); err != nil {
			return rest.NewInvalid("permission not found in this realm")
		}
	}
	return nil
}

func (uc *menuUsecaseImpl) CreateMenu(ctx context.Context, realm, clientID string, req *domain.RequestMenu) (res domain.ResponseMenu, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuUsecase:CreateMenu")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, client, err := uc.resolve(ctx, realm, clientID)
	if err != nil {
		return res, err
	}
	if _, err = uc.repoSQL.MenuRepo().FindByKey(ctx, client.ID, req.Key); err == nil {
		return res, rest.NewConflict("menu key " + req.Key + " already exists for client " + clientID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return res, err
	}
	if err = uc.validate(ctx, r.ID, client, 0, req); err != nil {
		return res, err
	}
	menu := shareddomain.Menu{
		RealmID: r.ID, ClientID: client.ID, ParentID: req.ParentID, Key: req.Key, Label: req.Label,
		Path: req.Path, Icon: req.Icon, SortOrder: req.SortOrder, PermissionID: req.PermissionID,
	}
	if err = uc.repoSQL.MenuRepo().Save(ctx, &menu); err != nil {
		return res, err
	}
	perms, err := uc.permissionIndex(ctx, r.ID, []shareddomain.Menu{menu})
	if err != nil {
		return res, err
	}
	res.Serialize(&menu, perms)
	return
}

func (uc *menuUsecaseImpl) UpdateMenu(ctx context.Context, realm, clientID string, id int, req *domain.RequestMenu) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuUsecase:UpdateMenu")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, client, err := uc.resolve(ctx, realm, clientID)
	if err != nil {
		return err
	}
	menu, err := uc.repoSQL.MenuRepo().Find(ctx, client.ID, id)
	if err != nil {
		return common.NotFound(err, "menu")
	}
	if existing, findErr := uc.repoSQL.MenuRepo().FindByKey(ctx, client.ID, req.Key); findErr == nil && existing.ID != menu.ID {
		return rest.NewConflict("menu key " + req.Key + " already exists for client " + clientID)
	}
	if err = uc.validate(ctx, r.ID, client, menu.ID, req); err != nil {
		return err
	}
	menu.ParentID, menu.Key, menu.Label, menu.Path, menu.Icon = req.ParentID, req.Key, req.Label, req.Path, req.Icon
	menu.SortOrder, menu.PermissionID = req.SortOrder, req.PermissionID
	return uc.repoSQL.MenuRepo().Save(ctx, &menu)
}

func (uc *menuUsecaseImpl) DeleteMenu(ctx context.Context, realm, clientID string, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuUsecase:DeleteMenu")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	_, client, err := uc.resolve(ctx, realm, clientID)
	if err != nil {
		return err
	}
	menu, err := uc.repoSQL.MenuRepo().Find(ctx, client.ID, id)
	if err != nil {
		return common.NotFound(err, "menu")
	}
	all, err := uc.repoSQL.MenuRepo().FetchAllOfClient(ctx, client.ID)
	if err != nil {
		return err
	}
	ids := append([]int{menu.ID}, descendantIDs(all, menu.ID)...)
	return uc.repoSQL.MenuRepo().DeleteMany(ctx, ids)
}

func (uc *menuUsecaseImpl) GetUserMenu(ctx context.Context, realmID, clientDBID int, granted []shareddomain.Permission) (data []domain.ResponseMenu, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "MenuUsecase:GetUserMenu")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	menus, err := uc.repoSQL.MenuRepo().FetchAllOfClient(ctx, clientDBID)
	if err != nil {
		return nil, err
	}
	perms, err := uc.permissionIndex(ctx, realmID, menus)
	if err != nil {
		return nil, err
	}
	pairs := make([][2]string, 0, len(granted))
	for _, p := range granted {
		pairs = append(pairs, [2]string{p.Service, p.Code})
	}
	nodes := make([]domain.ResponseMenu, 0, len(menus))
	for i := range menus {
		var n domain.ResponseMenu
		n.Serialize(&menus[i], perms)
		nodes = append(nodes, n)
	}
	return filterTree(buildTree(nodes), perms, pairs), nil
}

// filterTree drops nodes whose permission is not granted (with their subtree) and
// group nodes (no path) left without visible children
func filterTree(nodes []domain.ResponseMenu, perms map[int]shareddomain.Permission, granted [][2]string) []domain.ResponseMenu {
	out := make([]domain.ResponseMenu, 0, len(nodes))
	for _, n := range nodes {
		if n.PermissionID != 0 {
			p, ok := perms[n.PermissionID]
			if !ok || !helper.PermissionAllows(granted, p.Service, p.Code) {
				continue
			}
		}
		n.Children = filterTree(n.Children, perms, granted)
		if n.Path == "" && len(n.Children) == 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

// buildTree nests a flat, already ordered list; nodes whose parent is missing become roots
func buildTree(flat []domain.ResponseMenu) []domain.ResponseMenu {
	byParent := map[int][]domain.ResponseMenu{}
	known := map[int]bool{}
	for _, n := range flat {
		known[n.ID] = true
	}
	for _, n := range flat {
		parent := n.ParentID
		if !known[parent] {
			parent = 0
		}
		byParent[parent] = append(byParent[parent], n)
	}
	var attach func(parent int) []domain.ResponseMenu
	attach = func(parent int) []domain.ResponseMenu {
		nodes := byParent[parent]
		out := make([]domain.ResponseMenu, 0, len(nodes))
		for _, n := range nodes {
			n.Children = attach(n.ID)
			out = append(out, n)
		}
		return out
	}
	return attach(0)
}

// descendantIDs returns the ids of every descendant of root
func descendantIDs(all []shareddomain.Menu, root int) []int {
	children := map[int][]int{}
	for _, m := range all {
		if m.ParentID != nil {
			children[*m.ParentID] = append(children[*m.ParentID], m.ID)
		}
	}
	var out []int
	stack := []int{root}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, c := range children[cur] {
			out = append(out, c)
			stack = append(stack, c)
		}
	}
	return out
}
