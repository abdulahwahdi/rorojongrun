package usecase

import (
	"context"

	"monorepo/services/payment/internal/modules/topic/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"
	"monorepo/services/payment/pkg/shared/repository"
	"monorepo/services/payment/pkg/shared/usecase/common"

	"github.com/golangid/candi/codebase/factory/dependency"
)

// TopicUsecase abstraction
type TopicUsecase interface {
	GetAllTopics(ctx context.Context, filter *domain.FilterTopic) (data domain.ResponseTopicList, err error)
	GetTopic(ctx context.Context, id int) (data shareddomain.Topic, err error)
	CreateTopic(ctx context.Context, req *domain.RequestSaveTopic) (data shareddomain.Topic, err error)
	UpdateTopic(ctx context.Context, id int, req *domain.RequestSaveTopic) (data shareddomain.Topic, err error)
	SetTopicStatus(ctx context.Context, id int, enabled bool) (data shareddomain.Topic, err error)
	DeleteTopic(ctx context.Context, id int) (err error)
}

type topicUsecaseImpl struct {
	deps          dependency.Dependency
	sharedUsecase common.Usecase
	repoSQL       repository.RepoSQL
}

// NewTopicUsecase usecase impl constructor
func NewTopicUsecase(deps dependency.Dependency) (TopicUsecase, func(sharedUsecase common.Usecase)) {
	uc := &topicUsecaseImpl{deps: deps, repoSQL: repository.GetSharedRepoSQL()}
	return uc, func(sharedUsecase common.Usecase) {
		uc.sharedUsecase = sharedUsecase
	}
}
