package resthandler

import (
	"encoding/json"
	"net/http"
)

func writeRawJSON(rw http.ResponseWriter, v any) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(v)
}
