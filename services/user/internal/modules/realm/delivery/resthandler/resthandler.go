package resthandler

import (
	"net/http"

	"monorepo/globalshared/rest"
	"monorepo/services/user/internal/modules/realm/domain"
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

// Mount handler with root "/". Routes are registered flat (no Group) because every module of this
// service shares the /v1/realms/:realm prefix and chi cannot mount one prefix twice.
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	base := candihelper.V1 + "/realms"

	root.GET(base, h.getAllRealm, rest.Secure(h.mw, "getAllRealm")...)
	root.POST(base, h.createRealm, rest.Secure(h.mw, "createRealm")...)
	root.GET(base+"/:realm", h.getDetailRealm, rest.Secure(h.mw, "getDetailRealm")...)
	root.PUT(base+"/:realm", h.updateRealm, rest.Secure(h.mw, "updateRealm")...)
	root.DELETE(base+"/:realm", h.deleteRealm, rest.Secure(h.mw, "deleteRealm")...)

	root.GET(base+"/:realm/keys", h.getAllRealmKey, rest.Secure(h.mw, "getAllRealmKey")...)
	root.POST(base+"/:realm/keys/rotate", h.rotateRealmKey, rest.Secure(h.mw, "rotateRealmKey")...)
	root.GET(base+"/:realm/keys/:id", h.getDetailRealmKey, rest.Secure(h.mw, "getDetailRealmKey")...)
	root.PUT(base+"/:realm/keys/:id", h.updateRealmKey, rest.Secure(h.mw, "updateRealmKey")...)
	root.DELETE(base+"/:realm/keys/:id", h.deleteRealmKey, rest.Secure(h.mw, "deleteRealmKey")...)

	// public discovery documents
	root.GET(base+"/:realm/.well-known/jwks.json", h.getJWKS)
	root.GET(base+"/:realm/.well-known/openid-configuration", h.getOpenIDConfiguration)
}

