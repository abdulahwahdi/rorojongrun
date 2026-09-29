package usecase

import (
	"context"
	"errors"
	"strings"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/client/domain"
	"monorepo/services/user/pkg/helper"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
	"gorm.io/gorm"
)

var allowedGrants = map[string]bool{"password": true, "refresh_token": true, "client_credentials": true, "otp": true}

// normalizeGrants validates the grant list against the client type and joins it for storage
func normalizeGrants(clientType string, grants []string) (string, error) {
	if len(grants) == 0 {
		if clientType == shareddomain.ClientTypeConfidential {
			grants = []string{"client_credentials"}
		} else {
			grants = []string{"password", "refresh_token"}
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(grants))
	for _, g := range grants {
		if !allowedGrants[g] {
			return "", rest.NewInvalid("unsupported grant type " + g)
		}
		if g == "client_credentials" && clientType != shareddomain.ClientTypeConfidential {
			return "", rest.NewInvalid("client_credentials is only allowed for confidential clients")
		}
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	return strings.Join(out, ","), nil
}

func (uc *clientUsecaseImpl) GetAllClient(ctx context.Context, realm string, filter *domain.FilterClient) (data domain.ResponseClientList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientUsecase:GetAllClient")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	clients, err := uc.repoSQL.ClientRepo().FetchAll(ctx, r.ID, filter)
	if err != nil {
		return data, err
	}
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.ClientRepo().Count(ctx, r.ID, filter))
	data.Data = make([]domain.ResponseClient, 0, len(clients))
	for i := range clients {
		var c domain.ResponseClient
		c.Serialize(&clients[i])
		data.Data = append(data.Data, c)
	}
	return
}

func (uc *clientUsecaseImpl) GetDetailClient(ctx context.Context, realm string, id int) (data domain.ResponseClient, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientUsecase:GetDetailClient")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	client, err := uc.repoSQL.ClientRepo().Find(ctx, r.ID, id)
	if err != nil {
		return data, common.NotFound(err, "client")
	}
	data.Serialize(&client)
	return
}

func (uc *clientUsecaseImpl) CreateClient(ctx context.Context, realm string, req *domain.RequestCreateClient) (res domain.ResponseClient, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientUsecase:CreateClient")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return res, err
	}
	if req.Type == "" {
		req.Type = shareddomain.ClientTypePublic
	}
	if req.Type != shareddomain.ClientTypePublic && req.Type != shareddomain.ClientTypeConfidential {
		return res, rest.NewInvalid("type must be public or confidential")
	}
	grants, err := normalizeGrants(req.Type, req.GrantTypes)
	if err != nil {
		return res, err
	}
	if _, err = uc.repoSQL.ClientRepo().FindByClientID(ctx, r.ID, req.ClientID); err == nil {
		return res, rest.NewConflict("client " + req.ClientID + " already exists in realm " + realm)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return res, err
	}

	client := shareddomain.Client{
		RealmID: r.ID, ClientID: req.ClientID, Name: req.Name, Description: req.Description,
		Type: req.Type, GrantTypes: grants, Enabled: req.Enabled == nil || *req.Enabled,
	}
	var secret string
	err = uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if req.Type == shareddomain.ClientTypeConfidential {
			secret = helper.RandomToken(32)
			hash, err := helper.HashSecret(secret)
			if err != nil {
				return err
			}
			client.SecretHash = hash
			// a confidential client acts as a machine user, so roles are assigned like for any user
			svc := shareddomain.User{
				RealmID: r.ID, Username: "svc-" + req.ClientID, FullName: "Service account of " + req.ClientID,
				Status: shareddomain.UserStatusActive, IsServiceAccount: true,
			}
			if err := uc.repoSQL.UserRepo().Save(ctx, &svc); err != nil {
				return err
			}
			client.ServiceUserID = &svc.ID
		}
		return uc.repoSQL.ClientRepo().Save(ctx, &client)
	})
	if err != nil {
		return res, err
	}
	res.Serialize(&client)
	res.ClientSecret = secret
	return
}

func (uc *clientUsecaseImpl) UpdateClient(ctx context.Context, realm string, id int, req *domain.RequestUpdateClient) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientUsecase:UpdateClient")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	client, err := uc.repoSQL.ClientRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "client")
	}
	if len(req.GrantTypes) == 0 {
		req.GrantTypes = strings.Split(client.GrantTypes, ",")
	}
	grants, err := normalizeGrants(client.Type, req.GrantTypes)
	if err != nil {
		return err
	}
	client.Name, client.Description, client.GrantTypes = req.Name, req.Description, grants
	disabling := client.Enabled && req.Enabled != nil && !*req.Enabled
	if req.Enabled != nil {
		client.Enabled = *req.Enabled
	}
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.ClientRepo().Save(ctx, &client); err != nil {
			return err
		}
		if disabling {
			return uc.repoSQL.SessionRepo().RevokeByClient(ctx, client.ID)
		}
		return nil
	})
}

func (uc *clientUsecaseImpl) DeleteClient(ctx context.Context, realm string, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientUsecase:DeleteClient")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	client, err := uc.repoSQL.ClientRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "client")
	}
	return uc.repoSQL.WithTransaction(ctx, func(ctx context.Context) error {
		if err := uc.repoSQL.SessionRepo().RevokeByClient(ctx, client.ID); err != nil {
			return err
		}
		if err := uc.repoSQL.MenuRepo().DeleteByClient(ctx, client.ID); err != nil {
			return err
		}
		if client.ServiceUserID != nil {
			if err := uc.repoSQL.UserRepo().Delete(ctx, *client.ServiceUserID); err != nil {
				return err
			}
		}
		return uc.repoSQL.ClientRepo().Delete(ctx, client.ID)
	})
}

func (uc *clientUsecaseImpl) RotateClientSecret(ctx context.Context, realm string, id int) (res domain.ResponseClient, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "ClientUsecase:RotateClientSecret")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return res, err
	}
	client, err := uc.repoSQL.ClientRepo().Find(ctx, r.ID, id)
	if err != nil {
		return res, common.NotFound(err, "client")
	}
	if client.Type != shareddomain.ClientTypeConfidential {
		return res, rest.NewInvalid("public clients have no secret")
	}
	secret := helper.RandomToken(32)
	if client.SecretHash, err = helper.HashSecret(secret); err != nil {
		return res, err
	}
	if err = uc.repoSQL.ClientRepo().Save(ctx, &client); err != nil {
		return res, err
	}
	res.Serialize(&client)
	res.ClientSecret = secret
	return
}
