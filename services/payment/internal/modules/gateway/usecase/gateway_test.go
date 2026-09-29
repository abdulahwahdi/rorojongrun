package usecase

import (
	"context"
	"errors"
	"testing"

	"monorepo/globalshared/crypto"
	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/gateway/domain"
	mockgatewayrepo "monorepo/services/payment/pkg/mocks/modules/gateway/repository"
	mocksharedrepo "monorepo/services/payment/pkg/mocks/shared/repository"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const testSecret = "test-secret"

func newUC(repo *mockgatewayrepo.GatewayRepository) *gatewayUsecaseImpl {
	repoSQL := &mocksharedrepo.RepoSQL{}
	repoSQL.On("GatewayRepo").Return(repo)
	return &gatewayUsecaseImpl{repoSQL: repoSQL, encryptionSecret: func() string { return testSecret }}
}

func status(err error) int { return rest.HTTPStatus(err) }

func Test_UpdateGateway(t *testing.T) {
	ctx := context.Background()

	t.Run("credentials are stored encrypted and never returned in clear", func(t *testing.T) {
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "midtrans").Return(shareddomain.Gateway{ID: 1, Code: "midtrans", Environment: "sandbox"}, nil)
		var saved shareddomain.Gateway
		repo.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Gateway) }).Return(nil)

		res, err := newUC(repo).UpdateGateway(ctx, "midtrans", &domain.RequestUpdateGateway{
			Environment: "production", Credentials: map[string]string{"serverKey": "Mid-server-key-1234"},
		})
		require.NoError(t, err)

		assert.NotContains(t, saved.CredentialsEnc, "Mid-server-key")
		plain, err := crypto.Decrypt(testSecret, saved.CredentialsEnc)
		require.NoError(t, err)
		assert.JSONEq(t, `{"serverKey":"Mid-server-key-1234"}`, string(plain))
		assert.Equal(t, "production", saved.Environment)

		assert.True(t, res.HasCredentials)
		assert.Equal(t, "****1234", res.Credentials["serverKey"])
	})

	t.Run("credentials are merged, an empty value deletes a key", func(t *testing.T) {
		existing, _ := crypto.Encrypt(testSecret, []byte(`{"secretKey":"xnd_secret_0001","callbackToken":"cb-token-0002"}`))
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "xendit").Return(shareddomain.Gateway{ID: 2, Code: "xendit", CredentialsEnc: existing}, nil)
		var saved shareddomain.Gateway
		repo.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Gateway) }).Return(nil)

		_, err := newUC(repo).UpdateGateway(ctx, "xendit", &domain.RequestUpdateGateway{Credentials: map[string]string{"callbackToken": "", "secretKey": "xnd_secret_9999"}})
		require.NoError(t, err)
		plain, _ := crypto.Decrypt(testSecret, saved.CredentialsEnc)
		assert.JSONEq(t, `{"secretKey":"xnd_secret_9999"}`, string(plain))
	})

	t.Run("unknown credential keys are rejected", func(t *testing.T) {
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "midtrans").Return(shareddomain.Gateway{Code: "midtrans"}, nil)
		_, err := newUC(repo).UpdateGateway(ctx, "midtrans", &domain.RequestUpdateGateway{Credentials: map[string]string{"serverkey": "typo"}})
		assert.Equal(t, 400, status(err))
		repo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("an enabled gateway cannot lose a required credential", func(t *testing.T) {
		existing, _ := crypto.Encrypt(testSecret, []byte(`{"serverKey":"Mid-server-key-1234"}`))
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "midtrans").Return(shareddomain.Gateway{Code: "midtrans", IsEnabled: true, CredentialsEnc: existing}, nil)
		_, err := newUC(repo).UpdateGateway(ctx, "midtrans", &domain.RequestUpdateGateway{Credentials: map[string]string{"serverKey": ""}})
		assert.Equal(t, 400, status(err))
	})

	t.Run("the mock gateway cannot be moved to production", func(t *testing.T) {
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "mock").Return(shareddomain.Gateway{Code: "mock"}, nil)
		_, err := newUC(repo).UpdateGateway(ctx, "mock", &domain.RequestUpdateGateway{Environment: "production"})
		assert.Equal(t, 400, status(err))
	})

	t.Run("no encryption secret configured", func(t *testing.T) {
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "midtrans").Return(shareddomain.Gateway{Code: "midtrans"}, nil)
		uc := newUC(repo)
		uc.encryptionSecret = func() string { return "" }
		_, err := uc.UpdateGateway(ctx, "midtrans", &domain.RequestUpdateGateway{Credentials: map[string]string{"serverKey": "k"}})
		assert.Equal(t, 400, status(err))
	})

	t.Run("unknown gateway", func(t *testing.T) {
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "nope").Return(shareddomain.Gateway{}, gorm.ErrRecordNotFound)
		_, err := newUC(repo).UpdateGateway(ctx, "nope", &domain.RequestUpdateGateway{})
		assert.Equal(t, 404, status(err))
	})
}

func Test_SetGatewayStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("cannot enable without credentials", func(t *testing.T) {
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "xendit").Return(shareddomain.Gateway{Code: "xendit"}, nil)
		_, err := newUC(repo).SetGatewayStatus(ctx, "xendit", true)
		assert.Equal(t, 400, status(err))
		repo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("enable and disable, mock needs no credentials", func(t *testing.T) {
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "mock").Return(shareddomain.Gateway{Code: "mock"}, nil)
		var saved shareddomain.Gateway
		repo.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Gateway) }).Return(nil)

		res, err := newUC(repo).SetGatewayStatus(ctx, "mock", true)
		require.NoError(t, err)
		assert.True(t, saved.IsEnabled)
		assert.True(t, res.IsEnabled)

		_, err = newUC(repo).SetGatewayStatus(ctx, "mock", false)
		require.NoError(t, err)
		assert.False(t, saved.IsEnabled)
	})

	t.Run("a save failure is returned", func(t *testing.T) {
		repo := &mockgatewayrepo.GatewayRepository{}
		repo.On("FindByCode", mock.Anything, "mock").Return(shareddomain.Gateway{Code: "mock"}, nil)
		repo.On("Save", mock.Anything, mock.Anything).Return(errors.New("db down"))
		_, err := newUC(repo).SetGatewayStatus(ctx, "mock", true)
		assert.Error(t, err)
	})
}

func Test_GetGateway_MasksUnreadableCredentials(t *testing.T) {
	// credentials encrypted with another secret (GATEWAY_ENCRYPTION_SECRET was changed) stay visible as a gateway
	enc, _ := crypto.Encrypt("old-secret", []byte(`{"serverKey":"x"}`))
	repo := &mockgatewayrepo.GatewayRepository{}
	repo.On("FindByCode", mock.Anything, "midtrans").Return(shareddomain.Gateway{Code: "midtrans", CredentialsEnc: enc}, nil)

	res, err := newUC(repo).GetGateway(context.Background(), "midtrans")
	require.NoError(t, err)
	assert.False(t, res.HasCredentials, "unreadable credentials are reported as missing so the operator re-enters them")
}

func Test_Mask(t *testing.T) {
	assert.Equal(t, "****", domain.Mask("short"))
	assert.Equal(t, "****abcd", domain.Mask("0123456789abcd"))
}
