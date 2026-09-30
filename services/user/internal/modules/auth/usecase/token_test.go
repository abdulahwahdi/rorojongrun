package usecase

import (
	"context"
	"errors"
	"testing"

	"monorepo/globalshared/auth"
	"monorepo/globalshared/rest"
	mocknotification "monorepo/sdk/mocks/notification"
	"monorepo/sdk/notification"
	"monorepo/services/user/internal/modules/auth/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golangid/candi/candishared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func statusOf(err error) int { return rest.HTTPStatus(err) }

func passwordReq(pw string) *domain.RequestToken {
	return &domain.RequestToken{GrantType: domain.GrantPassword, ClientID: "web", Username: "Alice", Password: pw}
}

func Test_Token_passwordGrant(t *testing.T) {
	t.Run("success issues a verifiable RS256 token bound to the session family", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("password,refresh_token")
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(h.user("secret-pass"), nil)
		h.users.On("Save", mock.Anything, mock.MatchedBy(func(u *shareddomain.User) bool {
			return u.FailedAttempts == 0 && u.LockedUntil == nil && u.LastLoginAt != nil
		})).Return(nil)
		h.sessions.On("Save", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			s := args.Get(1).(*shareddomain.Session)
			s.ID, s.FamilyID = 42, 42
		}).Return(nil)

		res, err := h.uc.Token(context.Background(), "acme", passwordReq("secret-pass"), domain.ClientMeta{IP: "1.2.3.4"})
		assert.NoError(t, err)
		assert.NotEmpty(t, res.RefreshToken)
		assert.Equal(t, 900, res.ExpiresIn)

		var claim candishared.TokenClaim
		_, err = jwt.ParseWithClaims(res.AccessToken, &claim, func(*jwt.Token) (any, error) { return &h.priv.PublicKey, nil },
			jwt.WithValidMethods([]string{"RS256"}))
		assert.NoError(t, err)
		assert.Equal(t, "http://issuer.test/realms/acme", claim.Issuer)
		assert.Equal(t, "5", claim.Subject)
		assert.Equal(t, "acme", auth.RealmFromClaim(&claim))
		assert.Equal(t, 42, float2int(claim.Additional.(map[string]any)["sid"]))
		assert.Equal(t, "operator", claim.Role)
		// only the hash of the refresh token is stored
		saved := h.sessions.Calls[0].Arguments.Get(1).(*shareddomain.Session)
		assert.NotEqual(t, res.RefreshToken, saved.RefreshHash)
		assert.Equal(t, helper.SHA256Hex(res.RefreshToken), saved.RefreshHash)
	})

	t.Run("unknown user is a plain 401", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("password")
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(shareddomain.User{}, errNotFound)
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("x"), domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
		h.users.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("wrong password counts a failure and locks at the threshold", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("password")
		u := h.user("secret-pass")
		u.FailedAttempts = 2 // realm allows 3
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(u, nil)
		h.users.On("Save", mock.Anything, mock.MatchedBy(func(u *shareddomain.User) bool {
			return u.LockedUntil != nil && u.FailedAttempts == 0
		})).Return(nil)
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("wrong"), domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
		h.users.AssertNumberOfCalls(t, "Save", 1)
	})

	t.Run("locked account is 423 and the password is not evaluated", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("password")
		u := h.user("secret-pass")
		u.LockedUntil = future()
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(u, nil)
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("secret-pass"), domain.ClientMeta{})
		assert.Equal(t, 423, statusOf(err))
		h.users.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("an expired lock no longer blocks", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("password")
		u := h.user("secret-pass")
		u.LockedUntil = past()
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(u, nil)
		h.users.On("Save", mock.Anything, mock.Anything).Return(nil)
		h.sessions.On("Save", mock.Anything, mock.Anything).Return(nil)
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("secret-pass"), domain.ClientMeta{})
		assert.NoError(t, err)
	})

	t.Run("disabled account is 403 after a correct password", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("password")
		u := h.user("secret-pass")
		u.Status = shareddomain.UserStatusDisabled
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(u, nil)
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("secret-pass"), domain.ClientMeta{})
		assert.Equal(t, 403, statusOf(err))
	})

	t.Run("service accounts cannot use the password grant", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("password")
		u := h.user("secret-pass")
		u.IsServiceAccount = true
		h.users.On("FindByUsername", mock.Anything, 1, "alice").Return(u, nil)
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("secret-pass"), domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
	})

	t.Run("grant not allowed for the client", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("refresh_token")
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("x"), domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
	})

	t.Run("confidential clients must present their secret", func(t *testing.T) {
		h := newHarness(t)
		hash, _ := helper.HashSecret("s3cret")
		h.clients.On("FindByClientID", mock.Anything, 1, "web").Return(shareddomain.Client{
			ID: 7, ClientID: "web", Type: shareddomain.ClientTypeConfidential, SecretHash: hash, GrantTypes: "password", Enabled: true,
		}, nil)
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("x"), domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
	})

	t.Run("disabled realm", func(t *testing.T) {
		h := newHarness(t)
		h.realms.ExpectedCalls = nil
		r := h.realm
		r.Enabled = false
		h.realms.On("FindByName", mock.Anything, "acme").Return(r, nil)
		_, err := h.uc.Token(context.Background(), "acme", passwordReq("x"), domain.ClientMeta{})
		assert.Equal(t, 403, statusOf(err))
	})
}

