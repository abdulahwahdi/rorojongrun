package usecase

import (
	"context"
	"errors"
	"testing"

	"monorepo/services/notification/internal/modules/otp/domain"
	mockrepo "monorepo/services/notification/pkg/mocks/modules/otp/repository"
	mocksharedcommon "monorepo/services/notification/pkg/mocks/shared/usecase/common"
	mocksharedrepo "monorepo/services/notification/pkg/mocks/shared/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_otpUsecaseImpl_RequestOTP(t *testing.T) {
	t.Run("Positive", func(t *testing.T) {
		repo := &mockrepo.OtpRepository{}
		repo.On("Save", mock.Anything, mock.Anything).Return(nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("OtpRepo").Return(repo)

		shared := &mocksharedcommon.Usecase{}
		shared.On("SendNotification", mock.Anything, mock.Anything).Return("job-id", nil)

		uc := otpUsecaseImpl{repoSQL: repoSQL, sharedUsecase: shared}
		jobID, err := uc.RequestOTP(context.Background(), &domain.RequestOTP{
			Recipient: "a@b.com", Channel: "email", Purpose: "login",
		})
		assert.NoError(t, err)
		assert.Equal(t, "job-id", jobID)
	})

	t.Run("Negative repo save error", func(t *testing.T) {
		repo := &mockrepo.OtpRepository{}
		repo.On("Save", mock.Anything, mock.Anything).Return(errors.New("db error"))

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("OtpRepo").Return(repo)

		uc := otpUsecaseImpl{repoSQL: repoSQL}
		_, err := uc.RequestOTP(context.Background(), &domain.RequestOTP{Recipient: "a@b.com", Channel: "email", Purpose: "login"})
		assert.Error(t, err)
	})

	t.Run("Negative send error", func(t *testing.T) {
		repo := &mockrepo.OtpRepository{}
		repo.On("Save", mock.Anything, mock.Anything).Return(nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("OtpRepo").Return(repo)

		shared := &mocksharedcommon.Usecase{}
		shared.On("SendNotification", mock.Anything, mock.Anything).Return("", errors.New("send failed"))

		uc := otpUsecaseImpl{repoSQL: repoSQL, sharedUsecase: shared}
		_, err := uc.RequestOTP(context.Background(), &domain.RequestOTP{Recipient: "a@b.com", Channel: "email", Purpose: "login"})
		assert.Error(t, err)
	})
}
