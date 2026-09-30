package grpchandler

import (
	"context"
	"errors"

	"monorepo/globalshared/rest"
	proto "monorepo/sdk/user/proto/auth"
	"monorepo/services/user/internal/modules/auth/domain"
	"monorepo/services/user/pkg/helper"
	"monorepo/services/user/pkg/shared/usecase"

	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/factory/types"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/tracer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCHandler rpc handler — the internal authorization API other services call
type GRPCHandler struct {
	proto.UnimplementedAuthHandlerServer

	mw        interfaces.Middleware
	uc        usecase.Usecase
	validator interfaces.Validator
}

// NewGRPCHandler func
func NewGRPCHandler(uc usecase.Usecase, deps dependency.Dependency) *GRPCHandler {
	return &GRPCHandler{
		uc: uc, mw: deps.GetMiddleware(), validator: deps.GetValidator(),
	}
}

// Register grpc server. Callers are services, not end users, so the internal basic auth key protects it.
func (h *GRPCHandler) Register(server *grpc.Server, mwGroup *types.MiddlewareGroup) {
	proto.RegisterAuthHandlerServer(server, h)

	mwGroup.AddProto(proto.File_auth_auth_proto, h.CheckPermission, h.mw.GRPCBasicAuth)
	mwGroup.AddProto(proto.File_auth_auth_proto, h.GetUser, h.mw.GRPCBasicAuth)
	mwGroup.AddProto(proto.File_auth_auth_proto, h.GetUserPermissions, h.mw.GRPCBasicAuth)
}

func grpcError(err error) error {
	var appErr *rest.AppError
	if errors.As(err, &appErr) {
		switch appErr.Status {
		case 404:
			return status.Error(codes.NotFound, appErr.Message)
		case 400:
			return status.Error(codes.InvalidArgument, appErr.Message)
		}
	}
	return status.Error(codes.Internal, err.Error())
}

// CheckPermission rpc method
func (h *GRPCHandler) CheckPermission(ctx context.Context, req *proto.CheckPermissionRequest) (*proto.CheckPermissionResponse, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGRPC:CheckPermission")
	defer trace.Finish()

	allowed, role, err := h.uc.Auth().CheckPermission(ctx, domain.CheckPermissionRequest{
		Realm: req.Realm, UserID: req.UserId, SessionID: int(req.SessionId), Service: req.Service, Code: req.Code,
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &proto.CheckPermissionResponse{Allowed: allowed, Role: role}, nil
}

// GetUser rpc method
func (h *GRPCHandler) GetUser(ctx context.Context, req *proto.GetUserRequest) (*proto.UserModel, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGRPC:GetUser")
	defer trace.Finish()

	user, roles, err := h.uc.Auth().GetUserForService(ctx, req.Realm, int(req.Id))
	if err != nil {
		return nil, grpcError(err)
	}
	return &proto.UserModel{
		Id: int64(user.ID), Realm: req.Realm, Username: user.Username, Email: helper.StrVal(user.Email),
		Phone: helper.StrVal(user.Phone), FullName: user.FullName, Status: user.Status, Roles: roles,
	}, nil
}

// GetUserPermissions rpc method
func (h *GRPCHandler) GetUserPermissions(ctx context.Context, req *proto.GetUserPermissionsRequest) (*proto.GetUserPermissionsResponse, error) {
	trace, ctx := tracer.StartTraceWithContext(ctx, "AuthDeliveryGRPC:GetUserPermissions")
	defer trace.Finish()

	codesList, err := h.uc.Auth().GetUserPermissions(ctx, req.Realm, int(req.UserId), req.Service)
	if err != nil {
		return nil, grpcError(err)
	}
	return &proto.GetUserPermissionsResponse{Codes: codesList}, nil
}
