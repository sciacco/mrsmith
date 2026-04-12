package authz

import "testing"

func TestHasAnyRole(t *testing.T) {
	tests := []struct {
		name          string
		userRoles     []string
		requiredRoles []string
		want          bool
	}{
		{
			name:          "matching role",
			userRoles:     []string{"app_budget_access"},
			requiredRoles: []string{"app_budget_access"},
			want:          true,
		},
		{
			name:          "devadmin bypass",
			userRoles:     []string{DevAdminRole},
			requiredRoles: []string{"app_budget_access"},
			want:          true,
		},
		{
			name:          "no match",
			userRoles:     []string{"viewer"},
			requiredRoles: []string{"app_budget_access"},
			want:          false,
		},
		{
			name:          "empty required allows",
			userRoles:     []string{"viewer"},
			requiredRoles: nil,
			want:          true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HasAnyRole(tc.userRoles, tc.requiredRoles...)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name         string
		userRoles    []string
		requiredRole string
		want         bool
	}{
		{
			name:         "exact role",
			userRoles:    []string{"app_quotes_delete"},
			requiredRole: "app_quotes_delete",
			want:         true,
		},
		{
			name:         "devadmin bypass",
			userRoles:    []string{DevAdminRole},
			requiredRole: "app_quotes_delete",
			want:         true,
		},
		{
			name:         "empty required role denied",
			userRoles:    []string{DevAdminRole},
			requiredRole: "",
			want:         false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HasRole(tc.userRoles, tc.requiredRole)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
