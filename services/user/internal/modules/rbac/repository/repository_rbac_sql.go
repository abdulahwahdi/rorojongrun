package repository

import (
	"context"

	"monorepo/services/user/internal/modules/rbac/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type roleRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewRoleRepoSQL constructor
func NewRoleRepoSQL(readDB, writeDB *gorm.DB) RoleRepository {
	return &roleRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *roleRepoSQL) filter(db *gorm.DB, realmID int, f *domain.FilterRole) *gorm.DB {
	db = helper.Alive(db, "roles").Where("realm_id = ?", realmID)
	if f.Name != "" {
		db = db.Where("name = ?", f.Name)
	}
	if f.Search != "" {
		db = db.Where("(name ILIKE ? OR description ILIKE ?)", helper.Like(f.Search), helper.Like(f.Search))
	}
	return db
}

func (r *roleRepoSQL) FetchAll(ctx context.Context, realmID int, f *domain.FilterRole) (data []shareddomain.Role, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.ApplyPaging(r.filter(helper.DB(ctx, r.readDB), realmID, f), &f.Filter, "id", "id", "name", "created_at", "updated_at")
	err = db.Find(&data).Error
	return
}

func (r *roleRepoSQL) Count(ctx context.Context, realmID int, f *domain.FilterRole) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(helper.DB(ctx, r.readDB), realmID, f).Model(&shareddomain.Role{}).Count(&total)
	return int(total)
}

func (r *roleRepoSQL) Find(ctx context.Context, realmID, id int) (res shareddomain.Role, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:Find")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "roles").Where("realm_id = ? AND id = ?", realmID, id).First(&res).Error
	return
}

func (r *roleRepoSQL) FindByName(ctx context.Context, realmID int, name string) (res shareddomain.Role, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:FindByName")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "roles").Where("realm_id = ? AND name = ?", realmID, name).First(&res).Error
	return
}

func (r *roleRepoSQL) Save(ctx context.Context, data *shareddomain.Role) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.Save(helper.DB(ctx, r.writeDB), data.ID, data)
}

func (r *roleRepoSQL) Delete(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:Delete")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.DB(ctx, r.writeDB)
	if err = db.Where("role_id = ?", id).Delete(&shareddomain.UserRole{}).Error; err != nil {
		return err
	}
	if err = db.Where("role_id = ?", id).Delete(&shareddomain.RolePermission{}).Error; err != nil {
		return err
	}
	return helper.SoftDelete(db, &shareddomain.Role{}, "id = ?", id)
}

func (r *roleRepoSQL) CountUsers(ctx context.Context, roleID int) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:CountUsers")
	defer trace.Finish()

	var total int64
	helper.DB(ctx, r.readDB).Model(&shareddomain.UserRole{}).Where("role_id = ?", roleID).Count(&total)
	return int(total)
}

func (r *roleRepoSQL) Permissions(ctx context.Context, roleID int) (data []shareddomain.Permission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:Permissions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "permissions").
		Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
		Where("rp.role_id = ?", roleID).Order("permissions.service, permissions.code").Find(&data).Error
	return
}

func (r *roleRepoSQL) ReplacePermissions(ctx context.Context, roleID int, permissionIDs []int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:ReplacePermissions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.DB(ctx, r.writeDB)
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&shareddomain.RolePermission{}).Error; err != nil {
			return err
		}
		if len(permissionIDs) == 0 {
			return nil
		}
		rows := make([]shareddomain.RolePermission, 0, len(permissionIDs))
		for _, id := range permissionIDs {
			rows = append(rows, shareddomain.RolePermission{RoleID: roleID, PermissionID: id})
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
	})
}

func (r *roleRepoSQL) AddPermission(ctx context.Context, roleID, permissionID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:AddPermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.DB(ctx, r.writeDB).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&shareddomain.RolePermission{RoleID: roleID, PermissionID: permissionID}).Error
}

func (r *roleRepoSQL) RemovePermission(ctx context.Context, roleID, permissionID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RoleRepoSQL:RemovePermission")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.DB(ctx, r.writeDB).Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Delete(&shareddomain.RolePermission{}).Error
}

type permissionRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewPermissionRepoSQL constructor
func NewPermissionRepoSQL(readDB, writeDB *gorm.DB) PermissionRepository {
	return &permissionRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *permissionRepoSQL) filter(db *gorm.DB, realmID int, f *domain.FilterPermission) *gorm.DB {
	db = helper.Alive(db, "permissions").Where("realm_id = ?", realmID)
	if f.Service != "" {
		db = db.Where("service = ?", f.Service)
	}
	if f.Code != "" {
		db = db.Where("code = ?", f.Code)
	}
	if f.Type != "" {
		db = db.Where("type = ?", f.Type)
	}
	if f.Search != "" {
		db = db.Where("(code ILIKE ? OR description ILIKE ?)", helper.Like(f.Search), helper.Like(f.Search))
	}
	return db
}

func (r *permissionRepoSQL) FetchAll(ctx context.Context, realmID int, f *domain.FilterPermission) (data []shareddomain.Permission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.ApplyPaging(r.filter(helper.DB(ctx, r.readDB), realmID, f), &f.Filter, "id", "id", "service", "code", "created_at", "updated_at")
	err = db.Find(&data).Error
	return
}

func (r *permissionRepoSQL) Count(ctx context.Context, realmID int, f *domain.FilterPermission) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(helper.DB(ctx, r.readDB), realmID, f).Model(&shareddomain.Permission{}).Count(&total)
	return int(total)
}

func (r *permissionRepoSQL) Find(ctx context.Context, realmID, id int) (res shareddomain.Permission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:Find")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "permissions").Where("realm_id = ? AND id = ?", realmID, id).First(&res).Error
	return
}

func (r *permissionRepoSQL) FindByServiceCode(ctx context.Context, realmID int, service, code string) (res shareddomain.Permission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:FindByServiceCode")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "permissions").
		Where("realm_id = ? AND service = ? AND code = ?", realmID, service, code).First(&res).Error
	return
}

func (r *permissionRepoSQL) FetchByIDs(ctx context.Context, realmID int, ids []int) (data []shareddomain.Permission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:FetchByIDs")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	if len(ids) == 0 {
		return nil, nil
	}
	err = helper.Alive(helper.DB(ctx, r.readDB), "permissions").Where("realm_id = ? AND id IN ?", realmID, ids).Find(&data).Error
	return
}

func (r *permissionRepoSQL) CountByIDs(ctx context.Context, realmID int, ids []int) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:CountByIDs")
	defer trace.Finish()

	if len(ids) == 0 {
		return 0
	}
	var total int64
	helper.Alive(helper.DB(ctx, r.readDB), "permissions").Model(&shareddomain.Permission{}).
		Where("realm_id = ? AND id IN ?", realmID, ids).Count(&total)
	return int(total)
}

func (r *permissionRepoSQL) Save(ctx context.Context, data *shareddomain.Permission) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.Save(helper.DB(ctx, r.writeDB), data.ID, data)
}

func (r *permissionRepoSQL) Delete(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:Delete")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.DB(ctx, r.writeDB)
	if err = db.Where("permission_id = ?", id).Delete(&shareddomain.RolePermission{}).Error; err != nil {
		return err
	}
	if err = db.Model(&shareddomain.Menu{}).Where("permission_id = ?", id).Update("permission_id", nil).Error; err != nil {
		return err
	}
	return helper.SoftDelete(db, &shareddomain.Permission{}, "id = ?", id)
}

func (r *permissionRepoSQL) EffectiveForUser(ctx context.Context, realmID, userID int) (data []shareddomain.Permission, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PermissionRepoSQL:EffectiveForUser")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "permissions").
		Joins("JOIN role_permissions rp ON rp.permission_id = permissions.id").
		Joins("JOIN roles r ON r.id = rp.role_id AND r.deleted_at IS NULL").
		Joins("JOIN user_roles ur ON ur.role_id = r.id").
		Where("ur.user_id = ? AND permissions.realm_id = ?", userID, realmID).
		Distinct().Find(&data).Error
	return
}
