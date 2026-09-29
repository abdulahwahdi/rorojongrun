package usecase

import (
	"context"
	"errors"
	"testing"

	mockrepo "monorepo/services/activity/pkg/mocks/modules/activity/repository"
	mocksharedrepo "monorepo/services/activity/pkg/mocks/shared/repository"

	shareddomain "monorepo/services/activity/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_activityUsecaseImpl_GetActivityLogByID(t *testing.T) {
	t.Run("Testcase #1: Positive", func(t *testing.T) {
		activityRepo := &mockrepo.ActivityRepository{}
		activityRepo.On("Find", mock.Anything, mock.Anything).Return(shareddomain.Activity{}, nil)

		repoMongo := &mocksharedrepo.RepoMongo{}
		repoMongo.On("ActivityRepo").Return(activityRepo)

		uc := activityUsecaseImpl{repoMongo: repoMongo}

		_, err := uc.GetActivityLogByID(context.Background(), "some-id")
		assert.NoError(t, err)
	})

	t.Run("Testcase #2: Negative repo error", func(t *testing.T) {
		activityRepo := &mockrepo.ActivityRepository{}
		activityRepo.On("Find", mock.Anything, mock.Anything).Return(shareddomain.Activity{}, errors.New("not found"))

		repoMongo := &mocksharedrepo.RepoMongo{}
		repoMongo.On("ActivityRepo").Return(activityRepo)

		uc := activityUsecaseImpl{repoMongo: repoMongo}

		_, err := uc.GetActivityLogByID(context.Background(), "some-id")
		assert.Error(t, err)
	})
}