func Test_Token_refreshGrant(t *testing.T) {
	refresh := "the-refresh-token"
	live := func(h *harness) shareddomain.Session {
		return shareddomain.Session{ID: 10, RealmID: 1, UserID: 5, ClientID: 7, FamilyID: 3, RefreshHash: helper.SHA256Hex(refresh), ExpiresAt: *future()}
	}
	req := &domain.RequestToken{GrantType: domain.GrantRefreshToken, ClientID: "web", RefreshToken: refresh}

	t.Run("rotates: old token marked used, new one stays in the family", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("refresh_token")
		h.sessions.On("FindByRefreshHash", mock.Anything, helper.SHA256Hex(refresh)).Return(live(h), nil)
		h.users.On("Find", mock.Anything, 1, 5).Return(h.user("x"), nil)
		h.sessions.On("MarkRotated", mock.Anything, 10).Return(true, nil)
		h.sessions.On("Save", mock.Anything, mock.MatchedBy(func(s *shareddomain.Session) bool { return s.FamilyID == 3 })).Return(nil)

		res, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.NoError(t, err)
		assert.NotEmpty(t, res.RefreshToken)
		assert.NotEqual(t, refresh, res.RefreshToken)
	})

	t.Run("reusing a rotated token revokes the whole family", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("refresh_token")
		s := live(h)
		s.RotatedAt = past()
		h.sessions.On("FindByRefreshHash", mock.Anything, mock.Anything).Return(s, nil)
		h.sessions.On("RevokeFamily", mock.Anything, 3).Return(nil)
		_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
		h.sessions.AssertCalled(t, "RevokeFamily", mock.Anything, 3)
	})

	t.Run("losing the rotation race is treated as reuse", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("refresh_token")
		h.sessions.On("FindByRefreshHash", mock.Anything, mock.Anything).Return(live(h), nil)
		h.users.On("Find", mock.Anything, 1, 5).Return(h.user("x"), nil)
		h.sessions.On("MarkRotated", mock.Anything, 10).Return(false, nil)
		h.sessions.On("RevokeFamily", mock.Anything, 3).Return(nil)
		_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
		h.sessions.AssertCalled(t, "RevokeFamily", mock.Anything, 3)
		h.sessions.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("revoked, expired, foreign-client and unknown tokens are 401", func(t *testing.T) {
		mut := map[string]func(*shareddomain.Session){
			"revoked": func(s *shareddomain.Session) { s.RevokedAt = past() },
			"expired": func(s *shareddomain.Session) { s.ExpiresAt = *past() },
			"client":  func(s *shareddomain.Session) { s.ClientID = 99 },
			"realm":   func(s *shareddomain.Session) { s.RealmID = 99 },
		}
		for name, m := range mut {
			h := newHarness(t)
			h.publicClient("refresh_token")
			s := live(h)
			m(&s)
			h.sessions.On("FindByRefreshHash", mock.Anything, mock.Anything).Return(s, nil)
			_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
			assert.Equal(t, 401, statusOf(err), name)
			h.sessions.AssertNotCalled(t, "MarkRotated", mock.Anything, mock.Anything)
		}
		h := newHarness(t)
		h.publicClient("refresh_token")
		h.sessions.On("FindByRefreshHash", mock.Anything, mock.Anything).Return(shareddomain.Session{}, errNotFound)
		_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
	})

	t.Run("a disabled user cannot refresh", func(t *testing.T) {
		h := newHarness(t)
		h.publicClient("refresh_token")
		h.sessions.On("FindByRefreshHash", mock.Anything, mock.Anything).Return(live(h), nil)
		u := h.user("x")
		u.Status = shareddomain.UserStatusDisabled
		h.users.On("Find", mock.Anything, 1, 5).Return(u, nil)
		_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
	})
}

