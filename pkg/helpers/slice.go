package helpers

func SliceContainsString(slice []string, s string, modifier func(s string) string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
		if modifier != nil && modifier(item) == s {
			return true
		}
	}
	return false
}
