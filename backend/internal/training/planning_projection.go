package training

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
)

// PlanningSnapshot retains only local facts. Projection never calls a remote service.
type PlanningSnapshot struct {
	Courses            []planningCourse
	Requests           []planningRequest
	Events             []planningEvent
	Enrollments        []planningEnrollment
	Expenses           []planningExpense
	ExpenseEnrollments []planningExpenseEnrollment
	CurrentTeams       map[string][]PlanningRef
}
type planningCourse struct {
	ID, Title        string
	Tags             []string
	Areas            []PlanningRef
	Suspended        bool
	SuspensionReason *string
	ReminderText     string
	ReminderDate     *string
}
type planningRequest struct {
	ID, CourseID, CourseTitle              string
	Employee, Team                         PlanningRef
	Priority                               *int
	CreatedAt                              string
	Areas                                  []PlanningArea
	TLOpinion, PeopleDecision, Outcome     *string
	Suspended                              bool
	ReminderText                           string
	ReminderDate                           *string
	AcceptedCourse                         *PlanningRef
	AcceptedEventID, ResultingEnrollmentID *string
}
type planningEvent struct {
	ID, CourseID, Title, Origin                                 string
	Cancelled                                                   bool
	SessionsCount                                               int
	StartsAt, EndsAt, DueOn                                     *string
	WithoutSessions, UnassignedEnrollments, NeedsReconciliation bool
	ReminderText                                                string
	ReminderDate                                                *string
}
type planningEnrollment struct {
	ID, EmployeeID, EventID, DeliveryStatus string
	Employee                                PlanningRef
	SourceRequestID                         *string
}
type planningExpense struct {
	ID, EventID string
	POID        int64
}
type planningExpenseEnrollment struct{ ExpenseID, EnrollmentID string }

func planningToday(now time.Time) (string, error) {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		return "", err
	}
	return now.In(loc).Format("2006-01-02"), nil
}
func planningOperationalEvent(e planningEvent, es []planningEnrollment) bool {
	if e.Cancelled {
		return false
	}
	if len(es) == 0 {
		return true
	}
	for _, x := range es {
		if x.DeliveryStatus == deliveryPlanned || x.DeliveryStatus == deliveryInProgress {
			return true
		}
	}
	return false
}
func planningReminder(kind, id, label, courseID, text string, date *string, operative bool, today string) *PlanningReminder {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	timing := "undated"
	if date != nil {
		if *date < today {
			timing = "overdue"
		} else if *date == today {
			timing = "today"
		} else {
			timing = "future"
		}
	}
	return &PlanningReminder{OwnerKind: kind, OwnerID: id, OwnerLabel: label, CourseID: courseID, Text: strings.TrimSpace(text), Date: date, Operative: operative, Timing: timing}
}
func planningDeliveryCounts(es []planningEnrollment) (c PlanningDeliveryCounts) {
	for _, e := range es {
		switch e.DeliveryStatus {
		case deliveryPlanned:
			c.Planned++
		case deliveryInProgress:
			c.InProgress++
		case deliveryCompleted:
			c.Completed++
		case deliveryPartiallyCompleted:
			c.PartiallyCompleted++
		case deliveryNotAttended:
			c.NotAttended++
		case deliveryCancelled:
			c.Cancelled++
		}
	}
	return
}

func planningRefLess(a, b PlanningRef) bool {
	an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name)
	if an != bn {
		return an < bn
	}
	return a.ID < b.ID
}

func sortedPlanningRefs(refs []PlanningRef) []PlanningRef {
	out := append([]PlanningRef{}, refs...)
	sort.Slice(out, func(i, j int) bool { return planningRefLess(out[i], out[j]) })
	return out
}

func sortedPlanningAreas(areas []PlanningArea) []PlanningArea {
	out := append([]PlanningArea{}, areas...)
	sort.Slice(out, func(i, j int) bool {
		return planningRefLess(out[i].PlanningRef, out[j].PlanningRef)
	})
	return out
}

