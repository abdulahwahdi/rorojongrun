package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/auth/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

var errRefreshReuse = errors.New("refresh token reuse")

var (
	dummyHashOnce sync.Once
	dummyHash     string
)

// burnPasswordCheck spends the same time as a real password check, so response time does not
// reveal whether a username exists
func burnPasswordCheck(password string) {
	dummyHashOnce.Do(func() { dummyHash, _ = helper.HashSecret(helper.RandomToken(16)) })
	helper.CheckSecret(dummyHash, password)
}

func grantAllowed(client shareddomain.Client, grant string) bool {
	for _, g := range strings.Split(client.GrantTypes, ",") {
		if g == grant {
			return true
		}
	}
	return false
}

// loadClient authenticates the client of a token request: it must exist, be enabled, allow the grant
// and — when confidential — present its secret
func (uc *authUsecaseImpl) loadClient(ctx context.Context, realm shareddomain.Realm, req *domain.RequestToken) (shareddomain.Client, error) {
	client, err := uc.repoSQL.ClientRepo().FindByClientID(ctx, realm.ID, req.ClientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return client, rest.NewUnauthorized("invalid client")
		}
		return client, err
	}
	if !client.Enabled {
		return client, rest.NewUnauthorized("invalid client")
	}
	if client.Type == shareddomain.ClientTypeConfidential && !helper.CheckSecret(client.SecretHash, req.ClientSecret) {
		return client, rest.NewUnauthorized("invalid client credentials")
	}
	if !grantAllowed(client, req.GrantType) {
		return client, rest.NewUnauthorized("grant type " + req.GrantType + " is not allowed for this client")
	}
	return client, nil
}

func (uc *authUsecaseImpl) Token(ctx context.Context, realmName string, req *domain.RequestToken, meta domain.ClientMeta) (res domain.ResponseToken, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthUsecase:Token")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()
	trace.SetTag("grantType", req.GrantType)

	realm, err := common.LoadRealm(ctx, uc.repoSQL, realmName)
	if err != nil {
		return res, err
	}
	if !realm.Enabled {
		return res, rest.NewForbidden("realm " + realmName + " is disabled")
	}
	client, err := uc.loadClient(ctx, realm, req)
	if err != nil {
		return res, err
	}

	switch req.GrantType {
	case domain.GrantPassword:
		return uc.grantPassword(ctx, realm, client, req, meta)
	case domain.GrantRefreshToken:
		return uc.grantRefreshToken(ctx, realm, client, req, meta)
	case domain.GrantClientCredentials:
		return uc.grantClientCredentials(ctx, realm, client)
	case domain.GrantOTP:
		return uc.grantOTP(ctx, realm, client, req, meta)
	}
	return res, rest.NewInvalid("unsupported grant type " + req.GrantType)
}

// activeUser rejects users that may not log in
func activeUser(user shareddomain.User) error {
	if user.Status == shareddomain.UserStatusDisabled {
		return rest.NewForbidden("account is disabled")
	}
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return rest.NewLocked("account is temporarily locked, try again later")
	}
	return nil
}

func (uc *authUsecaseImpl) grantPassword(ctx context.Context, realm shareddomain.Realm, client shareddomain.Client, req *domain.RequestToken, meta domain.ClientMeta) (res domain.ResponseToken, err error) {
	login := strings.ToLower(strings.TrimSpace(req.Username))
	user, err := uc.repoSQL.UserRepo().FindByUsername(ctx, realm.ID, login)
	if errors.Is(err, gorm.ErrRecordNotFound) && strings.Contains(login, "@") {
		user, err = uc.repoSQL.UserRepo().FindByEmail(ctx, realm.ID, login)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && user.IsServiceAccount) {
		burnPasswordCheck(req.Password)
		return res, rest.NewUnauthorized("invalid credentials")
	}
	if err != nil {
		return res, err
	}
	if lockErr := activeUser(user); lockErr != nil && user.Status != shareddomain.UserStatusDisabled {
		return res, lockErr // locked: do not even evaluate the password
	}
	if !helper.CheckSecret(user.PasswordHash, req.Password) {
		return res, uc.registerFailure(ctx, realm, user)
	}
	if err = activeUser(user); err != nil {
		return res, err
	}
	return uc.loginSucceeded(ctx, realm, client, user, meta)
}

// registerFailure counts a wrong password and locks the account at the realm's threshold
func (uc *authUsecaseImpl) registerFailure(ctx context.Context, realm shareddomain.Realm, user shareddomain.User) error {
	user.FailedAttempts++
	if realm.MaxFailedAttempts > 0 && realm.LockoutSec > 0 && user.FailedAttempts >= realm.MaxFailedAttempts {
		until := time.Now().Add(time.Duration(realm.LockoutSec) * time.Second)
		user.LockedUntil, user.FailedAttempts = &until, 0
	}
	if err := uc.repoSQL.UserRepo().Save(ctx, &user); err != nil {
		return err
	}
	return rest.NewUnauthorized("invalid credentials")
}

