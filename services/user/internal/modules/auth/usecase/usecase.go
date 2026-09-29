package usecase

import (
	"context"
	"sync"

	"monorepo/sdk"
	"monorepo/sdk/notification"
	"monorepo/services/user/internal/modules/auth/domain"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/repository"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// AuthUsecase abstraction — login, sessions, "who am I" and the authorization decision
type AuthUsecase interface {
	// Token is the public login endpoint: password, refresh_token, client_credentials and otp grants
	Token(ctx context.Context, realm string, req *domain.RequestToken, meta domain.ClientMeta) (res domain.ResponseToken, err error)
	// RequestOTP sends a login code; it never reveals whether the account exists
	RequestOTP(ctx context.Context, realm string, req *domain.RequestOTPLogin) (err error)
	// Logout revokes the session of the calling token
	Logout(ctx context.Context, realm string) (err error)

	GetMe(ctx context.Context, realm string) (data domain.ResponseMe, err error)
	GetMyPermissions(ctx context.Context, realm, clientID string) (data domain.ResponseMyPermissions, err error)
	GetMySessions(ctx context.Context, realm string, filter *domain.FilterSession) (data domain.ResponseSessionList, err error)
	RevokeMySession(ctx context.Context, realm string, id int) (err error)

	// admin session management
	GetAllSession(ctx context.Context, realm string, filter *domain.FilterSession) (data domain.ResponseSessionList, err error)
	GetDetailSession(ctx context.Context, realm string, id int) (data domain.ResponseSession, err error)
	RevokeSession(ctx context.Context, realm string, id int) (err error)
	RevokeUserSessions(ctx context.Context, realm string, userID int) (err error)

	// internal authorization API, no caller authentication (gRPC is protected by the internal basic auth)
	CheckPermission(ctx context.Context, req domain.CheckPermissionRequest) (allowed bool, role string, err error)
	GetUserPermissions(ctx context.Context, realm string, userID int, service string) (codes []string, err error)
	GetUserForService(ctx context.Context, realm string, userID int) (user shareddomain.User, roles []string, err error)
}

type authUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
	// notifier can be injected (tests); otherwise the global sdk client is used
	notifier notification.Notification

	keyMu    sync.RWMutex
	keyCache map[string]*cachedKey // by kid
}

// NewAuthUsecase usecase impl constructor
func NewAuthUsecase(deps dependency.Dependency) (AuthUsecase, func(sharedUsecase common.Usecase)) {
	uc := &authUsecaseImpl{
		deps:     deps,
		repoSQL:  repository.GetSharedRepoSQL(),
		keyCache: map[string]*cachedKey{},
	}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}

func (uc *authUsecaseImpl) notification() notification.Notification {
	if uc.notifier != nil {
		return uc.notifier
	}
	if s := sdk.GetSDK(); s != nil {
		return s.Notification()
	}
	return nil
}
