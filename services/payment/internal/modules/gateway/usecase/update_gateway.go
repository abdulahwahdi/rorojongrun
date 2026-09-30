package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"monorepo/globalshared/crypto"
	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/gateway/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/golangid/candi/tracer"
)

func (uc *gatewayUsecaseImpl) UpdateGateway(ctx context.Context, code string, req *domain.RequestUpdateGateway) (data domain.ResponseGateway, err error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "GatewayUsecase:UpdateGateway")
	defer func() { trace.Finish(tracer.FinishWithError(err)) }()

	row, creds, err := uc.load(ctx, code)
	if err != nil {
		return data, err
	}

	if req.Environment != "" {
		if req.Environment != shareddomain.EnvSandbox && req.Environment != shareddomain.EnvProduction {
			return data, rest.NewInvalid("environment must be sandbox or production")
		}
		if code == shareddomain.GatewayMock && req.Environment == shareddomain.EnvProduction {
			return data, rest.NewInvalid("the mock gateway cannot run in production")
		}
		row.Environment = req.Environment
	}
	if req.Name != "" {
		row.Name = req.Name
	}
	if req.Settings != nil {
		row.Settings = shareddomain.NewJSON(req.Settings)
	}

	if len(req.Credentials) > 0 {
		allowed := domain.AllowedCredentialKeys[code]
		for k, v := range req.Credentials {
			if !slices.Contains(allowed, k) {
				return data, rest.NewInvalid(fmt.Sprintf("unknown credential %q for gateway %s (allowed: %v)", k, code, allowed))
			}
			if v == "" {
				delete(creds, k)
			} else {
				creds[k] = v
			}
		}
		if secret := uc.encryptionSecret(); secret == "" {
			return data, rest.NewInvalid("GATEWAY_ENCRYPTION_SECRET is not configured, credentials cannot be stored")
		}
		if len(creds) == 0 {
			row.CredentialsEnc = ""
		} else {
			plain, _ := json.Marshal(creds)
			if row.CredentialsEnc, err = crypto.Encrypt(uc.encryptionSecret(), plain); err != nil {
				return data, err
			}
		}
	}

	// a gateway that is already enabled must not lose what it needs to work
	if row.IsEnabled {
		if err = requireCredentials(code, creds); err != nil {
			return data, err
		}
	}

	if err = uc.repoSQL.GatewayRepo().Save(ctx, &row); err != nil {
		return data, err
	}
	return uc.response(&row, creds), nil
}

func requireCredentials(code string, creds map[string]string) error {
	for _, k := range domain.RequiredCredentialKeys[code] {
		if creds[k] == "" {
			return rest.NewInvalid(fmt.Sprintf("gateway %s needs credential %q", code, k))
		}
	}
	return nil
}
