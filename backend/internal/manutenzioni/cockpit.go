package manutenzioni

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

type maintenanceCockpitResponse struct {
	MaintenanceID int64                  `json:"maintenance_id"`
	Status        string                 `json:"status"`
	Lifecycle     []cockpitLifecycleStep `json:"lifecycle"`
	NextAction    *cockpitNextAction     `json:"next_action,omitempty"`
	Readiness     []cockpitReadinessItem `json:"readiness"`
	Impact        cockpitImpact          `json:"impact"`
	Timeline      []cockpitTimelineItem  `json:"timeline"`
}

type cockpitLifecycleStep struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	State   string `json:"state"`
	Current bool   `json:"current"`
}

type cockpitNextAction struct {
	Action    string   `json:"action"`
	Label     string   `json:"label"`
	Enabled   bool     `json:"enabled"`
	TargetTab string   `json:"target_tab"`
	BlockedBy []string `json:"blocked_by"`
}

type cockpitReadinessItem struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	TargetTab string `json:"target_tab"`
	Blocking  bool   `json:"blocking"`
}

type cockpitImpact struct {
	OperatedServices  []ClassificationItem `json:"operated_services"`
	DependentServices []ClassificationItem `json:"dependent_services"`
	Targets           []MaintenanceTarget  `json:"targets"`
	Customers         []ImpactedCustomer   `json:"customers"`
	Dependencies      []ServiceDependency  `json:"dependencies"`
	Summary           cockpitImpactSummary `json:"summary"`
}

type cockpitImpactSummary struct {
	Services     int `json:"services"`
	Targets      int `json:"targets"`
	Customers    int `json:"customers"`
	Dependencies int `json:"dependencies"`
}

type cockpitTimelineItem struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Label     string     `json:"label"`
	Status    string     `json:"status"`
	StartAt   *time.Time `json:"start_at,omitempty"`
	EndAt     *time.Time `json:"end_at,omitempty"`
	EventAt   *time.Time `json:"event_at,omitempty"`
	Summary   string     `json:"summary,omitempty"`
	TargetTab string     `json:"target_tab"`
}

func (h *Handler) handleMaintenanceCockpit(w http.ResponseWriter, r *http.Request) {
	if !h.requireMaintenanceDB(w) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		appError(w, http.StatusBadRequest, "invalid_maintenance_id")
		return
	}
	detail, err := h.loadMaintenanceDetail(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		appError(w, http.StatusNotFound, "maintenance_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "get_cockpit_detail", err, "maintenance_id", id)
		return
	}
	dependencies, err := h.loadCockpitDependencies(r.Context(), detail)
	if err != nil {
		h.dbFailure(w, r, "get_cockpit_dependencies", err, "maintenance_id", id)
		return
	}
	httputil.JSON(w, http.StatusOK, buildMaintenanceCockpit(detail, dependencies))
}

func buildMaintenanceCockpit(detail MaintenanceDetail, dependencies []ServiceDependency) maintenanceCockpitResponse {
	readiness := cockpitReadiness(detail)
	return maintenanceCockpitResponse{
		MaintenanceID: detail.MaintenanceID,
		Status:        detail.Status,
		Lifecycle:     cockpitLifecycle(detail.Status),
		NextAction:    cockpitNextActionFor(detail.Status, readiness),
		Readiness:     readiness,
		Impact:        cockpitImpactFrom(detail, dependencies),
		Timeline:      cockpitTimeline(detail),
	}
}

