package quotes

import "testing"

func TestCanDeleteQuote(t *testing.T) {
	tests := []struct {
		name      string
		userRoles []string
		want      bool
	}{
		{
			name:      "has delete role",
			userRoles: []string{"app_quotes_delete"},
			want:      true,
		},
		{
			name:      "app_devadmin bypass",
			userRoles: []string{"app_devadmin"},
			want:      true,
		},
		{
			name:      "missing delete role",
			userRoles: []string{"app_quotes_access"},
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canDeleteQuote(tc.userRoles)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
