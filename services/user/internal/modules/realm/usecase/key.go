package usecase

import (
	"context"

	"monorepo/globalshared/auth"
	"monorepo/globalshared/crypto"
	"monorepo/services/user/internal/modules/realm/domain"
	"monorepo/services/user/pkg/helper"
	"monorepo/services/user/pkg/shared"
	shareddomain "monorepo/services/user/pkg/shared/domain"
	"monorepo/services/user/pkg/shared/usecase/common"

	"github.com/golangid/candi/candishared"
	"github.com/golangid/candi/tracer"
)

// newSigningKey generates and stores a new active RSA key for the realm
func (uc *realmUsecaseImpl) newSigningKey(ctx context.Context, realmID int) (key shareddomain.RealmKey, err error) {
	priv, err := helper.GenerateRSAKey()
	if err != nil {
		return key, err
	}
	privPEM, err := helper.PrivateKeyToPEM(priv)
	if err != nil {
		return key, err
	}
	enc, err := crypto.Encrypt(shared.GetEnv().KeyEncryptionSecret, privPEM)
	if err != nil {
		return key, err
	}
	pubPEM, err := helper.PublicKeyToPEM(&priv.PublicKey)
	if err != nil {
		return key, err
	}
	key = shareddomain.RealmKey{
		RealmID: realmID, KID: helper.RandomToken(12), Algorithm: "RS256",
		PublicKeyPEM: string(pubPEM), PrivateKeyEnc: enc, Active: true,
	}
	err = uc.repoSQL.RealmKeyRepo().Save(ctx, &key)
	return
}

func (uc *realmUsecaseImpl) GetAllRealmKey(ctx context.Context, realm string, filter *domain.FilterRealmKey) (data domain.ResponseRealmKeyList, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:GetAllRealmKey")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	keys, err := uc.repoSQL.RealmKeyRepo().FetchAll(ctx, r.ID, filter)
	if err != nil {
		return data, err
	}
	data.Meta = candishared.NewMeta(filter.Page, filter.Limit, uc.repoSQL.RealmKeyRepo().Count(ctx, r.ID, filter))
	data.Data = make([]domain.ResponseRealmKey, 0, len(keys))
	for i := range keys {
		var k domain.ResponseRealmKey
		k.Serialize(&keys[i])
		data.Data = append(data.Data, k)
	}
	return
}

func (uc *realmUsecaseImpl) GetDetailRealmKey(ctx context.Context, realm string, id int) (data domain.ResponseRealmKey, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:GetDetailRealmKey")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	key, err := uc.repoSQL.RealmKeyRepo().Find(ctx, r.ID, id)
	if err != nil {
		return data, common.NotFound(err, "realm key")
	}
	data.Serialize(&key)
	return
}

// RotateRealmKey creates a new active key. Older keys stay active (published in the JWKS) until
// they are deactivated, so tokens signed before the rotation keep validating.
func (uc *realmUsecaseImpl) RotateRealmKey(ctx context.Context, realm string) (res domain.ResponseRealmKey, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:RotateRealmKey")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return res, err
	}
	key, err := uc.newSigningKey(ctx, r.ID)
	if err != nil {
		return res, err
	}
	res.Serialize(&key)
	return
}

func (uc *realmUsecaseImpl) UpdateRealmKey(ctx context.Context, realm string, id int, req *domain.RequestRealmKey) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:UpdateRealmKey")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	key, err := uc.repoSQL.RealmKeyRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "realm key")
	}
	if key.Active && !req.Active {
		if err := uc.ensureAnotherActiveKey(ctx, r.ID, key.ID); err != nil {
			return err
		}
	}
	key.Active = req.Active
	return uc.repoSQL.RealmKeyRepo().Save(ctx, &key)
}

func (uc *realmUsecaseImpl) DeleteRealmKey(ctx context.Context, realm string, id int) (err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:DeleteRealmKey")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.ResolveRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return err
	}
	key, err := uc.repoSQL.RealmKeyRepo().Find(ctx, r.ID, id)
	if err != nil {
		return common.NotFound(err, "realm key")
	}
	if key.Active {
		return helper.NewConflict("an active key cannot be deleted, deactivate it first")
	}
	return uc.repoSQL.RealmKeyRepo().Delete(ctx, r.ID, id)
}

// ensureAnotherActiveKey keeps at least one key able to sign
func (uc *realmUsecaseImpl) ensureAnotherActiveKey(ctx context.Context, realmID, exceptID int) error {
	keys, err := uc.repoSQL.RealmKeyRepo().FetchActive(ctx, realmID)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if k.ID != exceptID {
			return nil
		}
	}
	return helper.NewConflict("the realm needs at least one active key, rotate first")
}

// GetJWKS is public: it publishes the active public keys of a realm
func (uc *realmUsecaseImpl) GetJWKS(ctx context.Context, realm string) (data auth.JWKS, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:GetJWKS")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.LoadRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	keys, err := uc.repoSQL.RealmKeyRepo().FetchActive(ctx, r.ID)
	if err != nil {
		return data, err
	}
	data.Keys = make([]auth.JWK, 0, len(keys))
	for _, k := range keys {
		pub, err := helper.PublicKeyFromPEM([]byte(k.PublicKeyPEM))
		if err != nil {
			return data, err
		}
		data.Keys = append(data.Keys, auth.NewJWK(pub, k.KID))
	}
	return
}

// GetOpenIDConfiguration is public discovery metadata of a realm
func (uc *realmUsecaseImpl) GetOpenIDConfiguration(ctx context.Context, realm string) (data domain.ResponseOpenIDConfiguration, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "RealmUsecase:GetOpenIDConfiguration")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	r, err := common.LoadRealm(ctx, uc.repoSQL, realm)
	if err != nil {
		return data, err
	}
	base := shared.GetEnv().IssuerBaseURL
	iss := auth.IssuerFor(base, r.Name)
	return domain.ResponseOpenIDConfiguration{
		Issuer:               iss,
		JWKSURI:              base + "/v1/realms/" + r.Name + "/.well-known/jwks.json",
		TokenEndpoint:        base + "/v1/realms/" + r.Name + "/auth/token",
		GrantTypesSupported:  []string{"password", "refresh_token", "client_credentials", "otp"},
		SigningAlgsSupported: []string{"RS256"},
	}, nil
}
