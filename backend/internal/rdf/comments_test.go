package rdf

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/notifications"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/keycloak"
)

func TestHandlePostCommentPersistsMentionAndNotifiedUser(t *testing.T) {
	state := &rdfTestState{}
	h := &Handler{
		anisettaDB: openRDFStateTestDB(t, "comment-create", state),
		roleResolver: &fakeRDFRoleResolver{usersByRole: map[string][]keycloak.User{
			"app_rdf_access": {
				{ID: "alice-subject", Email: "alice@example.com", Name: "Alice"},
			},
			"app_rdf_manager": nil,
		}},
	}

	reqBody := []byte(`{
		"comment": "  Commento operativo  ",
		"mentioned_users": [
			{"id": "alice-subject", "email": "alice@example.com"}
		]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/rdf/v1/richieste/42/comments", bytes.NewReader(reqBody))
	req.SetPathValue("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{
		Subject: "author-subject",
		Email:   "author@example.com",
		Name:    "Author",
	}))
	rec := httptest.NewRecorder()

	h.handlePostComment(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !state.committed {
		t.Fatalf("expected transaction commit")
	}
	if state.insertedComment != "Commento operativo" {
		t.Fatalf("unexpected inserted comment %q", state.insertedComment)
	}
	if state.insertedAuthorSubject != "author-subject" || state.insertedAuthorEmail != "author@example.com" || state.insertedAuthorName != "Author" {
		t.Fatalf("unexpected author snapshot: subject=%q email=%q name=%q", state.insertedAuthorSubject, state.insertedAuthorEmail, state.insertedAuthorName)
	}
	if !reflect.DeepEqual(state.mentionEmails, []string{"alice@example.com"}) {
		t.Fatalf("unexpected mention inserts: %#v", state.mentionEmails)
	}
	if !reflect.DeepEqual(state.notifiedEmails, []string{"alice@example.com"}) {
		t.Fatalf("unexpected notified upserts: %#v", state.notifiedEmails)
	}

	var response rdfComment
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ID != 101 || response.RichiestaID != 42 || response.Comment != "Commento operativo" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if len(response.MentionedUsers) != 1 || response.MentionedUsers[0].Email != "alice@example.com" {
		t.Fatalf("unexpected response mentions: %#v", response.MentionedUsers)
	}
}

func TestHandlePostCommentRejectsNonRDFMention(t *testing.T) {
	state := &rdfTestState{}
	h := &Handler{
		anisettaDB: openRDFStateTestDB(t, "comment-create", state),
		roleResolver: &fakeRDFRoleResolver{usersByRole: map[string][]keycloak.User{
			"app_rdf_access":  nil,
			"app_rdf_manager": nil,
		}},
	}

	req := httptest.NewRequest(http.MethodPost, "/rdf/v1/richieste/42/comments", bytes.NewReader([]byte(`{
		"comment": "Commento",
		"mentioned_users": [{"email": "outsider@example.com"}]
	}`)))
	req.SetPathValue("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{Subject: "author", Email: "author@example.com"}))
	rec := httptest.NewRecorder()

	h.handlePostComment(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if state.insertedComment != "" || state.committed {
		t.Fatalf("invalid mention should not insert comment, state=%#v", state)
	}
}

func TestHandlePostCommentTypedMentionTextDoesNotNotify(t *testing.T) {
	state := &rdfTestState{}
	h := &Handler{anisettaDB: openRDFStateTestDB(t, "comment-create", state)}

	req := httptest.NewRequest(http.MethodPost, "/rdf/v1/richieste/42/comments", bytes.NewReader([]byte(`{
		"comment": "Ciao @alice@example.com"
	}`)))
	req.SetPathValue("id", "42")
	req = req.WithContext(context.WithValue(req.Context(), auth.ClaimsKey, auth.Claims{Subject: "author", Email: "author@example.com"}))
	rec := httptest.NewRecorder()

	h.handlePostComment(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(state.mentionEmails) != 0 || len(state.notifiedEmails) != 0 {
		t.Fatalf("typed mention text should not create mention side effects: mentions=%#v notified=%#v", state.mentionEmails, state.notifiedEmails)
	}
}

func TestRDFCommentNotificationRecipientsDeduplicateAndExcludeAuthor(t *testing.T) {
	recipients := rdfCommentNotificationRecipients(
		[]rdfUserRef{
			{Subject: "author", Email: "author@example.com", Name: "Author Manager"},
			{Subject: "manager", Email: "manager@example.com", Name: "Manager"},
		},
		[]rdfUserRef{
			{Subject: "old", Email: "old@example.com", Name: "Old"},
			{Subject: "manager-duplicate", Email: "MANAGER@example.com", Name: "Duplicate"},
		},
		[]rdfUserRef{
			{Subject: "old", Email: "old@example.com", Name: "Old"},
			{Subject: "new", Email: "new@example.com", Name: "New"},
			{Subject: "author", Email: "other-author@example.com", Name: "Author Alias"},
		},
		rdfUserRef{Subject: "author", Email: "author@example.com"},
	)

	got := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		got = append(got, recipient.Email)
	}
	want := []string{"manager@example.com", "old@example.com", "new@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected recipients: got %#v want %#v", got, want)
	}
}

func TestNotifyRDFRichiestaCreatedNotifiesManagersWithEmailPolicy(t *testing.T) {
	dealID := int64(12345)
	notifier := &fakeRDFNotifier{}
	h := &Handler{
		notifier: notifier,
		appURLs: applaunch.NewURLResolver(map[string]string{
			applaunch.RichiesteFattibilitaAppID: "https://portal.example/apps/richieste-fattibilita/",
		}),
		roleResolver: &fakeRDFRoleResolver{usersByRole: map[string][]keycloak.User{
			"app_rdf_manager": {
				{ID: "creator-subject", Email: "creator@example.com", Name: "Creator Manager"},
				{ID: "manager-subject", Email: "manager@example.com", Name: "Manager"},
				{ID: "manager-duplicate", Email: "MANAGER@example.com", Name: "Duplicate"},
			},
		}},
	}
	ctx := context.WithValue(context.Background(), auth.ClaimsKey, auth.Claims{
		Subject: "creator-subject",
		Email:   "creator@example.com",
		Name:    "Creator",
	})

	err := h.notifyRDFRichiestaCreated(ctx, RichiestaFull{
		Richiesta: Richiesta{
			ID:         42,
			DealID:     &dealID,
			CodiceDeal: "DL-42",
			CreatedBy:  stringPointer("creator@example.com"),
		},
		DealName:    stringPointer("Upgrade connettivita"),
		CompanyName: stringPointer("Acme"),
	})
	if err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	if len(notifier.notifies) != 1 {
		t.Fatalf("expected one notification, got %#v", notifier.notifies)
	}
	input := notifier.notifies[0]
	if input.TypeKey != rdfRichiestaCreatedNotificationType {
		t.Fatalf("unexpected type key: %q", input.TypeKey)
	}
	if input.EntityType != rdfNotificationEntityType || input.EntityID != "42" {
		t.Fatalf("unexpected entity: type=%q id=%q", input.EntityType, input.EntityID)
	}
	if input.DedupeKey != "rdf:richiesta:42:created" {
		t.Fatalf("unexpected dedupe key: %q", input.DedupeKey)
	}
	if input.DeepLink != "https://portal.example/apps/richieste-fattibilita/richieste/42/view" {
		t.Fatalf("unexpected deep link: %q", input.DeepLink)
	}
	if input.Title != "Nuova RDF DL-42" {
		t.Fatalf("unexpected title: %q", input.Title)
	}
	if input.Body != "Creator ha inserito una nuova richiesta di fattibilita per Acme." {
		t.Fatalf("unexpected body: %q", input.Body)
	}
	if input.CreatedBySubject != "creator-subject" || input.CreatedByEmail != "creator@example.com" {
		t.Fatalf("unexpected creator: subject=%q email=%q", input.CreatedBySubject, input.CreatedByEmail)
	}
	if len(input.Recipients) != 1 || input.Recipients[0].Email != "manager@example.com" {
		t.Fatalf("unexpected recipients: %#v", input.Recipients)
	}
	if input.Metadata["richiesta_id"] != 42 || input.Metadata["codice_deal"] != "DL-42" || input.Metadata["company_name"] != "Acme" {
		t.Fatalf("unexpected metadata: %#v", input.Metadata)
	}
	if input.Metadata["deal_id"] != dealID {
		t.Fatalf("unexpected deal id metadata: %#v", input.Metadata)
	}
	steps := rdfCommentEmailSteps(t, input.PolicyOverride)
	if len(steps) != 3 {
		t.Fatalf("expected 3 email steps, got %#v", steps)
	}
}

func TestRDFRichiestaCreatedNotificationRecipientsDeduplicateAndExcludeAuthor(t *testing.T) {
	recipients := rdfRichiestaCreatedNotificationRecipients(
		[]rdfUserRef{
			{Subject: "author", Email: "author@example.com", Name: "Author Manager"},
			{Subject: "manager", Email: "manager@example.com", Name: "Manager"},
			{Subject: "manager-copy", Email: "MANAGER@example.com", Name: "Duplicate"},
			{Subject: "author", Email: "alias@example.com", Name: "Author Alias"},
		},
		rdfUserRef{Subject: "author", Email: "author@example.com"},
	)

	got := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		got = append(got, recipient.Email)
	}
	want := []string{"manager@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected recipients: got %#v want %#v", got, want)
	}
}

func TestRDFCommentEmailScheduleUsesRomeBusinessWindow(t *testing.T) {
	loc := rdfRomeLocation()
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "morning",
			now:  time.Date(2026, 5, 14, 10, 0, 0, 0, loc),
			want: time.Date(2026, 5, 14, 14, 0, 0, 0, loc),
		},
		{
			name: "evening-carries-to-next-day",
			now:  time.Date(2026, 5, 14, 18, 30, 0, 0, loc),
			want: time.Date(2026, 5, 15, 10, 30, 0, 0, loc),
		},
		{
			name: "after-window",
			now:  time.Date(2026, 5, 14, 21, 0, 0, 0, loc),
			want: time.Date(2026, 5, 15, 12, 0, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			firstDue := rdfCommentFirstEmailDueAt(tc.now)
			if !firstDue.Equal(tc.want.UTC()) {
				t.Fatalf("first due mismatch: got %s want %s", firstDue, tc.want.UTC())
			}
			steps := rdfCommentEmailSteps(t, rdfCommentEmailPolicyOverride(tc.now))
			if len(steps) != 3 {
				t.Fatalf("expected 3 email steps, got %#v", steps)
			}
			assertDelayDueAt(t, tc.now, steps[0]["delay"].(string), tc.want)
			assertDelayDueAt(t, tc.now, steps[1]["delay"].(string), tc.want.Add(24*time.Hour))
			assertDelayDueAt(t, tc.now, steps[2]["delay"].(string), tc.want.Add(48*time.Hour))
		})
	}
}

func TestRDFCommentsMigrationDefinesExpectedConstraints(t *testing.T) {
	raw, err := os.ReadFile("../../../deploy/migrations/009_anisetta_rdf_comments.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	required := []string{
		"CREATE TABLE IF NOT EXISTS public.rdf_commenti",
		"REFERENCES public.rdf_richieste(id) ON DELETE CASCADE",
		"CREATE UNIQUE INDEX IF NOT EXISTS rdf_commenti_menzioni_commento_email_idx",
		"CREATE UNIQUE INDEX IF NOT EXISTS rdf_richieste_notificati_richiesta_email_idx",
		"'rdf_comment_created'",
		"'richieste-fattibilita'",
	}
	for _, snippet := range required {
		if !bytes.Contains(raw, []byte(snippet)) {
			t.Fatalf("migration missing %q\n%s", snippet, sql)
		}
	}
}

func TestRDFCreateNotificationMigrationDefinesExpectedType(t *testing.T) {
	raw, err := os.ReadFile("../../../deploy/migrations/010_anisetta_rdf_create_notifications.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	required := []string{
		"'rdf_richiesta_created'",
		"'richieste-fattibilita'",
		"'Nuova RDF'",
		`"portal":`,
		`"email":`,
	}
	for _, snippet := range required {
		if !bytes.Contains(raw, []byte(snippet)) {
			t.Fatalf("migration missing %q\n%s", snippet, sql)
		}
	}
}

func rdfCommentEmailSteps(t *testing.T, policy map[string]any) []map[string]any {
	t.Helper()
	emailPolicy, ok := policy["email"].(map[string]any)
	if !ok {
		t.Fatalf("missing email policy: %#v", policy)
	}
	steps, ok := emailPolicy["steps"].([]map[string]any)
	if !ok {
		t.Fatalf("missing email steps: %#v", emailPolicy)
	}
	return steps
}

func assertDelayDueAt(t *testing.T, now time.Time, delayRaw string, want time.Time) {
	t.Helper()
	delay, err := time.ParseDuration(delayRaw)
	if err != nil {
		t.Fatalf("parse delay %q: %v", delayRaw, err)
	}
	got := now.Add(delay)
	if !got.Equal(want) {
		t.Fatalf("delay %q due mismatch: got %s want %s", delayRaw, got, want)
	}
}

type fakeRDFRoleResolver struct {
	usersByRole map[string][]keycloak.User
	err         error
}

func (r *fakeRDFRoleResolver) UsersByRealmRole(_ context.Context, roleName string, _ keycloak.UsersByRealmRoleOptions) ([]keycloak.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	return append([]keycloak.User(nil), r.usersByRole[roleName]...), nil
}

type fakeRDFNotifier struct {
	notifies []notifications.NotifyInput
	resolves []notifications.ResolveInput
}

func (n *fakeRDFNotifier) Notify(_ context.Context, input notifications.NotifyInput) (notifications.NotifyResult, error) {
	n.notifies = append(n.notifies, input)
	return notifications.NotifyResult{
		NotificationID: int64(len(n.notifies)),
		RecipientCount: len(input.Recipients),
		Created:        true,
	}, nil
}

func (n *fakeRDFNotifier) Resolve(_ context.Context, input notifications.ResolveInput) error {
	n.resolves = append(n.resolves, input)
	return nil
}
