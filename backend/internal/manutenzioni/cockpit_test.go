package manutenzioni

import "testing"

func TestCockpitReadinessBlocksDraftWithoutCustomerScope(t *testing.T) {
	detail := cockpitReadyDetail(StatusDraft)
	detail.CustomerScope = nil

	cockpit := buildMaintenanceCockpit(detail, nil)

	if cockpit.NextAction == nil || cockpit.NextAction.Action != "approve" {
		t.Fatalf("next action = %#v, want approve", cockpit.NextAction)
	}
	if cockpit.NextAction.Enabled {
		t.Fatalf("approve should be blocked without customer scope")
	}
	if !containsString(cockpit.NextAction.BlockedBy, "customer_scope") {
		t.Fatalf("blocked_by = %v, want customer_scope", cockpit.NextAction.BlockedBy)
	}
}

func TestCockpitReadinessBlocksApprovedWithoutPlannedWindow(t *testing.T) {
	detail := cockpitReadyDetail(StatusApproved)
	detail.Windows = nil
	detail.CurrentWindow = nil

	cockpit := buildMaintenanceCockpit(detail, nil)

	if cockpit.NextAction == nil || cockpit.NextAction.Action != "schedule" {
		t.Fatalf("next action = %#v, want schedule", cockpit.NextAction)
	}
	if cockpit.NextAction.Enabled {
		t.Fatalf("schedule should be blocked without planned window")
	}
	if !containsString(cockpit.NextAction.BlockedBy, "window") {
		t.Fatalf("blocked_by = %v, want window", cockpit.NextAction.BlockedBy)
	}
}

func TestCockpitReadinessBlocksMissingImpact(t *testing.T) {
	detail := cockpitReadyDetail(StatusDraft)
	detail.ServiceTaxonomy = nil
	detail.Targets = nil

	cockpit := buildMaintenanceCockpit(detail, nil)

	if cockpit.NextAction == nil || cockpit.NextAction.Enabled {
		t.Fatalf("approve should be blocked without service or target impact")
	}
	if !containsString(cockpit.NextAction.BlockedBy, "impact") {
		t.Fatalf("blocked_by = %v, want impact", cockpit.NextAction.BlockedBy)
	}
}

func TestCockpitReadinessBlocksUnresolvedServiceAudience(t *testing.T) {
	detail := cockpitReadyDetail(StatusDraft)
	audience := "maintenance"
	detail.ServiceTaxonomy[0].Reference.Audience = &audience
	detail.ServiceTaxonomy[0].ExpectedAudience = nil

	cockpit := buildMaintenanceCockpit(detail, nil)

	if cockpit.NextAction == nil || cockpit.NextAction.Enabled {
		t.Fatalf("approve should be blocked by unresolved audience")
	}
	if !containsString(cockpit.NextAction.BlockedBy, "audience") {
		t.Fatalf("blocked_by = %v, want audience", cockpit.NextAction.BlockedBy)
	}
}

func TestCockpitReadyToAnnounce(t *testing.T) {
	detail := cockpitReadyDetail(StatusScheduled)

	cockpit := buildMaintenanceCockpit(detail, nil)

	if cockpit.NextAction == nil || cockpit.NextAction.Action != "announce" {
		t.Fatalf("next action = %#v, want announce", cockpit.NextAction)
	}
	if !cockpit.NextAction.Enabled {
		t.Fatalf("announce should be enabled, blocked_by = %v", cockpit.NextAction.BlockedBy)
	}
}

func cockpitReadyDetail(status string) MaintenanceDetail {
	scope := ReferenceItem{ID: 1, Code: "subset", NameIT: "Clienti selezionati"}
	role := "operated"
	audience := "external"
	return MaintenanceDetail{
		MaintenanceID:   42,
		Code:            "MNT-42",
		TitleIT:         "Manutenzione test",
		Status:          status,
		CustomerScope:   &scope,
		CurrentWindow:   &WindowSummary{MaintenanceWindowID: 10, SeqNo: 1, WindowStatus: "planned"},
		Windows:         []MaintenanceWindow{{MaintenanceWindowID: 10, SeqNo: 1, WindowStatus: "planned"}},
		ServiceTaxonomy: []ClassificationItem{{Reference: ReferenceItem{ID: 20, NameIT: "Cloudstack"}, Role: &role, ExpectedAudience: &audience}},
	}
}

func containsString(items []string, expected string) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}