func Test_Token_clientCredentials(t *testing.T) {
	svcID := 9
	hash, _ := helper.HashSecret("s3cret")
	confidential := shareddomain.Client{
		ID: 8, RealmID: 1, ClientID: "billing", Type: shareddomain.ClientTypeConfidential, SecretHash: hash,
		GrantTypes: "client_credentials", Enabled: true, ServiceUserID: &svcID,
	}

	t.Run("success returns a service token without refresh token or session", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("FindByClientID", mock.Anything, 1, "billing").Return(confidential, nil)
		h.users.On("Find", mock.Anything, 1, 9).Return(shareddomain.User{ID: 9, IsServiceAccount: true, Status: shareddomain.UserStatusActive}, nil)
		res, err := h.uc.Token(context.Background(), "acme", &domain.RequestToken{GrantType: domain.GrantClientCredentials, ClientID: "billing", ClientSecret: "s3cret"}, domain.ClientMeta{})
		assert.NoError(t, err)
		assert.Empty(t, res.RefreshToken)
		var claim candishared.TokenClaim
		_, err = jwt.ParseWithClaims(res.AccessToken, &claim, func(*jwt.Token) (any, error) { return &h.priv.PublicKey, nil })
		assert.NoError(t, err)
		assert.Equal(t, auth.TypeService, auth.TypeFromClaim(&claim))
		assert.Equal(t, 0, auth.SessionIDFromClaim(&claim))
		h.sessions.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("wrong secret", func(t *testing.T) {
		h := newHarness(t)
		h.clients.On("FindByClientID", mock.Anything, 1, "billing").Return(confidential, nil)
		_, err := h.uc.Token(context.Background(), "acme", &domain.RequestToken{GrantType: domain.GrantClientCredentials, ClientID: "billing", ClientSecret: "nope"}, domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
	})

	t.Run("disabled client", func(t *testing.T) {
		h := newHarness(t)
		c := confidential
		c.Enabled = false
		h.clients.On("FindByClientID", mock.Anything, 1, "billing").Return(c, nil)
		_, err := h.uc.Token(context.Background(), "acme", &domain.RequestToken{GrantType: domain.GrantClientCredentials, ClientID: "billing", ClientSecret: "s3cret"}, domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
	})
}

func Test_Token_otpGrant(t *testing.T) {
	email := "alice@example.com"
	req := &domain.RequestToken{GrantType: domain.GrantOTP, ClientID: "web", Email: "Alice@Example.com", Code: "123456"}
	withUser := func(h *harness) *mocknotification.Notification {
		n := &mocknotification.Notification{}
		h.uc.notifier = n
		h.publicClient("otp")
		u := h.user("x")
		h.users.On("FindByEmail", mock.Anything, 1, email).Return(u, nil)
		return n
	}

	t.Run("verified code logs in", func(t *testing.T) {
		h := newHarness(t)
		n := withUser(h)
		n.On("VerifyOTP", mock.Anything, notification.VerifyOTPRequest{Recipient: email, Purpose: "login:acme", Code: "123456"}).Return(true, nil)
		h.users.On("Save", mock.Anything, mock.Anything).Return(nil)
		h.sessions.On("Save", mock.Anything, mock.Anything).Return(nil)
		res, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.NoError(t, err)
		assert.NotEmpty(t, res.AccessToken)
	})

	t.Run("wrong code", func(t *testing.T) {
		h := newHarness(t)
		n := withUser(h)
		n.On("VerifyOTP", mock.Anything, mock.Anything).Return(false, nil)
		_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
	})

	t.Run("unknown email never reaches the notification service", func(t *testing.T) {
		h := newHarness(t)
		n := &mocknotification.Notification{}
		h.uc.notifier = n
		h.publicClient("otp")
		h.users.On("FindByEmail", mock.Anything, 1, email).Return(shareddomain.User{}, errNotFound)
		_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.Equal(t, 401, statusOf(err))
		n.AssertNotCalled(t, "VerifyOTP", mock.Anything, mock.Anything)
	})

	t.Run("realm without otp login", func(t *testing.T) {
		h := newHarness(t)
		h.realms.ExpectedCalls = nil
		r := h.realm
		r.OTPLoginEnabled = false
		h.realms.On("FindByName", mock.Anything, "acme").Return(r, nil)
		withUser(h)
		_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.Equal(t, 403, statusOf(err))
	})

	t.Run("notification outage is an error, not a login", func(t *testing.T) {
		h := newHarness(t)
		n := withUser(h)
		n.On("VerifyOTP", mock.Anything, mock.Anything).Return(false, errors.New("boom"))
		_, err := h.uc.Token(context.Background(), "acme", req, domain.ClientMeta{})
		assert.Error(t, err)
		assert.Equal(t, 500, statusOf(err))
	})
}

func Test_RequestOTP(t *testing.T) {
	req := &domain.RequestOTPLogin{ClientID: "web", Email: "Alice@Example.com"}

	t.Run("known account gets a code", func(t *testing.T) {
		h := newHarness(t)
		n := &mocknotification.Notification{}
		h.uc.notifier = n
		h.publicClient("otp")
		h.users.On("FindByEmail", mock.Anything, 1, "alice@example.com").Return(h.user("x"), nil)
		n.On("RequestOTP", mock.Anything, notification.RequestOTPRequest{Recipient: "alice@example.com", Channel: "email", Purpose: "login:acme"}).Return("job", nil)
		assert.NoError(t, h.uc.RequestOTP(context.Background(), "acme", req))
		n.AssertExpectations(t)
	})

	t.Run("unknown account gets the same answer and no mail", func(t *testing.T) {
		h := newHarness(t)
		n := &mocknotification.Notification{}
		h.uc.notifier = n
		h.publicClient("otp")
		h.users.On("FindByEmail", mock.Anything, 1, mock.Anything).Return(shareddomain.User{}, errNotFound)
		assert.NoError(t, h.uc.RequestOTP(context.Background(), "acme", req))
		n.AssertNotCalled(t, "RequestOTP", mock.Anything, mock.Anything)
	})

	t.Run("delivery failures are not leaked", func(t *testing.T) {
		h := newHarness(t)
		n := &mocknotification.Notification{}
		h.uc.notifier = n
		h.publicClient("otp")
		h.users.On("FindByEmail", mock.Anything, 1, mock.Anything).Return(h.user("x"), nil)
		n.On("RequestOTP", mock.Anything, mock.Anything).Return("", errors.New("smtp down"))
		assert.NoError(t, h.uc.RequestOTP(context.Background(), "acme", req))
	})

	t.Run("client without the otp grant is rejected", func(t *testing.T) {
		h := newHarness(t)
		h.uc.notifier = &mocknotification.Notification{}
		h.publicClient("password")
		assert.Equal(t, 401, statusOf(h.uc.RequestOTP(context.Background(), "acme", req)))
	})
}