func planningIndexes(ctx context.Context, s PlanningSnapshot) (map[string]planningCourse, map[string][]planningRequest, map[string][]planningEvent, map[string][]planningEnrollment, map[string]planningEvent, error) {
	cs := map[string]planningCourse{}
	rs := map[string][]planningRequest{}
	es := map[string][]planningEvent{}
	ens := map[string][]planningEnrollment{}
	eby := map[string]planningEvent{}
	for i, c := range s.Courses {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, nil, nil, nil, nil, ctx.Err()
		}
		cs[c.ID] = c
	}
	for i, r := range s.Requests {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, nil, nil, nil, nil, ctx.Err()
		}
		rs[r.CourseID] = append(rs[r.CourseID], r)
	}
	for i, e := range s.Events {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, nil, nil, nil, nil, ctx.Err()
		}
		es[e.CourseID] = append(es[e.CourseID], e)
		eby[e.ID] = e
	}
	for i, e := range s.Enrollments {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, nil, nil, nil, nil, ctx.Err()
		}
		ens[e.EventID] = append(ens[e.EventID], e)
	}
	return cs, rs, es, ens, eby, nil
}
func planningReminders(c planningCourse, rs []planningRequest, es []planningEvent, today string) []PlanningReminder {
	out := []PlanningReminder{}
	if r := planningReminder("course", c.ID, c.Title, c.ID, c.ReminderText, c.ReminderDate, !c.Suspended, today); r != nil {
		out = append(out, *r)
	}
	for _, x := range rs {
		if r := planningReminder("request", x.ID, x.Employee.Name, c.ID, x.ReminderText, x.ReminderDate, !x.Suspended && !c.Suspended, today); r != nil {
			out = append(out, *r)
		}
	}
	for _, x := range es {
		if r := planningReminder("event", x.ID, x.Title, c.ID, x.ReminderText, x.ReminderDate, true, today); r != nil {
			out = append(out, *r)
		}
	}
	return out
}