// getAllRealm godoc
// @Summary		Get All Realm (master realm only)
// @Tags		Realm
// @Produce		json
// @Success		200	{object}	domain.ResponseRealmList
// @Security	ApiKeyAuth
// @Router		/v1/realms [get]
func (h *RestHandler) getAllRealm(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:GetAllRealm")
	defer trace.Finish()

	var filter domain.FilterRealm
	if !rest.ParseFilter(rw, req, h.validator, "realm/get_all", &filter) {
		return
	}
	result, err := h.uc.Realm().GetAllRealm(ctx, &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

// getDetailRealm godoc
// @Summary		Get Detail Realm
// @Tags		Realm
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponseRealm
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm} [get]
func (h *RestHandler) getDetailRealm(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:GetDetailRealm")
	defer trace.Finish()

	data, err := h.uc.Realm().GetDetailRealm(ctx, restserver.URLParam(req, "realm"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// createRealm godoc
// @Summary		Create Realm (master realm only)
// @Tags		Realm
// @Accept		json
// @Param		data	body	domain.RequestRealm	true	"Body Data"
// @Success		201	{object}	domain.ResponseRealm
// @Security	ApiKeyAuth
// @Router		/v1/realms [post]
func (h *RestHandler) createRealm(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:CreateRealm")
	defer trace.Finish()

	var payload domain.RequestRealm
	if !rest.DecodeBody(rw, req, h.validator, "realm/save", &payload) {
		return
	}
	res, err := h.uc.Realm().CreateRealm(ctx, &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", res).JSON(rw)
}

// updateRealm godoc
// @Summary		Update Realm
// @Tags		Realm
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		data	body	domain.RequestRealm	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm} [put]
func (h *RestHandler) updateRealm(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:UpdateRealm")
	defer trace.Finish()

	var payload domain.RequestRealm
	if !rest.DecodeBody(rw, req, h.validator, "realm/update", &payload) {
		return
	}
	if err := h.uc.Realm().UpdateRealm(ctx, restserver.URLParam(req, "realm"), &payload); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// deleteRealm godoc
// @Summary		Delete Realm (master realm only)
// @Tags		Realm
// @Param		realm	path	string	true	"Realm name"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm} [delete]
func (h *RestHandler) deleteRealm(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:DeleteRealm")
	defer trace.Finish()

	if err := h.uc.Realm().DeleteRealm(ctx, restserver.URLParam(req, "realm")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// getAllRealmKey godoc
// @Summary		Get All Signing Keys of a Realm
// @Tags		Realm
// @Param		realm	path	string	true	"Realm name"
// @Success		200	{object}	domain.ResponseRealmKeyList
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/keys [get]
func (h *RestHandler) getAllRealmKey(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:GetAllRealmKey")
	defer trace.Finish()

	var filter domain.FilterRealmKey
	if !rest.ParseFilter(rw, req, h.validator, "realm/get_all_key", &filter) {
		return
	}
	result, err := h.uc.Realm().GetAllRealmKey(ctx, restserver.URLParam(req, "realm"), &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

// getDetailRealmKey godoc
// @Summary		Get Detail Signing Key (public part only)
// @Tags		Realm
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Key ID"
// @Success		200	{object}	domain.ResponseRealmKey
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/keys/{id} [get]
func (h *RestHandler) getDetailRealmKey(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:GetDetailRealmKey")
	defer trace.Finish()

	data, err := h.uc.Realm().GetDetailRealmKey(ctx, restserver.URLParam(req, "realm"), rest.URLParamInt(req, "id"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

// rotateRealmKey godoc
// @Summary		Rotate Signing Key (creates a new active key)
// @Tags		Realm
// @Param		realm	path	string	true	"Realm name"
// @Success		201	{object}	domain.ResponseRealmKey
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/keys/rotate [post]
func (h *RestHandler) rotateRealmKey(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:RotateRealmKey")
	defer trace.Finish()

	res, err := h.uc.Realm().RotateRealmKey(ctx, restserver.URLParam(req, "realm"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", res).JSON(rw)
}

// updateRealmKey godoc
// @Summary		Activate / Deactivate Signing Key
// @Tags		Realm
// @Accept		json
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Key ID"
// @Param		data	body	domain.RequestRealmKey	true	"Body Data"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/keys/{id} [put]
func (h *RestHandler) updateRealmKey(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:UpdateRealmKey")
	defer trace.Finish()

	var payload domain.RequestRealmKey
	if !rest.DecodeBody(rw, req, h.validator, "realm/update_key", &payload) {
		return
	}
	if err := h.uc.Realm().UpdateRealmKey(ctx, restserver.URLParam(req, "realm"), rest.URLParamInt(req, "id"), &payload); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// deleteRealmKey godoc
// @Summary		Delete an inactive Signing Key
// @Tags		Realm
// @Param		realm	path	string	true	"Realm name"
// @Param		id	path	int	true	"Key ID"
// @Security	ApiKeyAuth
// @Router		/v1/realms/{realm}/keys/{id} [delete]
func (h *RestHandler) deleteRealmKey(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:DeleteRealmKey")
	defer trace.Finish()

	if err := h.uc.Realm().DeleteRealmKey(ctx, restserver.URLParam(req, "realm"), rest.URLParamInt(req, "id")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}

// getJWKS godoc
// @Summary		JSON Web Key Set of a realm (public)
// @Tags		Realm
// @Param		realm	path	string	true	"Realm name"
// @Router		/v1/realms/{realm}/.well-known/jwks.json [get]
func (h *RestHandler) getJWKS(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:GetJWKS")
	defer trace.Finish()

	data, err := h.uc.Realm().GetJWKS(ctx, restserver.URLParam(req, "realm"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rw.Header().Set("Cache-Control", "public, max-age=60")
	writeRawJSON(rw, data) // standard document, not wrapped in the API envelope
}

// getOpenIDConfiguration godoc
// @Summary		OIDC discovery document of a realm (public)
// @Tags		Realm
// @Param		realm	path	string	true	"Realm name"
// @Router		/v1/realms/{realm}/.well-known/openid-configuration [get]
func (h *RestHandler) getOpenIDConfiguration(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "RealmDeliveryREST:GetOpenIDConfiguration")
	defer trace.Finish()

	data, err := h.uc.Realm().GetOpenIDConfiguration(ctx, restserver.URLParam(req, "realm"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	writeRawJSON(rw, data)
}
