package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/golangid/candi/candihelper"
	restserver "github.com/golangid/candi/codebase/app/rest_server"
	"github.com/golangid/candi/codebase/interfaces"
	"github.com/golangid/candi/wrapper"
)

// URLParamInt reads an integer path parameter, 0 when missing / not numeric
func URLParamInt(req *http.Request, name string) int {
	n, _ := strconv.Atoi(restserver.URLParam(req, name))
	return n
}

// DecodeBody validates the request body against a JSON schema then unmarshals it.
// On failure the error response is already written and false is returned.
func DecodeBody(rw http.ResponseWriter, req *http.Request, validator interfaces.Validator, schema string, out any) bool {
	body, _ := io.ReadAll(req.Body)
	if err := validator.ValidateDocument(schema, body); err != nil {
		wrapper.NewHTTPResponse(http.StatusBadRequest, "Failed validate payload", err).JSON(rw)
		return false
	}
	if err := json.Unmarshal(body, out); err != nil {
		wrapper.NewHTTPResponse(http.StatusBadRequest, err.Error()).JSON(rw)
		return false
	}
	return true
}

// WriteError writes err with the status mapped by HTTPStatus
func WriteError(rw http.ResponseWriter, err error) {
	wrapper.NewHTTPResponse(HTTPStatus(err), err.Error()).JSON(rw)
}

// WriteOK writes a 200 response
func WriteOK(rw http.ResponseWriter, data ...any) {
	if len(data) > 0 {
		wrapper.NewHTTPResponse(http.StatusOK, "Success", data[0]).JSON(rw)
		return
	}
	wrapper.NewHTTPResponse(http.StatusOK, "Success").JSON(rw)
}

// Secure returns the route middlewares of an authenticated + permission checked admin route
func Secure(mw interfaces.Middleware, permissionCode string) []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{mw.HTTPBearerAuth, mw.HTTPPermissionACL(permissionCode)}
}

// ParseFilter parses query params into filter and validates them against a schema
func ParseFilter(rw http.ResponseWriter, req *http.Request, validator interfaces.Validator, schema string, filter any) bool {
	if err := candihelper.ParseFromQueryParam(req.URL.Query(), filter); err != nil {
		wrapper.NewHTTPResponse(http.StatusBadRequest, "Failed parse filter", err).JSON(rw)
		return false
	}
	if err := validator.ValidateDocument(schema, filter); err != nil {
		wrapper.NewHTTPResponse(http.StatusBadRequest, "Failed validate filter", err).JSON(rw)
		return false
	}
	return true
}