func cockpitLifecycle(status string) []cockpitLifecycleStep {
	order := []struct {
		key   string
		label string
	}{
		{StatusDraft, "Bozza"},
		{StatusApproved, "Approvata"},
		{StatusScheduled, "Pianificata"},
		{StatusAnnounced, "Annunciata"},
		{StatusInProgress, "In corso"},
		{StatusCompleted, "Completata"},
	}
	currentIndex := -1
	for index, item := range order {
		if item.key == status {
			currentIndex = index
			break
		}
	}
	steps := make([]cockpitLifecycleStep, 0, len(order))
	for index, item := range order {
		state := "locked"
		if status == StatusCancelled || status == StatusSuperseded {
			state = "locked"
		} else if currentIndex >= 0 {
			switch {
			case index < currentIndex:
				state = "complete"
			case index == currentIndex:
				state = "current"
			default:
				state = "pending"
			}
		}
		steps = append(steps, cockpitLifecycleStep{
			Key:     item.key,
			Label:   item.label,
			State:   state,
			Current: item.key == status,
		})
	}
	return steps
}

func cockpitNextActionFor(status string, readiness []cockpitReadinessItem) *cockpitNextAction {
	next := &cockpitNextAction{TargetTab: "cockpit"}
	switch status {
	case StatusDraft:
		next.Action = "approve"
		next.Label = "Approva"
	case StatusApproved:
		next.Action = "schedule"
		next.Label = "Pianifica"
		next.TargetTab = "finestre"
	case StatusScheduled:
		next.Action = "announce"
		next.Label = "Annuncia"
		next.TargetTab = "comunicazioni"
	case StatusAnnounced:
		next.Action = "start"
		next.Label = "Avvia"
		next.TargetTab = "finestre"
	case StatusInProgress:
		next.Action = "complete"
		next.Label = "Completa"
	default:
		return nil
	}
	next.BlockedBy = cockpitBlockersForAction(next.Action, readiness)
	next.Enabled = len(next.BlockedBy) == 0
	return next
}

func cockpitBlockersForAction(action string, readiness []cockpitReadinessItem) []string {
	required := map[string]bool{}
	switch action {
	case "approve":
		required["customer_scope"] = true
		required["impact"] = true
		required["audience"] = true
	case "schedule", "announce", "start":
		required["customer_scope"] = true
		required["window"] = true
		required["impact"] = true
		required["audience"] = true
	default:
		return nil
	}
	blockers := []string{}
	for _, item := range readiness {
		if item.Blocking && required[item.Key] {
			blockers = append(blockers, item.Key)
		}
	}
	return blockers
}

func cockpitReadiness(detail MaintenanceDetail) []cockpitReadinessItem {
	items := []cockpitReadinessItem{}
	if detail.CustomerScope == nil {
		items = append(items, cockpitBlockingItem("customer_scope", "Ambito clienti", "Definisci l'ambito clienti prima di avanzare.", "riepilogo"))
	} else {
		items = append(items, cockpitReadyItem("customer_scope", "Ambito clienti", detail.CustomerScope.NameIT, "riepilogo"))
	}

	validWindows := chronologicalOperationalWindows(detail.Windows)
	if len(validWindows) > 0 {
		index := nextOperationalWindowIndex(validWindows, time.Now())
		label := "Finestra"
		if len(validWindows) > 1 {
			label = label + " (" + strconv.Itoa(index+1) + "/" + strconv.Itoa(len(validWindows)) + ")"
		}
		items = append(items, cockpitReadyItem("window", label, validWindows[index].ScheduledStartAt.Format("02/01/2006 15:04"), "finestre"))
	} else {
		items = append(items, cockpitBlockingItem("window", "Finestra", windowScheduledPromptLabel, "finestre"))
	}

	if len(detail.ServiceTaxonomy) == 0 && len(detail.Targets) == 0 {
		items = append(items, cockpitBlockingItem("impact", "Impatto", "Seleziona almeno un servizio o target coinvolto.", "impatto"))
	} else {
		items = append(items, cockpitReadyItem("impact", "Impatto", cockpitImpactReadinessSummary(detail), "impatto"))
	}

	unresolved := unresolvedAudienceCount(detail)
	if unresolved > 0 {
		items = append(items, cockpitBlockingItem("audience", "Audience servizi", strconv.Itoa(unresolved)+" servizi richiedono audience esplicita.", "impatto"))
	} else {
		items = append(items, cockpitReadyItem("audience", "Audience servizi", "Audience coerente con il catalogo.", "impatto"))
	}

	switch {
	case hasNoticeWithStatus(detail, "sent"):
		items = append(items, cockpitReadyItem("communications", "Comunicazioni", "Almeno una comunicazione inviata.", "comunicazioni"))
	case hasNoticeWithStatus(detail, "ready"):
		items = append(items, cockpitWarningItem("communications", "Comunicazioni", "Comunicazione pronta, non ancora inviata.", "comunicazioni"))
	case len(detail.Notices) > 0:
		items = append(items, cockpitWarningItem("communications", "Comunicazioni", "Comunicazioni in bozza.", "comunicazioni"))
	default:
		items = append(items, cockpitWarningItem("communications", "Comunicazioni", "Nessuna comunicazione registrata.", "comunicazioni"))
	}

	return items
}

