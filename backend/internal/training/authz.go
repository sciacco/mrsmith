package training

import "strings"

func principalCanAccessEmployee(principal Principal, employeeEmail string) bool {
	if principal.IsPeopleAdmin {
		return true
	}
	return normalizeEmail(principal.Email) != "" && normalizeEmail(principal.Email) == normalizeEmail(employeeEmail)
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
