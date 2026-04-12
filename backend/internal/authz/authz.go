package authz

const DevAdminRole = "devadmin"

func HasRole(userRoles []string, requiredRole string) bool {
	if requiredRole == "" {
		return false
	}
	return HasAnyRole(userRoles, requiredRole)
}

func HasAnyRole(userRoles []string, requiredRoles ...string) bool {
	if len(requiredRoles) == 0 {
		return true
	}

	if containsRole(userRoles, DevAdminRole) {
		return true
	}

	for _, role := range requiredRoles {
		if role != "" && containsRole(userRoles, role) {
			return true
		}
	}

	return false
}

func containsRole(userRoles []string, role string) bool {
	for _, userRole := range userRoles {
		if userRole == role {
			return true
		}
	}
	return false
}
