package usecase

import (
	"context"
	"errors"
	"testing"

	"monorepo/services/activity/internal/modules/activity/domain"
	mockrepo "monorepo/services/activity/pkg/mocks/modules/activity/repository"
	mocksharedrepo "monorepo/services/activity/pkg/mocks/shared/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_activityUsecaseImpl_PersistActivityLog(t *testing.T) {
	t.Run("Testcase #1: Positive", func(t *testing.T) {
		activityRepo := &mockrepo.ActivityRepository{}
		activityRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

		repoMongo := &mocksharedrepo.RepoMongo{}
		repoMongo.On("ActivityRepo").Return(activityRepo)

		uc := activityUsecaseImpl{repoMongo: repoMongo}

		err := uc.PersistActivityLog(context.Background(), &domain.RequestSaveActivity{
			ServiceName: "order", EventType: "order.created", ReferenceID: "ord-123",
		})
		assert.NoError(t, err)
	})

	t.Run("Testcase #2: Negative repo error", func(t *testing.T) {
		activityRepo := &mockrepo.ActivityRepository{}
		activityRepo.On("Save", mock.Anything, mock.Anything).Return(errors.New("db error"))

		repoMongo := &mocksharedrepo.RepoMongo{}
		repoMongo.On("ActivityRepo").Return(activityRepo)

		uc := activityUsecaseImpl{repoMongo: repoMongo}

		err := uc.PersistActivityLog(context.Background(), &domain.RequestSaveActivity{
			ServiceName: "order", EventType: "order.created", ReferenceID: "ord-123",
		})
		assert.Error(t, err)
	})
}
