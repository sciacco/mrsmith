package training

import "testing"

func TestNormalizeHREmployeeDefaultsAndRequiresIdentity(t *testing.T) {
	employee, ok := normalizeHREmployee(HREmployee{
		ExternalID: " 42 ",
		FirstName:  " Ada ",
		LastName:   " Lovelace ",
		Email:      " ADA@example.COM ",
	})
	if !ok {
		t.Fatal("expected valid HR employee")
	}
	if employee.ExternalSource != "factorial" {
		t.Fatalf("ExternalSource = %q, want factorial", employee.ExternalSource)
	}
	if employee.Email != "ada@example.com" {
		t.Fatalf("Email = %q", employee.Email)
	}
	if employee.Status != "active" {
		t.Fatalf("Status = %q, want active", employee.Status)
	}

	if _, ok := normalizeHREmployee(HREmployee{ExternalID: "42", FirstName: "Ada", LastName: "Lovelace"}); ok {
		t.Fatal("missing email should be invalid")
	}
}