// projectPlanningCourses requires every PO referenced by the candidate snapshot to
// be classified. Missing data is an error, never silently represented as pending.
func projectPlanningCourses(ctx context.Context, s PlanningSnapshot, po map[int64]string, today string) ([]CourseSummary, error) {
	cs, rs, es, ens, eby, err := planningIndexes(ctx, s)
	if err != nil {
		return nil, err
	}
	_ = cs
	exps := map[string][]planningExpense{}
	for i, x := range s.Expenses {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if e, ok := eby[x.EventID]; ok {
			exps[e.CourseID] = append(exps[e.CourseID], x)
		}
	}
	coverageByExpense := map[string][]planningExpenseEnrollment{}
	for i, link := range s.ExpenseEnrollments {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		coverageByExpense[link.ExpenseID] = append(coverageByExpense[link.ExpenseID], link)
	}
	out := make([]CourseSummary, 0, len(s.Courses))
	for i, c := range s.Courses {
		if i%128 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		rset, eset := rs[c.ID], es[c.ID]
		x := CourseSummary{ID: c.ID, Title: c.Title, Tags: append([]string{}, c.Tags...), Areas: sortedPlanningRefs(c.Areas), CourseSuspended: c.Suspended, SuspensionReason: c.SuspensionReason, Reasons: []string{}, EnrollmentsByStatus: PlanningDeliveryCounts{}}
		people, enrolled := map[string]bool{}, map[string]bool{}
		suspendedOpen := false
		var operativePriority, suspendedPriority *int
		for _, r := range rset {
			people[r.Employee.ID] = true
			open := r.Outcome == nil
			operative := open && !r.Suspended && !c.Suspended
			suspended := open && (r.Suspended || c.Suspended)
			if operative {
				x.OperativeRequestsCount++
				if r.Priority != nil && (operativePriority == nil || *r.Priority < *operativePriority) {
					v := *r.Priority
					operativePriority = &v
				}
			}
			if suspended {
				x.SuspendedRequestsCount++
				suspendedOpen = true
				if r.Priority != nil && (suspendedPriority == nil || *r.Priority < *suspendedPriority) {
					v := *r.Priority
					suspendedPriority = &v
				}
			}
		}
		x.RequestsCount = len(rset)
		x.Priority = operativePriority
		x.EventsCount = len(eset)
		for _, e := range eset {
			set := ens[e.ID]
			if planningOperationalEvent(e, set) {
				x.OperativeEventsCount++
			}
			d := planningDeliveryCounts(set)
			x.EnrollmentsCount += len(set)
			x.EnrollmentsByStatus.Planned += d.Planned
			x.EnrollmentsByStatus.InProgress += d.InProgress
			x.EnrollmentsByStatus.Completed += d.Completed
			x.EnrollmentsByStatus.PartiallyCompleted += d.PartiallyCompleted
			x.EnrollmentsByStatus.NotAttended += d.NotAttended
			x.EnrollmentsByStatus.Cancelled += d.Cancelled
			for _, en := range set {
				people[en.EmployeeID] = true
				enrolled[en.EmployeeID] = true
			}
		}
		x.PeopleCount = len(people)
		x.EnrolledPeopleCount = len(enrolled)
		x.SuspendedWork = c.Suspended || suspendedOpen
		poids, expensePO, coverage := map[int64]bool{}, map[string]int64{}, map[string][]int64{}
		for _, e := range exps[c.ID] {
			poids[e.POID] = true
			expensePO[e.ID] = e.POID
		}
		for expenseID, id := range expensePO {
			for _, link := range coverageByExpense[expenseID] {
				coverage[link.EnrollmentID] = append(coverage[link.EnrollmentID], id)
			}
		}
		x.Economic.DistinctPOCount = len(poids)
		pending := false
		for id := range poids {
			state, ok := po[id]
			if !ok {
				return nil, fmt.Errorf("planning PO %d has no classification", id)
			}
			switch state {
			case "approved":
				x.Economic.Approved++
			case "rejected":
				x.Economic.Rejected++
				pending = true
			default:
				x.Economic.Pending++
				pending = true
			}
		}
		for _, ids := range coverage {
			x.Economic.CoveredEnrollments++
			all := true
			for _, id := range ids {
				if po[id] != "approved" {
					all = false
				}
			}
			if all {
				x.Economic.ApprovedCoveredEnrollments++
			}
		}
		reminders := planningReminders(c, rset, eset, today)
		x.PrimaryReminder = lensReminder(reminders, "operative", today)
		for _, r := range reminders {
			x.ReminderCount++
			if r.Operative {
				x.OperativeReminderCount++
				if r.Date != nil && *r.Date <= today {
					x.DueReminderCount++
				}
			}
		}
		x.Operative = x.OperativeRequestsCount > 0 || x.OperativeEventsCount > 0 || x.OperativeReminderCount > 0 || pending
		if x.OperativeRequestsCount > 0 {
			x.Reasons = append(x.Reasons, "requests")
		}
		if x.OperativeEventsCount > 0 {
			x.Reasons = append(x.Reasons, "events")
		}
		if x.OperativeReminderCount > 0 {
			x.Reasons = append(x.Reasons, "reminders")
		}
		if pending {
			x.Reasons = append(x.Reasons, "expenses")
		}
		x.History = !x.Operative && !x.SuspendedWork && (x.RequestsCount > 0 || x.EventsCount > 0)
		// Suspended priority is selected later with the lens, but retain it locally.
		_ = suspendedPriority
		out = append(out, x)
	}
	return out, nil
}
func reminderLess(a, b PlanningReminder) bool {
	rank := func(x PlanningReminder) int {
		if x.Date == nil {
			return 2
		}
		if x.Timing == "future" {
			return 1
		}
		return 0
	}
	if rank(a) != rank(b) {
		return rank(a) < rank(b)
	}
	ad, bd := "", ""
	if a.Date != nil {
		ad = *a.Date
	}
	if b.Date != nil {
		bd = *b.Date
	}
	if ad != bd {
		return ad < bd
	}
	if a.OwnerKind != b.OwnerKind {
		return a.OwnerKind < b.OwnerKind
	}
	return a.OwnerID < b.OwnerID
}
func lensReminder(reminders []PlanningReminder, view, today string) *PlanningReminder {
	candidates := []PlanningReminder{}
	for _, r := range reminders {
		switch view {
		case "operative":
			if r.Operative {
				candidates = append(candidates, r)
			}
		case "reminders":
			if r.Operative && r.Date != nil && *r.Date <= today {
				candidates = append(candidates, r)
			}
		case "suspended":
			if !r.Operative {
				candidates = append(candidates, r)
			}
		}
	}
	if view == "history" || len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return reminderLess(candidates[i], candidates[j]) })
	return &candidates[0]
}
func planningForLens(ctx context.Context, items []CourseSummary, s PlanningSnapshot, today, view string) ([]CourseSummary, error) {
	cs, rs, es, _, _, err := planningIndexes(ctx, s)
	if err != nil {
		return nil, err
	}
	out := make([]CourseSummary, 0, len(items))
	for i, x := range items {
		if i%128 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		c := cs[x.ID]
		x.PrimaryReminder = lensReminder(planningReminders(c, rs[x.ID], es[x.ID], today), view, today)
		if view == "suspended" {
			var p *int
			for _, r := range rs[x.ID] {
				if r.Outcome == nil && (r.Suspended || c.Suspended) && r.Priority != nil && (p == nil || *r.Priority < *p) {
					v := *r.Priority
					p = &v
				}
			}
			x.Priority = p
		}
		if view == "history" {
			x.Priority = nil
		}
		out = append(out, x)
	}
	return out, nil
}