func cockpitReadyItem(key, label, summary, targetTab string) cockpitReadinessItem {
	return cockpitReadinessItem{Key: key, Label: label, Status: "ready", Summary: summary, TargetTab: targetTab}
}

func cockpitBlockingItem(key, label, summary, targetTab string) cockpitReadinessItem {
	return cockpitReadinessItem{Key: key, Label: label, Status: "blocking", Summary: summary, TargetTab: targetTab, Blocking: true}
}

func cockpitWarningItem(key, label, summary, targetTab string) cockpitReadinessItem {
	return cockpitReadinessItem{Key: key, Label: label, Status: "warning", Summary: summary, TargetTab: targetTab}
}

func chronologicalOperationalWindows(windows []MaintenanceWindow) []MaintenanceWindow {
	items := make([]MaintenanceWindow, 0, len(windows))
	for _, window := range windows {
		if isOperationalWindowStatus(window.WindowStatus) {
			items = append(items, window)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].ScheduledStartAt.Equal(items[j].ScheduledStartAt) {
			return items[i].ScheduledStartAt.Before(items[j].ScheduledStartAt)
		}
		return items[i].SeqNo < items[j].SeqNo
	})
	return items
}

func isOperationalWindowStatus(status string) bool {
	return status != "cancelled" && status != "superseded"
}

func nextOperationalWindowIndex(windows []MaintenanceWindow, now time.Time) int {
	for index, window := range windows {
		if !window.ScheduledEndAt.Before(now) {
			return index
		}
	}
	return 0
}

func unresolvedAudienceCount(detail MaintenanceDetail) int {
	count := 0
	for _, item := range detail.ServiceTaxonomy {
		if item.Reference.Audience != nil && *item.Reference.Audience == "maintenance" && item.ExpectedAudience == nil {
			count++
		}
	}
	return count
}

func hasNoticeWithStatus(detail MaintenanceDetail, status string) bool {
	for _, notice := range detail.Notices {
		if notice.SendStatus == status {
			return true
		}
	}
	return false
}

func cockpitImpactReadinessSummary(detail MaintenanceDetail) string {
	parts := []string{}
	if len(detail.ServiceTaxonomy) > 0 {
		parts = append(parts, strconv.Itoa(len(detail.ServiceTaxonomy))+" servizi")
	}
	if len(detail.Targets) > 0 {
		parts = append(parts, strconv.Itoa(len(detail.Targets))+" target")
	}
	return strings.Join(parts, " · ")
}

func cockpitImpactFrom(detail MaintenanceDetail, dependencies []ServiceDependency) cockpitImpact {
	operated := []ClassificationItem{}
	dependent := []ClassificationItem{}
	for _, item := range detail.ServiceTaxonomy {
		if item.Role != nil && *item.Role == "dependent" {
			dependent = append(dependent, item)
			continue
		}
		operated = append(operated, item)
	}
	return cockpitImpact{
		OperatedServices:  operated,
		DependentServices: dependent,
		Targets:           detail.Targets,
		Customers:         detail.ImpactedCustomers,
		Dependencies:      dependencies,
		Summary: cockpitImpactSummary{
			Services:     len(detail.ServiceTaxonomy),
			Targets:      len(detail.Targets),
			Customers:    len(detail.ImpactedCustomers),
			Dependencies: len(dependencies),
		},
	}
}

