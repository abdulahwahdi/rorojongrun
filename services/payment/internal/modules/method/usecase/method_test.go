package usecase

import (
	"context"
	"testing"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/method/domain"
	mockgatewayrepo "monorepo/services/payment/pkg/mocks/modules/gateway/repository"
	mockmethodrepo "monorepo/services/payment/pkg/mocks/modules/method/repository"
	mocksharedrepo "monorepo/services/payment/pkg/mocks/shared/repository"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setup() (*methodUsecaseImpl, *mockmethodrepo.MethodRepository, *mockgatewayrepo.GatewayRepository) {
	methods, gateways := &mockmethodrepo.MethodRepository{}, &mockgatewayrepo.GatewayRepository{}
	repoSQL := &mocksharedrepo.RepoSQL{}
	repoSQL.On("MethodRepo").Return(methods)
	repoSQL.On("GatewayRepo").Return(gateways)
	return &methodUsecaseImpl{repoSQL: repoSQL}, methods, gateways
}

func Test_CreateMethod(t *testing.T) {
	ctx := context.Background()

	t.Run("gateway method", func(t *testing.T) {
		uc, methods, gateways := setup()
		gateways.On("FindByCode", mock.Anything, "midtrans").Return(shareddomain.Gateway{Code: "midtrans"}, nil)
		var saved shareddomain.Method
		methods.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Method) }).Return(nil)

		_, err := uc.CreateMethod(ctx, &domain.RequestSaveMethod{
			Code: "bca_va", Name: "BCA VA", Type: "virtual_account", GatewayCode: "midtrans", GatewayChannel: "bca",
			IsEnabled: true, FeeFlat: 4000, MinAmount: 10000,
		})
		require.NoError(t, err)
		assert.Equal(t, "midtrans", *saved.GatewayCode)
		assert.EqualValues(t, 4000, saved.FeeFlat)
	})

	t.Run("cash method has no gateway", func(t *testing.T) {
		uc, methods, _ := setup()
		var saved shareddomain.Method
		methods.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Method) }).Return(nil)
		_, err := uc.CreateMethod(ctx, &domain.RequestSaveMethod{Code: "cash", Name: "Cash", Type: "cash", IsEnabled: true})
		require.NoError(t, err)
		assert.Nil(t, saved.GatewayCode, "stored as NULL, not an empty string")
	})

	invalid := map[string]domain.RequestSaveMethod{
		"bad code":            {Code: "BCA VA", Name: "x", Type: "cash"},
		"unknown type":        {Code: "abc", Name: "x", Type: "crypto"},
		"cash with gateway":   {Code: "cash2", Name: "x", Type: "cash", GatewayCode: "midtrans"},
		"gateway without one": {Code: "abc", Name: "x", Type: "qris"},
		"negative fee":        {Code: "abc", Name: "x", Type: "cash", FeeFlat: -1},
		"percent over 100":    {Code: "abc", Name: "x", Type: "cash", FeePercent: 101},
		"max below min":       {Code: "abc", Name: "x", Type: "cash", MinAmount: 10, MaxAmount: 5},
	}
	for name, req := range invalid {
		t.Run("invalid: "+name, func(t *testing.T) {
			uc, methods, _ := setup()
			r := req
			_, err := uc.CreateMethod(ctx, &r)
			assert.Equal(t, 400, rest.HTTPStatus(err))
			methods.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
		})
	}

	t.Run("invalid: unknown gateway", func(t *testing.T) {
		uc, _, gateways := setup()
		gateways.On("FindByCode", mock.Anything, "ghost").Return(shareddomain.Gateway{}, gorm.ErrRecordNotFound)
		_, err := uc.CreateMethod(ctx, &domain.RequestSaveMethod{Code: "abc", Name: "x", Type: "qris", GatewayCode: "ghost", GatewayChannel: "q"})
		assert.Equal(t, 400, rest.HTTPStatus(err))
	})
}

func Test_MethodStatusAndDelete(t *testing.T) {
	ctx := context.Background()
	uc, methods, _ := setup()
	methods.On("FindByID", mock.Anything, 1).Return(shareddomain.Method{ID: 1, Code: "cash", IsEnabled: true}, nil)
	methods.On("FindByID", mock.Anything, 404).Return(shareddomain.Method{}, gorm.ErrRecordNotFound)
	var saved shareddomain.Method
	methods.On("Save", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = *a.Get(1).(*shareddomain.Method) }).Return(nil)
	methods.On("Delete", mock.Anything, 1).Return(nil)

	res, err := uc.SetMethodStatus(ctx, 1, false)
	require.NoError(t, err)
	assert.False(t, res.IsEnabled)
	assert.False(t, saved.IsEnabled)

	_, err = uc.SetMethodStatus(ctx, 404, true)
	assert.Equal(t, 404, rest.HTTPStatus(err))
	assert.NoError(t, uc.DeleteMethod(ctx, 1))
	assert.Equal(t, 404, rest.HTTPStatus(uc.DeleteMethod(ctx, 404)))
}
