package resthandler

import (
	"net"
	"net/http"
	"strings"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/auth/domain"
	"monorepo/services/user/pkg/shared/usecase"

	"github.com/golangid/candi/candihelper"
	restserver "github.com/golangid/candi/codebase/app/rest_server"
	"github.com/golangid/candi/codebase/factory/dependency"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/tracer"
	"github.com/golangid/candi/wrapper"
)

// RestHandler handler
type RestHandler struct {
	mw        interfaces.Middleware
	uc        usecase.Usecase
	validator interfaces.Validator
}

// NewRestHandler create new rest handler
func NewRestHandler(uc usecase.Usecase, deps dependency.Dependency) *RestHandler {
	return &RestHandler{
		uc: uc, mw: deps.GetMiddleware(), validator: deps.GetValidator(),
	}
}

// Mount handler with root "/" (flat routes, see realm resthandler for why)
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	base := candihelper.V1 + "/realms/:realm"

	// public: login
	root.POST(base+"/auth/token", h.token)
	root.POST(base+"/auth/otp/request", h.requestOTP)

	// authenticated principal, no extra permission needed to look at yourself
	root.POST(base+"/auth/logout", h.logout, h.mw.HTTPBearerAuth)
	root.GET(base+"/me", h.getMe, h.mw.HTTPBearerAuth)
	root.GET(base+"/me/permissions", h.getMyPermissions, h.mw.HTTPBearerAuth)
	root.GET(base+"/me/sessions", h.getMySessions, h.mw.HTTPBearerAuth)
	root.DELETE(base+"/me/sessions/:sessionId", h.revokeMySession, h.mw.HTTPBearerAuth)

	// admin: sessions of the realm
	root.GET(base+"/sessions", h.getAllSession, rest.Secure(h.mw, "getAllSession")...)
	root.GET(base+"/sessions/:sessionId", h.getDetailSession, rest.Secure(h.mw, "getDetailSession")...)
	root.DELETE(base+"/sessions/:sessionId", h.revokeSession, rest.Secure(h.mw, "revokeSession")...)
	root.GET(base+"/users/:id/sessions", h.getUserSessions, rest.Secure(h.mw, "getAllSession")...)
	root.DELETE(base+"/users/:id/sessions", h.revokeUserSessions, rest.Secure(h.mw, "revokeUserSessions")...)
}

func realmParam(req *http.Request) string { return restserver.URLParam(req, "realm") }

// clientMeta extracts the caller's ip and user agent
func clientMeta(req *http.Request) domain.ClientMeta {
	ip := strings.TrimSpace(strings.Split(req.Header.Get("X-Forwarded-For"), ",")[0])
	if ip == "" {
		ip, _, _ = net.SplitHostPort(req.RemoteAddr)
	}
	return domain.ClientMeta{IP: ip, UserAgent: req.UserAgent()}
}

// token godoc
// @Summary		Login / refresh: password, refresh_token, client_credentials and otp grants
// @Tags		Auth
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		data	body	domain.RequestToken	true	"Body Data"
// @Success		200	{object}	domain.ResponseToken
// @Router		/v1/realms/{realm}/auth/token [post]
func (h *RestHandler) token(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:Token")
	defer trace.Finish()

	var payload domain.RequestToken
	if !rest.DecodeBody(rw, req, h.validator, "auth/token", &payload) {
		return
	}
	res, err := h.uc.Auth().Token(ctx, realmParam(req), &payload, clientMeta(req))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rw.Header().Set("Cache-Control", "no-store")
	rest.WriteOK(rw, res)
}

// requestOTP godoc
// @Summary		Send a login OTP to an email address (always 202, never reveals whether the account exists)
// @Tags		Auth
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		data	body	domain.RequestOTPLogin	true	"Body Data"
// @Success		202
// @Router		/v1/realms/{realm}/auth/otp/request [post]
func (h *RestHandler) requestOTP(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:RequestOTP")
	defer trace.Finish()

	var payload domain.RequestOTPLogin
	if !rest.DecodeBody(rw, req, h.validator, "auth/otp_request", &payload) {
		return
	}
	if err := h.uc.Auth().RequestOTP(ctx, realmParam(req), &payload); err != nil {
		rest.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusAccepted, "If the account exists a code was sent").JSON(rw)
}

