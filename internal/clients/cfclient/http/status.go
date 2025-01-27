package http

func isStatusIn(statusCode int, statuses ...int) bool {
	for _, s := range statuses {
		if statusCode == s {
			return true
		}
	}
	return false
}
