package usecase

import (
	"context"
	"errors"
	"testing"

	"monorepo/services/notification/internal/modules/notification/domain"
	mockrepo "monorepo/services/notification/pkg/mocks/modules/notification/repository"
	mocksharedrepo "monorepo/services/notification/pkg/mocks/shared/repository"
	shareddomain "monorepo/services/notification/pkg/shared/domain"

	taskqueueworker "github.com/golangid/candi/codebase/app/task_queue_worker"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_notificationUsecaseImpl_SendNotification(t *testing.T) {
	activeTemplate := shareddomain.NotificationTemplate{
		ID: 1, Code: "otp_verification", Channel: "email",
		BodyTemplate: "code is {{.code}}", IsActive: true,
	}
	enabledConfig := shareddomain.NotificationChannelConfig{ID: 1, Channel: "email", IsEnabled: true}

	t.Run("Positive", func(t *testing.T) {
		repo := &mockrepo.NotificationRepository{}
		repo.On("FindTemplateByCodeChannel", mock.Anything, "otp_verification", "email").Return(activeTemplate, nil)
		repo.On("FindChannelConfig", mock.Anything, "email").Return(enabledConfig, nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("NotificationRepo").Return(repo)

		uc := notificationUsecaseImpl{
			repoSQL: repoSQL,
			jobEnqueuer: func(ctx context.Context, req *taskqueueworker.AddJobRequest) (string, error) {
				assert.Equal(t, domain.TaskNameNotificationDispatch, req.TaskName)
				return "job-id", nil
			},
		}

		jobID, err := uc.SendNotification(context.Background(), &domain.RequestSendNotification{
			Channel: "email", TemplateCode: "otp_verification", Recipient: "a@b.com",
			Variables: map[string]any{"code": "123456"},
		})
		assert.NoError(t, err)
		assert.Equal(t, "job-id", jobID)
	})

	t.Run("Negative template not found", func(t *testing.T) {
		repo := &mockrepo.NotificationRepository{}
		repo.On("FindTemplateByCodeChannel", mock.Anything, mock.Anything, mock.Anything).Return(shareddomain.NotificationTemplate{}, errors.New("not found"))

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("NotificationRepo").Return(repo)

		uc := notificationUsecaseImpl{repoSQL: repoSQL}
		_, err := uc.SendNotification(context.Background(), &domain.RequestSendNotification{Channel: "email", TemplateCode: "missing"})
		assert.Error(t, err)
	})

	t.Run("Negative channel disabled", func(t *testing.T) {
		repo := &mockrepo.NotificationRepository{}
		repo.On("FindTemplateByCodeChannel", mock.Anything, mock.Anything, mock.Anything).Return(activeTemplate, nil)
		repo.On("FindChannelConfig", mock.Anything, mock.Anything).Return(shareddomain.NotificationChannelConfig{IsEnabled: false}, nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("NotificationRepo").Return(repo)

		uc := notificationUsecaseImpl{repoSQL: repoSQL}
		_, err := uc.SendNotification(context.Background(), &domain.RequestSendNotification{Channel: "email", TemplateCode: "otp_verification"})
		assert.Error(t, err)
	})

	t.Run("Negative enqueue error", func(t *testing.T) {
		repo := &mockrepo.NotificationRepository{}
		repo.On("FindTemplateByCodeChannel", mock.Anything, mock.Anything, mock.Anything).Return(activeTemplate, nil)
		repo.On("FindChannelConfig", mock.Anything, mock.Anything).Return(enabledConfig, nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("NotificationRepo").Return(repo)

		uc := notificationUsecaseImpl{
			repoSQL: repoSQL,
			jobEnqueuer: func(ctx context.Context, req *taskqueueworker.AddJobRequest) (string, error) {
				return "", errors.New("enqueue failed")
			},
		}
		_, err := uc.SendNotification(context.Background(), &domain.RequestSendNotification{
			Channel: "email", TemplateCode: "otp_verification", Variables: map[string]any{"code": "1"},
		})
		assert.Error(t, err)
	})
}
