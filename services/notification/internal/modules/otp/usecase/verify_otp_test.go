package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"monorepo/services/notification/internal/modules/otp/domain"
	mockrepo "monorepo/services/notification/pkg/mocks/modules/otp/repository"
	mocksharedrepo "monorepo/services/notification/pkg/mocks/shared/repository"
	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

func Test_otpUsecaseImpl_VerifyOTP(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("123456"), 10)

	t.Run("Positive", func(t *testing.T) {
		repo := &mockrepo.OtpRepository{}
		repo.On("FindLatestUnverified", mock.Anything, "a@b.com", "login").Return(shareddomain.OTPRequest{
			ID: 1, CodeHash: string(hash), ExpiresAt: time.Now().Add(time.Minute), MaxAttempts: 5, AttemptCount: 0,
		}, nil)
		repo.On("MarkVerified", mock.Anything, 1).Return(nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("OtpRepo").Return(repo)

		uc := otpUsecaseImpl{repoSQL: repoSQL}
		verified, err := uc.VerifyOTP(context.Background(), &domain.RequestVerifyOTP{Recipient: "a@b.com", Purpose: "login", Code: "123456"})
		assert.NoError(t, err)
		assert.True(t, verified)
	})

	t.Run("Negative not found", func(t *testing.T) {
		repo := &mockrepo.OtpRepository{}
		repo.On("FindLatestUnverified", mock.Anything, mock.Anything, mock.Anything).Return(shareddomain.OTPRequest{}, errors.New("not found"))

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("OtpRepo").Return(repo)

		uc := otpUsecaseImpl{repoSQL: repoSQL}
		_, err := uc.VerifyOTP(context.Background(), &domain.RequestVerifyOTP{Recipient: "a@b.com", Purpose: "login", Code: "123456"})
		assert.ErrorIs(t, err, domain.ErrOTPNotFound)
	})

	t.Run("Negative expired", func(t *testing.T) {
		repo := &mockrepo.OtpRepository{}
		repo.On("FindLatestUnverified", mock.Anything, mock.Anything, mock.Anything).Return(shareddomain.OTPRequest{
			ID: 1, CodeHash: string(hash), ExpiresAt: time.Now().Add(-time.Minute), MaxAttempts: 5, AttemptCount: 0,
		}, nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("OtpRepo").Return(repo)

		uc := otpUsecaseImpl{repoSQL: repoSQL}
		_, err := uc.VerifyOTP(context.Background(), &domain.RequestVerifyOTP{Recipient: "a@b.com", Purpose: "login", Code: "123456"})
		assert.ErrorIs(t, err, domain.ErrOTPExpired)
	})

	t.Run("Negative too many attempts", func(t *testing.T) {
		repo := &mockrepo.OtpRepository{}
		repo.On("FindLatestUnverified", mock.Anything, mock.Anything, mock.Anything).Return(shareddomain.OTPRequest{
			ID: 1, CodeHash: string(hash), ExpiresAt: time.Now().Add(time.Minute), MaxAttempts: 5, AttemptCount: 5,
		}, nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("OtpRepo").Return(repo)

		uc := otpUsecaseImpl{repoSQL: repoSQL}
		_, err := uc.VerifyOTP(context.Background(), &domain.RequestVerifyOTP{Recipient: "a@b.com", Purpose: "login", Code: "123456"})
		assert.ErrorIs(t, err, domain.ErrOTPTooManyAttempts)
	})

	t.Run("Negative mismatch", func(t *testing.T) {
		repo := &mockrepo.OtpRepository{}
		repo.On("FindLatestUnverified", mock.Anything, mock.Anything, mock.Anything).Return(shareddomain.OTPRequest{
			ID: 1, CodeHash: string(hash), ExpiresAt: time.Now().Add(time.Minute), MaxAttempts: 5, AttemptCount: 0,
		}, nil)
		repo.On("IncrementAttempt", mock.Anything, 1).Return(nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("OtpRepo").Return(repo)

		uc := otpUsecaseImpl{repoSQL: repoSQL}
		_, err := uc.VerifyOTP(context.Background(), &domain.RequestVerifyOTP{Recipient: "a@b.com", Purpose: "login", Code: "000000"})
		assert.ErrorIs(t, err, domain.ErrOTPMismatch)
	})
}