func filterPlanningCourses(ctx context.Context, items []CourseSummary, f PlanningListFilter, s PlanningSnapshot) ([]CourseSummary, error) {
	_, requests, events, enrollments, _, err := planningIndexes(ctx, s)
	if err != nil {
		return nil, err
	}
	out := []CourseSummary{}
	q := strings.ToLower(strings.TrimSpace(f.Q))
	for i, x := range items {
		if i%128 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		tagOK, areaOK := f.Tag == "", f.SkillAreaID == ""
		searchable := x.Title
		for _, t := range x.Tags {
			searchable += " " + t
			if t == f.Tag {
				tagOK = true
			}
		}
		for _, a := range x.Areas {
			searchable += " " + a.Name
			if a.ID == f.SkillAreaID {
				areaOK = true
			}
		}
		matching := map[string]bool{}
		for j, r := range requests[x.ID] {
			if j%128 == 0 && ctx.Err() != nil {
				return nil, ctx.Err()
			}
			searchable += " " + r.Employee.Name + " " + r.Team.Name
			teamOK := f.TeamID == "" || r.Team.ID == f.TeamID
			if (f.EmployeeID == "" || f.EmployeeID == r.Employee.ID) && teamOK {
				matching[r.Employee.ID] = true
			}
			for _, a := range r.Areas {
				searchable += " " + a.Name
				if a.ID == f.SkillAreaID {
					areaOK = true
				}
			}
		}
		for _, event := range events[x.ID] {
			for j, en := range enrollments[event.ID] {
				if j%128 == 0 && ctx.Err() != nil {
					return nil, ctx.Err()
				}
				searchable += " " + en.Employee.Name
				teamOK := f.TeamID == ""
				for _, t := range s.CurrentTeams[en.EmployeeID] {
					searchable += " " + t.Name
					if t.ID == f.TeamID {
						teamOK = true
					}
				}
				if (f.EmployeeID == "" || f.EmployeeID == en.EmployeeID) && teamOK {
					matching[en.EmployeeID] = true
				}
			}
		}
		if !tagOK || !areaOK || ((f.EmployeeID != "" || f.TeamID != "") && len(matching) == 0) || (q != "" && !strings.Contains(strings.ToLower(searchable), q)) {
			continue
		}
		out = append(out, x)
	}
	return out, nil
}

// sortPlanningCourses only lets reminders due today or earlier establish the
// first bucket. Future and undated reminders remain in the DTO for display, but
// sort with courses that have no due reminder.
func sortPlanningCourses(items []CourseSummary, today string) {
	sort.SliceStable(items, func(i, j int) bool {
		date := func(x CourseSummary) string {
			if x.PrimaryReminder == nil || x.PrimaryReminder.Date == nil || *x.PrimaryReminder.Date > today {
				return ""
			}
			return *x.PrimaryReminder.Date
		}
		di, dj := date(items[i]), date(items[j])
		if di != dj {
			if di == "" {
				return false
			}
			if dj == "" {
				return true
			}
			return di < dj
		}
		if items[i].Priority != nil || items[j].Priority != nil {
			if items[i].Priority == nil {
				return false
			}
			if items[j].Priority == nil {
				return true
			}
			if *items[i].Priority != *items[j].Priority {
				return *items[i].Priority < *items[j].Priority
			}
		}
		a, b := strings.ToLower(items[i].Title), strings.ToLower(items[j].Title)
		if a == b {
			return items[i].ID < items[j].ID
		}
		return a < b
	})
}
func paginatePlanning[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

// PlanningCandidateSnapshot is the phase-two boundary: its snapshot contains
// complete facts owned by locally filtered courses plus only external requests
// explicitly linked to their enrollments. POIDs is the exact remote hydration set.
type PlanningCandidateSnapshot struct {
	Snapshot PlanningSnapshot
	POIDs    []int64
}

// selectPlanningCandidates applies all local list filters before PO hydration.
func selectPlanningCandidates(ctx context.Context, s PlanningSnapshot, f PlanningListFilter) (PlanningCandidateSnapshot, error) {
	local := make([]CourseSummary, 0, len(s.Courses))
	for i, c := range s.Courses {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		local = append(local, CourseSummary{ID: c.ID, Title: c.Title, Tags: c.Tags, Areas: c.Areas})
	}
	selected, err := filterPlanningCourses(ctx, local, f, s)
	if err != nil {
		return PlanningCandidateSnapshot{}, err
	}
	ids := make(map[string]bool, len(selected))
	for _, c := range selected {
		ids[c.ID] = true
	}
	return restrictPlanningSnapshot(ctx, s, ids)
}

// restrictPlanningSnapshot retains complete owned facts for candidate courses.
// External requests are retained only when an explicit enrollment link requires
// resolving it; their course and facts are never introduced into owned courses.
func restrictPlanningSnapshot(ctx context.Context, s PlanningSnapshot, courseIDs map[string]bool) (PlanningCandidateSnapshot, error) {
	out := PlanningSnapshot{Courses: []planningCourse{}, Requests: []planningRequest{}, Events: []planningEvent{}, Enrollments: []planningEnrollment{}, Expenses: []planningExpense{}, ExpenseEnrollments: []planningExpenseEnrollment{}, CurrentTeams: map[string][]PlanningRef{}}
	for i, c := range s.Courses {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		if courseIDs[c.ID] {
			out.Courses = append(out.Courses, c)
		}
	}
	eventIDs := map[string]bool{}
	for i, e := range s.Events {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		if courseIDs[e.CourseID] {
			out.Events = append(out.Events, e)
			eventIDs[e.ID] = true
		}
	}
	enrollmentIDs, requestIDs := map[string]bool{}, map[string]bool{}
	for i, en := range s.Enrollments {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		if eventIDs[en.EventID] {
			out.Enrollments = append(out.Enrollments, en)
			enrollmentIDs[en.ID] = true
			if en.SourceRequestID != nil {
				requestIDs[*en.SourceRequestID] = true
			}
		}
	}
	for i, r := range s.Requests {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		if courseIDs[r.CourseID] || requestIDs[r.ID] || (r.ResultingEnrollmentID != nil && enrollmentIDs[*r.ResultingEnrollmentID]) {
			out.Requests = append(out.Requests, r)
		}
	}
	expenseIDs, poIDs := map[string]bool{}, map[int64]bool{}
	for i, x := range s.Expenses {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		if eventIDs[x.EventID] {
			out.Expenses = append(out.Expenses, x)
			expenseIDs[x.ID] = true
			poIDs[x.POID] = true
		}
	}
	for i, x := range s.ExpenseEnrollments {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		if expenseIDs[x.ExpenseID] {
			out.ExpenseEnrollments = append(out.ExpenseEnrollments, x)
		}
	}
	people := map[string]bool{}
	for i, r := range out.Requests {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		people[r.Employee.ID] = true
	}
	for i, en := range out.Enrollments {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningCandidateSnapshot{}, ctx.Err()
		}
		people[en.EmployeeID] = true
	}
	for id := range people {
		out.CurrentTeams[id] = append([]PlanningRef{}, s.CurrentTeams[id]...)
	}
	ids := make([]int64, 0, len(poIDs))
	for id := range poIDs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if err := ctx.Err(); err != nil {
		return PlanningCandidateSnapshot{}, err
	}
	return PlanningCandidateSnapshot{Snapshot: out, POIDs: ids}, nil
}

