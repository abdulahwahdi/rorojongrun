package resthandler

import (
	"net/http"

	"monorepo/globalshared/rest"
	"monorepo/services/payment/internal/modules/topic/domain"
	"monorepo/services/payment/pkg/shared/usecase"

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
	return &RestHandler{uc: uc, mw: deps.GetMiddleware(), validator: deps.GetValidator()}
}

// Mount handler with root "/". Routes are registered flat, see the user service for why.
func (h *RestHandler) Mount(root interfaces.RESTRouter) {
	root.GET("/v1/kafka-topics", h.getAll, rest.Secure(h.mw, "manageTopics")...)
	root.POST("/v1/kafka-topics", h.create, rest.Secure(h.mw, "manageTopics")...)
	root.GET("/v1/kafka-topics/:id", h.get, rest.Secure(h.mw, "manageTopics")...)
	root.PUT("/v1/kafka-topics/:id", h.update, rest.Secure(h.mw, "manageTopics")...)
	root.PATCH("/v1/kafka-topics/:id/status", h.setStatus, rest.Secure(h.mw, "manageTopics")...)
	root.DELETE("/v1/kafka-topics/:id", h.delete, rest.Secure(h.mw, "manageTopics")...)
}

func (h *RestHandler) getAll(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "TopicDeliveryREST:GetAll")
	defer trace.Finish()

	var filter domain.FilterTopic
	if !rest.ParseFilter(rw, req, h.validator, "topic/get_all", &filter) {
		return
	}
	result, err := h.uc.Topic().GetAllTopics(ctx, &filter)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	response := wrapper.NewHTTPResponse(http.StatusOK, "Success", result.Data)
	response.Meta = result.Meta
	response.JSON(rw)
}

func (h *RestHandler) get(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "TopicDeliveryREST:Get")
	defer trace.Finish()

	data, err := h.uc.Topic().GetTopic(ctx, rest.URLParamInt(req, "id"))
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

func (h *RestHandler) create(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "TopicDeliveryREST:Create")
	defer trace.Finish()

	var payload domain.RequestSaveTopic
	if !rest.DecodeBody(rw, req, h.validator, "topic/save", &payload) {
		return
	}
	data, err := h.uc.Topic().CreateTopic(ctx, &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	wrapper.NewHTTPResponse(http.StatusCreated, "Success", data).JSON(rw)
}

func (h *RestHandler) update(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "TopicDeliveryREST:Update")
	defer trace.Finish()

	var payload domain.RequestSaveTopic
	if !rest.DecodeBody(rw, req, h.validator, "topic/save", &payload) {
		return
	}
	data, err := h.uc.Topic().UpdateTopic(ctx, rest.URLParamInt(req, "id"), &payload)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

func (h *RestHandler) setStatus(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "TopicDeliveryREST:SetStatus")
	defer trace.Finish()

	var payload domain.RequestSetStatus
	if !rest.DecodeBody(rw, req, h.validator, "topic/status", &payload) {
		return
	}
	data, err := h.uc.Topic().SetTopicStatus(ctx, rest.URLParamInt(req, "id"), payload.IsEnabled)
	if err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw, data)
}

func (h *RestHandler) delete(rw http.ResponseWriter, req *http.Request) {
	trace, ctx := tracer.StartTraceWithContext(req.Context(), "TopicDeliveryREST:Delete")
	defer trace.Finish()

	if err := h.uc.Topic().DeleteTopic(ctx, rest.URLParamInt(req, "id")); err != nil {
		rest.WriteError(rw, err)
		return
	}
	rest.WriteOK(rw)
}
