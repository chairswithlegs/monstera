package testutil

// StrPtr returns a pointer to s. Useful in tests when building structs with optional *string fields.
func StrPtr(s string) *string {
	return &s
}
