package training

import "testing"

func TestFilterCertificationRowsMatchesVisibleFilters(t *testing.T) {
	rows := []CertificationRow{
		{
			AwardID:           "award-valid",
			EmployeeName:      "Bianchi Laura",
			EmployeeEmail:     "laura@example.com",
			CertificationCode: "CKA",
			CertificationName: "Certified Kubernetes Administrator",
			Outcome:           "passed_exam",
			AwardedOn:         "2026-04-18",
			CurrentStatus:     "valid",
			ValidationSource:  "document_verified",
			DocumentFilename:  "cka.pdf",
		},
		{
			AwardID:           "award-expired",
			EmployeeName:      "Rossi Marco",
			EmployeeEmail:     "marco@example.com",
			CertificationCode: "ITIL4",
			CertificationName: "ITIL 4 Foundation",
			Outcome:           "attendance_only",
			AwardedOn:         "2025-09-18",
			CurrentStatus:     "missing_or_expired",
			ValidationSource:  "imported_legacy",
			DocumentFilename:  "itil.pdf",
		},
	}

	filtered := filterCertificationRows(rows, map[string]string{
		"q":      "kubernetes",
		"status": "valid",
		"year":   "2026",
	})
	if len(filtered) != 1 {
		t.Fatalf("filtered length = %d, want 1", len(filtered))
	}
	if filtered[0].AwardID != "award-valid" {
		t.Fatalf("AwardID = %q, want award-valid", filtered[0].AwardID)
	}
}
