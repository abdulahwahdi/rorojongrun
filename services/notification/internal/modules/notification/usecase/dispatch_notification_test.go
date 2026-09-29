package usecase

import (
	"context"
	"errors"
	"testing"

	"monorepo/services/notification/internal/modules/notification/channel"
	"monorepo/services/notification/internal/modules/notification/domain"
	mockchannel "monorepo/services/notification/pkg/mocks/modules/notification/channel"
	mockrepo "monorepo/services/notification/pkg/mocks/modules/notification/repository"
	mocksharedrepo "monorepo/services/notification/pkg/mocks/shared/repository"
	shareddomain "monorepo/services/notification/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_notificationUsecaseImpl_DispatchNotification(t *testing.T) {
	payload := &domain.DispatchPayload{
		Channel: "email", TemplateCode: "otp_verification", Recipient: "a@b.com",
		RenderedSubject: "subj", RenderedBody: "body",
	}

	t.Run("Positive", func(t *testing.T) {
		repo := &mockrepo.NotificationRepository{}
		repo.On("SaveLog", mock.Anything, mock.MatchedBy(func(l *shareddomain.NotificationLog) bool {
			l.ID = 1 // simulate DB assigning an ID
			return true
		})).Return(nil)
		repo.On("UpdateLogStatus", mock.Anything, 1, domain.StatusSent, mock.Anything, mock.Anything).Return(nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("NotificationRepo").Return(repo)

		sender := &mockchannel.Sender{}
		sender.On("Send", mock.Anything, "a@b.com", "subj", "body").Return(nil)

		uc := notificationUsecaseImpl{repoSQL: repoSQL, senders: map[string]channel.Sender{"email": sender}}
		err := uc.DispatchNotification(context.Background(), payload)
		assert.NoError(t, err)
	})

	t.Run("Negative send error", func(t *testing.T) {
		repo := &mockrepo.NotificationRepository{}
		repo.On("SaveLog", mock.Anything, mock.Anything).Return(nil)
		repo.On("UpdateLogStatus", mock.Anything, mock.Anything, domain.StatusFailed, mock.Anything, mock.Anything).Return(nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("NotificationRepo").Return(repo)

		sender := &mockchannel.Sender{}
		sender.On("Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("smtp down"))

		uc := notificationUsecaseImpl{repoSQL: repoSQL, senders: map[string]channel.Sender{"email": sender}}
		err := uc.DispatchNotification(context.Background(), payload)
		assert.Error(t, err)
	})

	t.Run("Negative no sender for channel", func(t *testing.T) {
		repo := &mockrepo.NotificationRepository{}
		repo.On("SaveLog", mock.Anything, mock.Anything).Return(nil)
		repo.On("UpdateLogStatus", mock.Anything, mock.Anything, domain.StatusFailed, mock.Anything, mock.Anything).Return(nil)

		repoSQL := &mocksharedrepo.RepoSQL{}
		repoSQL.On("NotificationRepo").Return(repo)

		uc := notificationUsecaseImpl{repoSQL: repoSQL, senders: map[string]channel.Sender{}}
		err := uc.DispatchNotification(context.Background(), payload)
		assert.NoError(t, err) // non-retriable: logged as failed, not returned as retriable error
	})
}
