package repository

import (
	"context"

	"monorepo/services/user/internal/modules/user/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userRepoSQL struct {
	readDB, writeDB *gorm.DB
}

// NewUserRepoSQL constructor
func NewUserRepoSQL(readDB, writeDB *gorm.DB) UserRepository {
	return &userRepoSQL{readDB: readDB, writeDB: writeDB}
}

func (r *userRepoSQL) filter(db *gorm.DB, realmID int, f *domain.FilterUser) *gorm.DB {
	db = helper.Alive(db, "users").Where("users.realm_id = ?", realmID)
	if !f.IncludeServiceAccounts {
		db = db.Where("users.is_service_account = FALSE")
	}
	if f.Username != "" {
		db = db.Where("users.username = ?", f.Username)
	}
	if f.Email != "" {
		db = db.Where("users.email = ?", f.Email)
	}
	if f.Phone != "" {
		db = db.Where("users.phone = ?", f.Phone)
	}
	if f.Status != "" {
		db = db.Where("users.status = ?", f.Status)
	}
	if f.Role != "" {
		db = db.Where("EXISTS (SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id AND r.deleted_at IS NULL "+
			"WHERE ur.user_id = users.id AND r.name = ?)", f.Role)
	}
	if f.Search != "" {
		like := helper.Like(f.Search)
		db = db.Where("(users.username ILIKE ? OR users.email ILIKE ? OR users.phone ILIKE ? OR users.full_name ILIKE ?)", like, like, like, like)
	}
	return db
}

func (r *userRepoSQL) FetchAll(ctx context.Context, realmID int, f *domain.FilterUser) (data []shareddomain.User, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:FetchAll")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.ApplyPaging(r.filter(helper.DB(ctx, r.readDB), realmID, f), &f.Filter, "id", "id", "username", "email", "created_at", "updated_at")
	err = db.Find(&data).Error
	return
}

func (r *userRepoSQL) Count(ctx context.Context, realmID int, f *domain.FilterUser) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:Count")
	defer trace.Finish()

	var total int64
	r.filter(helper.DB(ctx, r.readDB), realmID, f).Model(&shareddomain.User{}).Count(&total)
	return int(total)
}

func (r *userRepoSQL) findBy(ctx context.Context, trace string, where string, args ...any) (res shareddomain.User, err error) {
	t, ctx := tracer.StartTraceWithContext(ctx, trace)
	defer func() { t.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "users").Where(where, args...).First(&res).Error
	return
}

func (r *userRepoSQL) Find(ctx context.Context, realmID, id int) (shareddomain.User, error) {
	return r.findBy(ctx, "UserRepoSQL:Find", "realm_id = ? AND id = ?", realmID, id)
}

func (r *userRepoSQL) FindByUsername(ctx context.Context, realmID int, username string) (shareddomain.User, error) {
	return r.findBy(ctx, "UserRepoSQL:FindByUsername", "realm_id = ? AND username = ?", realmID, username)
}

func (r *userRepoSQL) FindByEmail(ctx context.Context, realmID int, email string) (shareddomain.User, error) {
	return r.findBy(ctx, "UserRepoSQL:FindByEmail", "realm_id = ? AND email = ?", realmID, email)
}

func (r *userRepoSQL) FindByPhone(ctx context.Context, realmID int, phone string) (shareddomain.User, error) {
	return r.findBy(ctx, "UserRepoSQL:FindByPhone", "realm_id = ? AND phone = ?", realmID, phone)
}

func (r *userRepoSQL) Save(ctx context.Context, data *shareddomain.User) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:Save")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.MapDBError(helper.Save(helper.DB(ctx, r.writeDB), data.ID, data))
}

func (r *userRepoSQL) Delete(ctx context.Context, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:Delete")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	db := helper.DB(ctx, r.writeDB)
	if err = db.Where("user_id = ?", id).Delete(&shareddomain.UserRole{}).Error; err != nil {
		return err
	}
	return helper.SoftDelete(db, &shareddomain.User{}, "id = ?", id)
}

func (r *userRepoSQL) Roles(ctx context.Context, userID int) (data []shareddomain.Role, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:Roles")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	err = helper.Alive(helper.DB(ctx, r.readDB), "roles").
		Joins("JOIN user_roles ur ON ur.role_id = roles.id").
		Where("ur.user_id = ?", userID).Order("roles.name").Find(&data).Error
	return
}

func (r *userRepoSQL) ReplaceRoles(ctx context.Context, userID int, roleIDs []int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:ReplaceRoles")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.DB(ctx, r.writeDB).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&shareddomain.UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rows := make([]shareddomain.UserRole, 0, len(roleIDs))
		for _, id := range roleIDs {
			rows = append(rows, shareddomain.UserRole{UserID: userID, RoleID: id})
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
	})
}

func (r *userRepoSQL) AddRole(ctx context.Context, userID, roleID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:AddRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.DB(ctx, r.writeDB).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&shareddomain.UserRole{UserID: userID, RoleID: roleID}).Error
}

func (r *userRepoSQL) RemoveRole(ctx context.Context, userID, roleID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:RemoveRole")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	return helper.DB(ctx, r.writeDB).Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&shareddomain.UserRole{}).Error
}

func (r *userRepoSQL) CountRolesByIDs(ctx context.Context, realmID int, ids []int) int {
	trace, ctx := tracer.StartTraceWithContext(ctx, "UserRepoSQL:CountRolesByIDs")
	defer trace.Finish()

	if len(ids) == 0 {
		return 0
	}
	var total int64
	helper.Alive(helper.DB(ctx, r.readDB), "roles").Model(&shareddomain.Role{}).
		Where("realm_id = ? AND id IN ?", realmID, ids).Count(&total)
	return int(total)
}
