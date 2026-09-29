package grpchandler

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"monorepo/globalshared/rest"
	proto "monorepo/sdk/payment/proto/payment"
	"monorepo/services/payment/internal/modules/payment/domain"
	shareddomain "monorepo/services/payment/pkg/shared/domain"
	"monorepo/services/payment/pkg/shared/usecase"

	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/tracer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCHandler rpc handler — the internal API other services use to request payments
type GRPCHandler struct {
	proto.UnimplementedPaymentHandlerServer

	mw        interfaces.Middleware
	uc        usecase.Usecase
	validator interfaces.Validator
}

// NewGRPCHandler func
func NewGRPCHandler(uc usecase.Usecase, deps dependency.Dependency) *GRPCHandler {
	return &GRPCHandler{uc: uc, mw: deps.GetMiddleware(), validator: deps.GetValidator()}
}

// Register grpc server. Callers are services, so the internal basic auth key protects it.
func (h *GRPCHandler) Register(server *grpc.Server, mwGroup *types.MiddlewareGroup) {
	proto.RegisterPaymentHandlerServer(server, h)

	mwGroup.AddProto(proto.File_payment_payment_proto, h.CreatePayment, h.mw.GRPCBasicAuth)
	mwGroup.AddProto(proto.File_payment_payment_proto, h.GetPayment, h.mw.GRPCBasicAuth)
	mwGroup.AddProto(proto.File_payment_payment_proto, h.CancelPayment, h.mw.GRPCBasicAuth)
}

func grpcError(err error) error {
	var appErr *rest.AppError
	if errors.As(err, &appErr) {
		switch appErr.Status {
		case 404:
			return status.Error(codes.NotFound, appErr.Message)
		case 400:
			return status.Error(codes.InvalidArgument, appErr.Message)
		case 409:
			return status.Error(codes.AlreadyExists, appErr.Message)
		case 502, 503:
			return status.Error(codes.Unavailable, appErr.Message)
		}
	}
	return status.Error(codes.Internal, err.Error())
}

// CreatePayment rpc method
func (h *GRPCHandler) CreatePayment(ctx context.Context, req *proto.CreatePaymentRequest) (*proto.CreatePaymentResponse, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentDeliveryGRPC:CreatePayment")
	defer trace.Finish()

	payload := domain.RequestCreatePayment{
		Source: req.Source, ReferenceID: req.ReferenceId, Description: req.Description, Amount: req.Amount,
		AllowedMethods: req.AllowedMethods, ExpiresInSec: int(req.ExpiresInSec),
		SuccessURL: req.SuccessUrl, FailureURL: req.FailureUrl,
	}
	if c := req.Customer; c != nil {
		payload.Customer = shareddomain.Customer{Name: c.Name, Email: c.Email, Phone: c.Phone}
	}
	for _, it := range req.Items {
		payload.Items = append(payload.Items, shareddomain.Item{Name: it.Name, Price: it.Price, Quantity: int(it.Quantity)})
	}
	if req.MetadataJson != "" {
		if err := json.Unmarshal([]byte(req.MetadataJson), &payload.Metadata); err != nil {
			return nil, status.Error(codes.InvalidArgument, "metadataJson must be a JSON object")
		}
	}

	res, err := h.uc.Payment().CreatePayment(ctx, payload.Source, &payload)
	if err != nil {
		return nil, grpcError(err)
	}
	return &proto.CreatePaymentResponse{
		PaymentId: res.PaymentID, Token: res.Token, PaymentUrl: res.PaymentURL, Status: res.Status,
		ReferenceId: res.ReferenceID, Amount: res.Amount, Currency: res.Currency, ExpiresAt: res.ExpiresAt,
	}, nil
}

// GetPayment rpc method
func (h *GRPCHandler) GetPayment(ctx context.Context, req *proto.GetPaymentRequest) (*proto.PaymentModel, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentDeliveryGRPC:GetPayment")
	defer trace.Finish()

	res, err := h.uc.Payment().GetPayment(ctx, req.Id)
	if err != nil {
		return nil, grpcError(err)
	}
	return toModel(&res.Payment), nil
}

// CancelPayment rpc method
func (h *GRPCHandler) CancelPayment(ctx context.Context, req *proto.CancelPaymentRequest) (*proto.PaymentModel, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "PaymentDeliveryGRPC:CancelPayment")
	defer trace.Finish()

	res, err := h.uc.Payment().CancelPayment(ctx, req.Id)
	if err != nil {
		return nil, grpcError(err)
	}
	return toModel(&res.Payment), nil
}

func toModel(p *shareddomain.Payment) *proto.PaymentModel {
	m := &proto.PaymentModel{
		Id: p.ID, Source: p.Source, ReferenceId: p.ReferenceID, Description: p.Description,
		Amount: p.Amount, Fee: p.Fee, TotalAmount: p.TotalAmount, Currency: p.Currency, Status: p.Status,
		MethodCode: p.MethodCode, ExpiresAt: p.ExpiresAt.Format(time.RFC3339), CreatedAt: p.CreatedAt.Format(time.RFC3339),
	}
	if p.PaidAt != nil {
		m.PaidAt = p.PaidAt.Format(time.RFC3339)
	}
	return m
}
