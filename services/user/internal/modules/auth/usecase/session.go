package usecase

import (
	"context"

	"monorepo/globalshared/auth"
	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/auth/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

func (uc *authUsecaseImpl) serializeSessions(sessions []shareddomain.Session) []domain.ResponseSession {
	out := make([]domain.ResponseSession, 0, len(sessions))
	for i := range sessions {
		var s domain.ResponseSession
		s.Serialize(&sessions[i])
		out = append(out, s)
	}
	return out
}

func (uc *authUsecaseImpl) listSessions(ctx context.Context, realmID int, filter *domain.FilterSession) (data domain.ResponseSessionList, err error) {
	sessions, err := uc.repoSQL.SessionRepo().FetchAll(ctx, realmID, filter)
	if err != nil {
		return data, err
	}
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.SessionRepo().Count(ctx, realmID, filter))
	data.Data = uc.serializeSessions(sessions)
	return
}

func (uc *authUsecaseImpl) GetAllSession(ctx context.Context, realm string, filter *domain.FilterSession) (data domain.ResponseSessionList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:GetAllSession")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	return uc.listSessions(ctx, r.ID, filter)
}

func (uc *authUsecaseImpl) GetDetailSession(ctx context.Context, realm string, id int) (data domain.ResponseSession, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:GetDetailSession")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	session, err := uc.repoSQL.SessionRepo().Find(ctx, r.ID, id)
	if err != nil {
		return data, common.NotFound(err, "session")
	}
	data.Serialize(&session)
	return
}

func (uc *authUsecaseImpl) RevokeSession(ctx context.Context, realm string, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:RevokeSession")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	session, err := uc.repoSQL.SessionRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "session")
	}
	return uc.repoSQL.SessionRepo().RevokeFamily(ctx, session.FamilyID)
}

func (uc *authUsecaseImpl) RevokeUserSessions(ctx context.Context, realm string, userID int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:RevokeUserSessions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, r.ID, userID)
	if err != nil {
		return common.NotFound(err, "user")
	}
	return uc.repoSQL.SessionRepo().RevokeByUser(ctx, user.ID)
}

// authenticate resolves the principal of the calling token and makes sure its session is still alive.
// The path realm must be the realm of the token.
func (uc *authUsecaseImpl) authenticate(ctx context.Context, realm string) (shareddomain.Realm, shareddomain.User, *candishared.TokenClaim, error) {
	claim := auth.TokenClaimFromContext(ctx)
	if claim == nil {
		return shareddomain.Realm{}, shareddomain.User{}, nil, rest.NewUnauthorized("missing token")
	}
	if auth.RealmFromClaim(claim) != realm {
		return shareddomain.Realm{}, shareddomain.User{}, nil, rest.NewForbidden("token was not issued by realm " + realm)
	}
	r, err := common.LoadRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return r, shareddomain.User{}, nil, err
	}
	var userID int
	if err := scanInt(claim.Subject, &userID); err != nil {
		return r, shareddomain.User{}, nil, rest.NewUnauthorized("invalid token subject")
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, r.ID, userID)
	if err != nil || user.Status == shareddomain.UserStatusDisabled {
		return r, user, nil, rest.NewUnauthorized("account not available")
	}
	if sid := auth.SessionIDFromClaim(claim); sid > 0 {
		active, err := uc.repoSQL.SessionRepo().IsFamilyActive(ctx, sid)
		if err != nil {
			return r, user, nil, err
		}
		if !active {
			return r, user, nil, rest.NewUnauthorized("session was revoked")
		}
	}
	return r, user, claim, nil
}

func (uc *authUsecaseImpl) GetMySessions(ctx context.Context, realm string, filter *domain.FilterSession) (data domain.ResponseSessionList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:GetMySessions")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, user, _, err := uc.authenticate(ctx, realm)
	if err != nil {
		return data, err
	}
	filter.UserID = &user.ID // always own sessions only
	return uc.listSessions(ctx, r.ID, filter)
}

func (uc *authUsecaseImpl) RevokeMySession(ctx context.Context, realm string, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:RevokeMySession")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, user, _, err := uc.authenticate(ctx, realm)
	if err != nil {
		return err
	}
	session, err := uc.repoSQL.SessionRepo().Find(ctx, r.ID, id)
	if err != nil || session.UserID != user.ID {
		return rest.NewNotFound("session not found")
	}
	return uc.repoSQL.SessionRepo().RevokeFamily(ctx, session.FamilyID)
}

func (uc *authUsecaseImpl) Logout(ctx context.Context, realm string) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:Logout")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	claim := auth.TokenClaimFromContext(ctx)
	if claim == nil {
		return rest.NewUnauthorized("missing token")
	}
	if auth.RealmFromClaim(claim) != realm {
		return rest.NewForbidden("token was not issued by realm " + realm)
	}
	sid := auth.SessionIDFromClaim(claim)
	if sid == 0 {
		return nil // service tokens have no session, they simply expire
	}
	return uc.repoSQL.SessionRepo().RevokeFamily(ctx, sid)
}
