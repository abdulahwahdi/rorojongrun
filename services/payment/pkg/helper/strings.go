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