// projectPlanningList filters locally, then requires PO states only for the
// resulting candidate snapshot before applying lens/order/page.
func projectPlanningList(ctx context.Context, candidate PlanningCandidateSnapshot, po map[int64]string, today, generatedAt string, f PlanningListFilter) (PlanningListResponse, error) {
	all, err := projectPlanningCourses(ctx, candidate.Snapshot, po, today)
	if err != nil {
		return PlanningListResponse{}, err
	}
	candidates := all
	due := 0
	for i, x := range candidates {
		if i%128 == 0 && ctx.Err() != nil {
			return PlanningListResponse{}, ctx.Err()
		}
		if x.DueReminderCount > 0 {
			due++
		}
	}
	selected := []CourseSummary{}
	for i, x := range candidates {
		if i%128 == 0 && ctx.Err() != nil {
			return PlanningListResponse{}, ctx.Err()
		}
		if (f.View == "operative" && x.Operative) || (f.View == "reminders" && x.DueReminderCount > 0) || (f.View == "suspended" && x.SuspendedWork) || (f.View == "history" && x.History) {
			selected = append(selected, x)
		}
	}
	selected, err = planningForLens(ctx, selected, candidate.Snapshot, today, f.View)
	if err != nil {
		return PlanningListResponse{}, err
	}
	sortPlanningCourses(selected, today)
	return PlanningListResponse{Today: today, GeneratedAt: generatedAt, Items: paginatePlanning(selected, f.Limit, f.Offset), Total: len(selected), Limit: f.Limit, Offset: f.Offset, DueCoursesTotal: due}, nil
}
func projectPlanningFilters(ctx context.Context, s PlanningSnapshot) (PlanningFiltersResponse, error) {
	relevant := map[string]bool{}
	for _, c := range s.Courses {
		if c.Suspended || strings.TrimSpace(c.ReminderText) != "" {
			relevant[c.ID] = true
		}
	}
	for i, r := range s.Requests {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningFiltersResponse{}, ctx.Err()
		}
		relevant[r.CourseID] = true
	}
	for i, e := range s.Events {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningFiltersResponse{}, ctx.Err()
		}
		relevant[e.CourseID] = true
	}
	tags := map[string]bool{}
	people, teams, areas := map[string]PlanningRef{}, map[string]PlanningRef{}, map[string]PlanningRef{}
	for _, c := range s.Courses {
		if !relevant[c.ID] {
			continue
		}
		for _, t := range c.Tags {
			tags[t] = true
		}
		for _, a := range c.Areas {
			areas[a.ID] = a
		}
	}
	for i, r := range s.Requests {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningFiltersResponse{}, ctx.Err()
		}
		if relevant[r.CourseID] {
			people[r.Employee.ID] = r.Employee
			teams[r.Team.ID] = r.Team
			for _, a := range r.Areas {
				areas[a.ID] = a.PlanningRef
			}
		}
	}
	eventCourse := map[string]string{}
	for _, e := range s.Events {
		eventCourse[e.ID] = e.CourseID
	}
	for i, e := range s.Enrollments {
		if i%256 == 0 && ctx.Err() != nil {
			return PlanningFiltersResponse{}, ctx.Err()
		}
		if !relevant[eventCourse[e.EventID]] {
			continue
		}
		people[e.EmployeeID] = e.Employee
		for _, t := range s.CurrentTeams[e.EmployeeID] {
			teams[t.ID] = t
		}
	}
	out := PlanningFiltersResponse{Tags: []string{}, People: []PlanningRef{}, Teams: []PlanningRef{}, Areas: []PlanningRef{}}
	for x := range tags {
		out.Tags = append(out.Tags, x)
	}
	for _, m := range []map[string]PlanningRef{people, teams, areas} {
		dst := &out.People
		if len(out.People) > 0 {
			_ = dst
		}
		_ = m
	}
	for _, x := range people {
		out.People = append(out.People, x)
	}
	for _, x := range teams {
		out.Teams = append(out.Teams, x)
	}
	for _, x := range areas {
		out.Areas = append(out.Areas, x)
	}
	sort.Strings(out.Tags)
	out.People = sortedPlanningRefs(out.People)
	out.Teams = sortedPlanningRefs(out.Teams)
	out.Areas = sortedPlanningRefs(out.Areas)
	if err := ctx.Err(); err != nil {
		return PlanningFiltersResponse{}, err
	}
	return out, nil
}
func planningRequestItem(r planningRequest, courses map[string]planningCourse, courseSuspended bool) PlanningRequestItem {
	c := courses[r.CourseID]
	if c.Title == "" {
		c = planningCourse{ID: r.CourseID, Title: r.CourseTitle}
	}
	return PlanningRequestItem{ID: r.ID, Employee: r.Employee, Team: r.Team, Course: PlanningRef{ID: c.ID, Name: c.Title}, Priority: r.Priority, CreatedAt: r.CreatedAt, Areas: sortedPlanningAreas(r.Areas), TLOpinion: r.TLOpinion, PeopleDecision: r.PeopleDecision, Outcome: r.Outcome, Suspended: r.Suspended || courseSuspended, Operative: r.Outcome == nil && !r.Suspended && !courseSuspended, AcceptedCourse: r.AcceptedCourse, AcceptedEventID: r.AcceptedEventID, ResultingEnrollmentID: r.ResultingEnrollmentID}
}
func planningEventItem(e planningEvent, ens []planningEnrollment) PlanningEventItem {
	return PlanningEventItem{ID: e.ID, Title: e.Title, CourseID: e.CourseID, Origin: e.Origin, Cancelled: e.Cancelled, Operative: planningOperationalEvent(e, ens), SessionsCount: e.SessionsCount, StartsAt: e.StartsAt, EndsAt: e.EndsAt, DueOn: e.DueOn, EnrollmentsCount: len(ens), EnrollmentsByStatus: planningDeliveryCounts(ens), WithoutSessions: e.WithoutSessions, UnassignedEnrollments: e.UnassignedEnrollments, NeedsReconciliation: e.NeedsReconciliation}
}