func cockpitTimeline(detail MaintenanceDetail) []cockpitTimelineItem {
	items := []cockpitTimelineItem{}
	for _, window := range detail.Windows {
		start := window.ScheduledStartAt
		end := window.ScheduledEndAt
		items = append(items, cockpitTimelineItem{
			ID:        "window-" + strconv.FormatInt(window.MaintenanceWindowID, 10),
			Kind:      "window",
			Label:     "Finestra " + strconv.Itoa(window.SeqNo),
			Status:    window.WindowStatus,
			StartAt:   &start,
			EndAt:     &end,
			TargetTab: "finestre",
		})
	}
	for _, notice := range detail.Notices {
		eventAt := notice.CreatedAt
		if notice.SentAt != nil {
			eventAt = *notice.SentAt
		}
		items = append(items, cockpitTimelineItem{
			ID:        "notice-" + strconv.FormatInt(notice.NoticeID, 10),
			Kind:      "notice",
			Label:     notice.NoticeType,
			Status:    notice.SendStatus,
			EventAt:   &eventAt,
			TargetTab: "comunicazioni",
		})
	}
	for _, event := range detail.Events {
		eventAt := event.EventAt
		summary := ""
		if event.Summary != nil {
			summary = *event.Summary
		}
		items = append(items, cockpitTimelineItem{
			ID:        "event-" + strconv.FormatInt(event.MaintenanceEventID, 10),
			Kind:      "event",
			Label:     event.EventType,
			Status:    event.ActorType,
			EventAt:   &eventAt,
			Summary:   summary,
			TargetTab: "storico",
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		leftBucket := cockpitTimelineBucket(items[i])
		rightBucket := cockpitTimelineBucket(items[j])
		if leftBucket != rightBucket {
			return leftBucket < rightBucket
		}
		leftTime := cockpitTimelineTime(items[i])
		rightTime := cockpitTimelineTime(items[j])
		if leftBucket == 2 {
			return leftTime.After(rightTime)
		}
		if !leftTime.Equal(rightTime) {
			return leftTime.Before(rightTime)
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func cockpitTimelineBucket(item cockpitTimelineItem) int {
	if item.Kind != "window" {
		return 2
	}
	if isOperationalWindowStatus(item.Status) {
		return 0
	}
	return 1
}

func cockpitTimelineTime(item cockpitTimelineItem) time.Time {
	if item.EventAt != nil {
		return *item.EventAt
	}
	if item.StartAt != nil {
		return *item.StartAt
	}
	return time.Time{}
}

func (h *Handler) loadCockpitDependencies(ctx context.Context, detail MaintenanceDetail) ([]ServiceDependency, error) {
	ids := selectedServiceIDs(detail)
	if len(ids) == 0 {
		return []ServiceDependency{}, nil
	}
	args := make([]any, 0, len(ids))
	holders := make([]string, 0, len(ids))
	for _, id := range ids {
		holders = append(holders, placeholder(&args, id))
	}
	rows, err := h.maintenance.QueryContext(
		ctx,
		serviceDependencySelect()+` WHERE sd.is_active = true AND sd.upstream_service_id IN (`+strings.Join(holders, ", ")+`)`+serviceDependencyOrder(),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ServiceDependency{}
	for rows.Next() {
		item, err := scanServiceDependency(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func selectedServiceIDs(detail MaintenanceDetail) []int64 {
	seen := map[int64]struct{}{}
	ids := []int64{}
	for _, item := range detail.ServiceTaxonomy {
		if item.Role != nil && *item.Role == "dependent" {
			continue
		}
		id := item.Reference.ID
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
