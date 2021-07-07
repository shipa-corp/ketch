package testutils

func IntPtr(i int) *int {
	return &i
}

func StrPtr(s string) *string {
	return &s
}

func BoolPtr(b bool) *bool {
	return &b
}
