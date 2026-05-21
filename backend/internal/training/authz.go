package training

import "strings"

func principalCanAccessEmployee(principal Principal, employeeEmail string) bool {
	if principal.IsPeopleAdmin {
		return true
	}
	return normalizeEmail(principal.Email) != "" && normalizeEmail(principal.Email) == normalizeEmail(employeeEmail)
}

func actorForPrincipal(principal Principal) Actor {
	if principal.IsPeopleAdmin {
		return ActorPeopleAdmin
	}
	return ActorEmployee
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