// loginSucceeded resets the lockout counters and issues a session with tokens
func (uc *authUsecaseImpl) loginSucceeded(ctx context.Context, realm shareddomain.Realm, client shareddomain.Client, user shareddomain.User, meta domain.ClientMeta) (res domain.ResponseToken, err error) {
	now := time.Now()
	user.FailedAttempts, user.LockedUntil, user.LastLoginAt = 0, nil, &now
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.UserRepo().Save(ctx, &user); err != nil {
			return err
		}
		res, err = uc.issueSession(ctx, realm, client, user, 0, meta)
		return err
	})
	return
}

// issueSession stores a new refresh token (in family, or a new family when 0) and signs the access token
func (uc *authUsecaseImpl) issueSession(ctx context.Context, realm shareddomain.Realm, client shareddomain.Client, user shareddomain.User, familyID int, meta domain.ClientMeta) (res domain.ResponseToken, err error) {
	refresh := helper.RandomToken(32)
	ua := meta.UserAgent
	if len(ua) > 255 {
		ua = ua[:255]
	}
	session := shareddomain.Session{
		RealmID: realm.ID, UserID: user.ID, ClientID: client.ID, FamilyID: familyID,
		RefreshHash: helper.SHA256Hex(refresh), ExpiresAt: time.Now().Add(time.Duration(realm.RefreshTokenTTLSec) * time.Second),
		IP: meta.IP, UserAgent: ua,
	}
	if err = uc.repoSQL.SessionRepo().Save(ctx, &session); err != nil {
		return res, err
	}
	access, expiresIn, err := uc.signAccessToken(ctx, realm, client.ClientID, user, session.FamilyID)
	if err != nil {
		return res, err
	}
	return domain.ResponseToken{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: expiresIn,
		RefreshToken: refresh, RefreshExpiresIn: realm.RefreshTokenTTLSec,
	}, nil
}

func (uc *authUsecaseImpl) grantRefreshToken(ctx context.Context, realm shareddomain.Realm, client shareddomain.Client, req *domain.RequestToken, meta domain.ClientMeta) (res domain.ResponseToken, err error) {
	invalid := rest.NewUnauthorized("invalid refresh token")
	session, err := uc.repoSQL.SessionRepo().FindByRefreshHash(ctx, helper.SHA256Hex(req.RefreshToken))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return res, invalid
	}
	if err != nil {
		return res, err
	}
	if session.RealmID != realm.ID || session.ClientID != client.ID || session.RevokedAt != nil || time.Now().After(session.ExpiresAt) {
		return res, invalid
	}
	if session.RotatedAt != nil { // an already used token comes back: assume theft, kill the whole login
		if err = uc.repoSQL.SessionRepo().RevokeFamily(ctx, session.FamilyID); err != nil {
			return res, err
		}
		return res, rest.NewUnauthorized("refresh token reuse detected, the session was revoked")
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, realm.ID, session.UserID)
	if err != nil || user.Status == shareddomain.UserStatusDisabled {
		return res, invalid
	}

	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		ok, err := uc.repoSQL.SessionRepo().MarkRotated(ctx, session.ID)
		if err != nil {
			return err
		}
		if !ok { // lost a race against another use of the same token
			return errRefreshReuse
		}
		res, err = uc.issueSession(ctx, realm, client, user, session.FamilyID, meta)
		return err
	})
	if errors.Is(err, errRefreshReuse) {
		if revokeErr := uc.repoSQL.SessionRepo().RevokeFamily(ctx, session.FamilyID); revokeErr != nil {
			return res, revokeErr
		}
		return res, rest.NewUnauthorized("refresh token reuse detected, the session was revoked")
	}
	return
}

func (uc *authUsecaseImpl) grantClientCredentials(ctx context.Context, realm shareddomain.Realm, client shareddomain.Client) (res domain.ResponseToken, err error) {
	if client.Type != shareddomain.ClientTypeConfidential || client.ServiceUserID == nil {
		return res, rest.NewUnauthorized("client_credentials requires a confidential client")
	}
	user, err := uc.repoSQL.UserRepo().Find(ctx, realm.ID, *client.ServiceUserID)
	if err != nil {
		return res, rest.NewUnauthorized("service account not found")
	}
	access, expiresIn, err := uc.signAccessToken(ctx, realm, client.ClientID, user, 0)
	if err != nil {
		return res, err
	}
	return domain.ResponseToken{AccessToken: access, TokenType: "Bearer", ExpiresIn: expiresIn}, nil
}

func (uc *authUsecaseImpl) grantOTP(ctx context.Context, realm shareddomain.Realm, client shareddomain.Client, req *domain.RequestToken, meta domain.ClientMeta) (res domain.ResponseToken, err error) {
	if !realm.OTPLoginEnabled {
		return res, rest.NewForbidden("otp login is not enabled for this realm")
	}
	notifier := uc.notification()
	if notifier == nil {
		return res, rest.NewInvalid("otp login is not configured")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	invalid := rest.NewUnauthorized("invalid or expired code")
	user, err := uc.repoSQL.UserRepo().FindByEmail(ctx, realm.ID, email)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && user.IsServiceAccount) {
		return res, invalid
	}
	if err != nil {
		return res, err
	}
	if err = activeUser(user); err != nil {
		return res, err
	}
	verified, err := notifier.VerifyOTP(ctx, otpVerifyRequest(email, realm.Name, req.Code))
	if err != nil {
		return res, err
	}
	if !verified {
		return res, invalid
	}
	return uc.loginSucceeded(ctx, realm, client, user, meta)
}