// planningScopedRequests exposes only requests owned by the course. The snapshot
// may retain externally owned requests to resolve enrollment links, but those
// requests must never become course requests, reminders, previews, or teams.
func planningScopedRequests(ctx context.Context, s PlanningSnapshot, courseID string) ([]planningRequest, error) {
	out := []planningRequest{}
	for i, r := range s.Requests {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if r.CourseID == courseID {
			out = append(out, r)
		}
	}
	return out, nil
}
func requestLess(a, b PlanningRequestItem) bool {
	rank := func(x PlanningRequestItem) int {
		if x.Operative {
			return 0
		}
		// Suspension is suspended work only while the request is open. A closed
		// accepted/rejected/withdrawn request keeps its outcome and ranks closed.
		if x.Outcome == nil && x.Suspended {
			return 1
		}
		return 2
	}
	if rank(a) != rank(b) {
		return rank(a) < rank(b)
	}
	if a.Priority != nil || b.Priority != nil {
		if a.Priority == nil {
			return false
		}
		if b.Priority == nil {
			return true
		}
		if *a.Priority != *b.Priority {
			return *a.Priority < *b.Priority
		}
	}
	if a.CreatedAt != b.CreatedAt {
		return a.CreatedAt > b.CreatedAt
	}
	return a.ID < b.ID
}
func eventLess(a, b PlanningEventItem) bool {
	rank := func(x PlanningEventItem) int {
		if x.Operative {
			return 0
		}
		if !x.Cancelled {
			return 1
		}
		return 2
	}
	if rank(a) != rank(b) {
		return rank(a) < rank(b)
	}
	date := func(x PlanningEventItem) string {
		if x.StartsAt != nil {
			return *x.StartsAt
		}
		if x.DueOn != nil {
			return *x.DueOn
		}
		return ""
	}
	ad, bd := date(a), date(b)
	if ad != bd {
		if ad == "" {
			return false
		}
		if bd == "" {
			return true
		}
		return ad < bd
	}
	return a.ID < b.ID
}
func projectPlanningCourseDetail(ctx context.Context, s PlanningSnapshot, po map[int64]string, today, courseID string) (PlanningCourseDetail, error) {
	summaries, err := projectPlanningCourses(ctx, s, po, today)
	if err != nil {
		return PlanningCourseDetail{}, err
	}
	var summary *CourseSummary
	for i := range summaries {
		if i%128 == 0 && ctx.Err() != nil {
			return PlanningCourseDetail{}, ctx.Err()
		}
		if summaries[i].ID == courseID {
			summary = &summaries[i]
			break
		}
	}
	if summary == nil {
		return PlanningCourseDetail{}, notFoundError("course_not_found", "corso non trovato")
	}
	cs, _, es, ens, _, err := planningIndexes(ctx, s)
	if err != nil {
		return PlanningCourseDetail{}, err
	}
	reqs, err := planningScopedRequests(ctx, s, courseID)
	if err != nil {
		return PlanningCourseDetail{}, err
	}
	ri := make([]PlanningRequestItem, 0, len(reqs))
	for _, r := range reqs {
		ri = append(ri, planningRequestItem(r, cs, cs[r.CourseID].Suspended))
	}
	sort.Slice(ri, func(i, j int) bool { return requestLess(ri[i], ri[j]) })
	ei := []PlanningEventItem{}
	eventOptions := []PlanningRef{}
	teamSet := map[string]PlanningRef{}
	for _, e := range es[courseID] {
		ei = append(ei, planningEventItem(e, ens[e.ID]))
		eventOptions = append(eventOptions, PlanningRef{ID: e.ID, Name: e.Title})
	}
	for _, r := range reqs {
		teamSet[r.Team.ID] = r.Team
	}
	for i, en := range s.Enrollments {
		if i%128 == 0 && ctx.Err() != nil {
			return PlanningCourseDetail{}, ctx.Err()
		}
		for _, e := range es[courseID] {
			if en.EventID == e.ID {
				for _, t := range s.CurrentTeams[en.EmployeeID] {
					teamSet[t.ID] = t
				}
			}
		}
	}
	teams := []PlanningRef{}
	for _, t := range teamSet {
		teams = append(teams, t)
	}
	sort.Slice(ei, func(i, j int) bool { return eventLess(ei[i], ei[j]) })
	eventOptions = sortedPlanningRefs(eventOptions)
	teams = sortedPlanningRefs(teams)
	return PlanningCourseDetail{Today: today, Course: *summary, RequestsPreview: paginatePlanning(ri, 5, 0), EventsPreview: paginatePlanning(ei, 3, 0), EventOptions: eventOptions, TeamOptions: teams}, nil
}

