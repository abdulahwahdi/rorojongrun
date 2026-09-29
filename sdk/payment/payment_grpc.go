package payment

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	proto "monorepo/sdk/payment/proto/payment"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type paymentGRPCImpl struct {
	host    string
	authKey string
	conn    *grpc.ClientConn
	client  proto.PaymentHandlerClient
}

// NewPaymentServiceGRPC constructor
func NewPaymentServiceGRPC(host string, authKey string) Payment {

	if u, _ := url.Parse(host); u.Host != "" {
		host = u.Host
	}
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithConnectParams(grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  50 * time.Millisecond,
			Multiplier: 5,
			MaxDelay:   50 * time.Millisecond,
		},
		MinConnectTimeout: 1 * time.Second,
	}))
	if err != nil {
		panic(err)
	}

	return &paymentGRPCImpl{
		host:    host,
		authKey: authKey,
		conn:    conn,
		client:  proto.NewPaymentHandlerClient(conn),
	}
}

// authCtx attaches the internal basic auth key expected by the payment service's gRPC server
func (p *paymentGRPCImpl) authCtx(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Basic "+p.authKey)
}

func (p *paymentGRPCImpl) CreatePayment(ctx context.Context, req CreatePaymentRequest) (res CreatePaymentResponse, err error) {
	in := &proto.CreatePaymentRequest{
		Source: req.Source, ReferenceId: req.ReferenceID, Description: req.Description, Amount: req.Amount,
		Customer:       &proto.Customer{Name: req.Customer.Name, Email: req.Customer.Email, Phone: req.Customer.Phone},
		AllowedMethods: req.AllowedMethods, ExpiresInSec: int64(req.ExpiresInSec),
		SuccessUrl: req.SuccessURL, FailureUrl: req.FailureURL,
	}
	for _, it := range req.Items {
		in.Items = append(in.Items, &proto.Item{Name: it.Name, Price: it.Price, Quantity: int32(it.Quantity)})
	}
	if len(req.Metadata) > 0 {
		b, err := json.Marshal(req.Metadata)
		if err != nil {
			return res, err
		}
		in.MetadataJson = string(b)
	}
	out, err := p.client.CreatePayment(p.authCtx(ctx), in)
	if err != nil {
		return res, err
	}
	return CreatePaymentResponse{
		PaymentID: out.PaymentId, Token: out.Token, PaymentURL: out.PaymentUrl, Status: out.Status,
		ReferenceID: out.ReferenceId, Amount: out.Amount, Currency: out.Currency, ExpiresAt: out.ExpiresAt,
	}, nil
}

func toPaymentResponse(m *proto.PaymentModel) PaymentResponse {
	return PaymentResponse{
		ID: m.Id, Source: m.Source, ReferenceID: m.ReferenceId, Description: m.Description, Amount: m.Amount,
		Fee: m.Fee, TotalAmount: m.TotalAmount, Currency: m.Currency, Status: m.Status, MethodCode: m.MethodCode,
		ExpiresAt: m.ExpiresAt, PaidAt: m.PaidAt, CreatedAt: m.CreatedAt,
	}
}

func (p *paymentGRPCImpl) GetPayment(ctx context.Context, id string) (res PaymentResponse, err error) {
	out, err := p.client.GetPayment(p.authCtx(ctx), &proto.GetPaymentRequest{Id: id})
	if err != nil {
		return res, err
	}
	return toPaymentResponse(out), nil
}

func (p *paymentGRPCImpl) CancelPayment(ctx context.Context, id string) (res PaymentResponse, err error) {
	out, err := p.client.CancelPayment(p.authCtx(ctx), &proto.CancelPaymentRequest{Id: id})
	if err != nil {
		return res, err
	}
	return toPaymentResponse(out), nil
}
