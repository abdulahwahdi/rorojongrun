package helper

// StrPtr returns nil for an empty string, so optional unique columns store NULL instead of ”
func StrPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// StrVal dereferences a possibly nil string
func StrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// PermissionAllows tells whether any granted (service, code) pair covers the wanted one.
// "*" as service or code matches everything.
func PermissionAllows(granted [][2]string, service, code string) bool {
	for _, g := range granted {
		if (g[0] == service || g[0] == "*") && (g[1] == code || g[1] == "*") {
			return true
		}
	}
	return false
}
