package usecase

import (
	"context"
	"errors"
	"testing"

	"monorepo/services/activity/internal/modules/activity/domain"
	mockrepo "monorepo/services/activity/pkg/mocks/modules/activity/repository"
	mocksharedrepo "monorepo/services/activity/pkg/mocks/shared/repository"

	shareddomain "monorepo/services/activity/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_activityUsecaseImpl_GetAllActivityLogs(t *testing.T) {
	t.Run("Testcase #1: Positive", func(t *testing.T) {
		activityRepo := &mockrepo.ActivityRepository{}
		activityRepo.On("FetchAll", mock.Anything, mock.Anything).Return([]shareddomain.Activity{{}}, nil)
		activityRepo.On("Count", mock.Anything, mock.Anything).Return(1)

		repoMongo := &mocksharedrepo.RepoMongo{}
		repoMongo.On("ActivityRepo").Return(activityRepo)

		uc := activityUsecaseImpl{repoMongo: repoMongo}

		result, err := uc.GetAllActivityLogs(context.Background(), &domain.FilterActivity{})
		assert.NoError(t, err)
		assert.Len(t, result.Data, 1)
	})

	t.Run("Testcase #2: Negative repo error", func(t *testing.T) {
		activityRepo := &mockrepo.ActivityRepository{}
		activityRepo.On("FetchAll", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))

		repoMongo := &mocksharedrepo.RepoMongo{}
		repoMongo.On("ActivityRepo").Return(activityRepo)

		uc := activityUsecaseImpl{repoMongo: repoMongo}

		_, err := uc.GetAllActivityLogs(context.Background(), &domain.FilterActivity{})
		assert.Error(t, err)
	})
}