// logout godoc
// @Summary		Revoke the session of the calling token
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/auth/logout [post]
func (h *RestHandler) logout(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:Logout")
	defer trace.Finish()

	if err := h.uc.Auth().Logout(ctx, realmParam(req)); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// getMe godoc
// @Summary		Profile of the authenticated principal
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponseMe
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/me [get]
func (h *RestHandler) getMe(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:GetMe")
	defer trace.Finish()

	data, err := h.uc.Auth().GetMe(ctx, realmParam(req))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// getMyPermissions godoc
// @Summary		Granted permission codes plus the menu tree of a client, filtered for the frontend
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Param		client	query	string	false	"client_id whose menu tree to return"
// @Success		200	{object}	domain.ResponseMyPermissions
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/me/permissions [get]
func (h *RestHandler) getMyPermissions(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:GetMyPermissions")
	defer trace.Finish()

	data, err := h.uc.Auth().GetMyPermissions(ctx, realmParam(req), req.URL.Query().Get("client"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// getMySessions godoc
// @Summary		Own sessions (logins)
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponseSessionList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/me/sessions [get]
func (h *RestHandler) getMySessions(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:GetMySessions")
	defer trace.Finish()

	var filter domain.FilterSession
	if !rest.ParseFilter(rw, req, h.validator, "auth/get_all_session", &filter) {
		return
	}
	writeSessions(rw)(h.uc.Auth().GetMySessions(ctx, realmParam(req), &filter))
}

// revokeMySession godoc
// @Summary		Revoke one of the own sessions
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Param		sessionId	path	int	true	"Session ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/me/sessions/{sessionId} [delete]
func (h *RestHandler) revokeMySession(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:RevokeMySession")
	defer trace.Finish()

	if err := h.uc.Auth().RevokeMySession(ctx, realmParam(req), rest.URLParamInt(req, "sessionId")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

func writeSessions(rw http.ResponseWriter) func(domain.ResponseSessionList, error) {
	return func(result domain.ResponseSessionList, err error) {
		if err != nil {
			rest.WriteError(rw, err)
			return
		}
		response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
		response.Meta = result.Meta
		response.JSON(rw)
	}
}

// getAllSession godoc
// @Summary		Get All Session of a realm
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponseSessionList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/sessions [get]
func (h *RestHandler) getAllSession(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:GetAllSession")
	defer trace.Finish()

	var filter domain.FilterSession
	if !rest.ParseFilter(rw, req, h.validator, "auth/get_all_session", &filter) {
		return
	}
	writeSessions(rw)(h.uc.Auth().GetAllSession(ctx, realmParam(req), &filter))
}

// getUserSessions godoc
// @Summary		Sessions of one user
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Success		200	{object}	domain.ResponseSessionList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id}/sessions [get]
func (h *RestHandler) getUserSessions(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:GetUserSessions")
	defer trace.Finish()

	var filter domain.FilterSession
	if !rest.ParseFilter(rw, req, h.validator, "auth/get_all_session", &filter) {
		return
	}
	userID := rest.URLParamInt(req, "id")
	filter.UserID = &userID
	writeSessions(rw)(h.uc.Auth().GetAllSession(ctx, realmParam(req), &filter))
}

// getDetailSession godoc
// @Summary		Get Detail Session
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Param		sessionId	path	int	true	"Session ID"
// @Success		200	{object}	domain.ResponseSession
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/sessions/{sessionId} [get]
func (h *RestHandler) getDetailSession(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:GetDetailSession")
	defer trace.Finish()

	data, err := h.uc.Auth().GetDetailSession(ctx, realmParam(req), rest.URLParamInt(req, "sessionId"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// revokeSession godoc
// @Summary		Revoke a session (the whole refresh token family)
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Param		sessionId	path	int	true	"Session ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/sessions/{sessionId} [delete]
func (h *RestHandler) revokeSession(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:RevokeSession")
	defer trace.Finish()

	if err := h.uc.Auth().RevokeSession(ctx, realmParam(req), rest.URLParamInt(req, "sessionId")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// revokeUserSessions godoc
// @Summary		Revoke every session of a user
// @Tags		Auth
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"User ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/users/{id}/sessions [delete]
func (h *RestHandler) revokeUserSessions(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "AuthDeliveryREST:RevokeUserSessions")
	defer trace.Finish()

	if err := h.uc.Auth().RevokeUserSessions(ctx, realmParam(req), rest.URLParamInt(req, "id")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}
