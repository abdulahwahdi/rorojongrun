package workerhandler

import (
	"os"
)

// testSchemas serves the service's JSON schemas (api/jsonschema) to the handler tests
var testSchemas = os.DirFS("../../../../../api")