func projectPlanningItems(ctx context.Context, s PlanningSnapshot, today, courseID string, f PlanningItemsFilter) (PlanningItemsResponse, error) {
	cs, _, es, ens, _, err := planningIndexes(ctx, s)
	if err != nil {
		return PlanningItemsResponse{}, err
	}
	if _, ok := cs[courseID]; !ok {
		return PlanningItemsResponse{}, notFoundError("course_not_found", "corso non trovato")
	}
	resp := PlanningItemsResponse{Today: today, Kind: f.Kind, Limit: f.Limit, Offset: f.Offset}
	switch f.Kind {
	case "requests":
		all := []PlanningRequestItem{}
		reqs, err := planningScopedRequests(ctx, s, courseID)
		if err != nil {
			return PlanningItemsResponse{}, err
		}
		for _, r := range reqs {
			all = append(all, planningRequestItem(r, cs, cs[r.CourseID].Suspended))
		}
		resp.UnfilteredTotal = len(all)
		filtered := []PlanningRequestItem{}
		for i, x := range all {
			if i%128 == 0 && ctx.Err() != nil {
				return PlanningItemsResponse{}, ctx.Err()
			}
			status := ""
			if x.Outcome != nil {
				status = *x.Outcome
			} else if x.Operative {
				status = "open"
			} else if x.Suspended {
				status = "suspended"
			}
			if (f.Q == "" || strings.Contains(strings.ToLower(x.Employee.Name), strings.ToLower(f.Q))) && (f.TeamID == "" || x.Team.ID == f.TeamID) && (f.Status == "" || status == f.Status) {
				filtered = append(filtered, x)
			}
		}
		sort.Slice(filtered, func(i, j int) bool { return requestLess(filtered[i], filtered[j]) })
		resp.Total = len(filtered)
		resp.Items = paginatePlanning(filtered, f.Limit, f.Offset)
	case "enrollments":
		all := []PlanningEnrollmentItem{}
		eventTitle := map[string]string{}
		for _, e := range es[courseID] {
			eventTitle[e.ID] = e.Title
		}
		resultingRequests := map[string][]string{}
		for i, r := range s.Requests {
			if i%256 == 0 && ctx.Err() != nil {
				return PlanningItemsResponse{}, ctx.Err()
			}
			if r.ResultingEnrollmentID != nil {
				resultingRequests[*r.ResultingEnrollmentID] = append(resultingRequests[*r.ResultingEnrollmentID], r.ID)
			}
		}
		for _, event := range es[courseID] {
			for i, en := range ens[event.ID] {
				if i%128 == 0 && ctx.Err() != nil {
					return PlanningItemsResponse{}, ctx.Err()
				}
				requestIDs := map[string]bool{}
				if en.SourceRequestID != nil {
					requestIDs[*en.SourceRequestID] = true
				}
				for _, id := range resultingRequests[en.ID] {
					requestIDs[id] = true
				}
				ids := make([]string, 0, len(requestIDs))
				for id := range requestIDs {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				all = append(all, PlanningEnrollmentItem{ID: en.ID, Employee: en.Employee, Teams: sortedPlanningRefs(s.CurrentTeams[en.EmployeeID]), Event: PlanningRef{ID: en.EventID, Name: eventTitle[en.EventID]}, DeliveryStatus: en.DeliveryStatus, RequestIDs: ids})
			}
		}
		resp.UnfilteredTotal = len(all)
		filtered := []PlanningEnrollmentItem{}
		for i, x := range all {
			if i%128 == 0 && ctx.Err() != nil {
				return PlanningItemsResponse{}, ctx.Err()
			}
			team := f.TeamID == ""
			for _, t := range x.Teams {
				if t.ID == f.TeamID {
					team = true
				}
			}
			if (f.Q == "" || strings.Contains(strings.ToLower(x.Employee.Name), strings.ToLower(f.Q))) && team && (f.EventID == "" || x.Event.ID == f.EventID) && (f.Status == "" || x.DeliveryStatus == f.Status) {
				filtered = append(filtered, x)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			a, b := strings.ToLower(filtered[i].Employee.Name), strings.ToLower(filtered[j].Employee.Name)
			if a != b {
				return a < b
			}
			if filtered[i].Event.Name != filtered[j].Event.Name {
				return filtered[i].Event.Name < filtered[j].Event.Name
			}
			return filtered[i].ID < filtered[j].ID
		})
		resp.Total = len(filtered)
		resp.Items = paginatePlanning(filtered, f.Limit, f.Offset)
	case "events":
		all := []PlanningEventItem{}
		for _, e := range es[courseID] {
			all = append(all, planningEventItem(e, ens[e.ID]))
		}
		resp.UnfilteredTotal = len(all)
		filtered := []PlanningEventItem{}
		for i, x := range all {
			if i%128 == 0 && ctx.Err() != nil {
				return PlanningItemsResponse{}, ctx.Err()
			}
			status := "terminal"
			if x.Cancelled {
				status = "cancelled"
			} else if x.Operative {
				status = "operational"
			}
			if (f.Q == "" || strings.Contains(strings.ToLower(x.Title), strings.ToLower(f.Q))) && (f.Status == "" || f.Status == status) {
				filtered = append(filtered, x)
			}
		}
		sort.Slice(filtered, func(i, j int) bool { return eventLess(filtered[i], filtered[j]) })
		resp.Total = len(filtered)
		resp.Items = paginatePlanning(filtered, f.Limit, f.Offset)
	case "reminders":
		c := cs[courseID]
		reqs, err := planningScopedRequests(ctx, s, courseID)
		if err != nil {
			return PlanningItemsResponse{}, err
		}
		all := planningReminders(c, reqs, es[courseID], today)
		resp.UnfilteredTotal = len(all)
		filtered := []PlanningReminder{}
		for i, x := range all {
			if i%128 == 0 && ctx.Err() != nil {
				return PlanningItemsResponse{}, ctx.Err()
			}
			status := x.Timing
			if !x.Operative {
				status = "suspended"
			} else if x.Timing == "overdue" || x.Timing == "today" {
				status = "due"
			}
			if (f.Q == "" || strings.Contains(strings.ToLower(x.Text), strings.ToLower(f.Q))) && (f.EventID == "" || (x.OwnerKind == "event" && x.OwnerID == f.EventID)) && (f.Status == "" || f.Status == status) {
				filtered = append(filtered, x)
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			if filtered[i].Operative != filtered[j].Operative {
				return filtered[i].Operative
			}
			return reminderLess(filtered[i], filtered[j])
		})
		resp.Total = len(filtered)
		resp.Items = paginatePlanning(filtered, f.Limit, f.Offset)
	}
	return resp, nil
}
