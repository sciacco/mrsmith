package rdf

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/sciacco/mrsmith/internal/auth"
	"github.com/sciacco/mrsmith/internal/notifications"
	"github.com/sciacco/mrsmith/internal/platform/applaunch"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
	"github.com/sciacco/mrsmith/internal/platform/keycloak"
	"github.com/sciacco/mrsmith/internal/platform/logging"
)

const (
	rdfCommentNotificationType          = "rdf_comment_created"
	rdfRichiestaCreatedNotificationType = "rdf_richiesta_created"
	rdfNotificationEntityType           = "rdf_richiesta"
)

var (
	errRDFUsersUnavailable  = errors.New("rdf_users_unavailable")
	errInvalidMentionedUser = errors.New("invalid_mentioned_users")
)

type rdfUserRef struct {
	ID        string `json:"id,omitempty"`
	Subject   string `json:"subject,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email"`
}

type rdfComment struct {
	ID             int64        `json:"id"`
	RichiestaID    int          `json:"richiesta_id"`
	Comment        string       `json:"comment"`
	Author         rdfUserRef   `json:"author"`
	CreatedAt      string       `json:"created_at"`
	MentionedUsers []rdfUserRef `json:"mentioned_users"`
}

type rdfCommentsResponse struct {
	Items         []rdfComment `json:"items"`
	NotifiedUsers []rdfUserRef `json:"notified_users"`
}

type rdfUsersResponse struct {
	Items []rdfUserRef `json:"items"`
}

type postRDFCommentRequest struct {
	Comment        string       `json:"comment"`
	MentionedUsers []rdfUserRef `json:"mentioned_users"`
}

func (h *Handler) handleListComments(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}
	richiestaID, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_richiesta_id")
		return
	}
	if _, err := h.fetchRichiesta(r.Context(), richiestaID); errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "richiesta_not_found")
		return
	} else if err != nil {
		h.dbFailure(w, r, "list_comments_fetch_richiesta", err, "richiesta_id", richiestaID)
		return
	}

	comments, err := h.listComments(r.Context(), richiestaID)
	if err != nil {
		h.dbFailure(w, r, "list_comments", err, "richiesta_id", richiestaID)
		return
	}
	notifiedUsers, err := h.listNotifiedUsers(r.Context(), richiestaID)
	if err != nil {
		h.dbFailure(w, r, "list_comments_notified_users", err, "richiesta_id", richiestaID)
		return
	}
	httputil.JSON(w, http.StatusOK, rdfCommentsResponse{
		Items:         comments,
		NotifiedUsers: notifiedUsers,
	})
}

func (h *Handler) handlePostComment(w http.ResponseWriter, r *http.Request) {
	if !h.requireAnisetta(w) {
		return
	}
	richiestaID, err := pathInt(r, "id")
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_richiesta_id")
		return
	}
	var body postRDFCommentRequest
	if err := decodeBody(r, &body); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_json")
		return
	}
	commentText := strings.TrimSpace(body.Comment)
	if commentText == "" {
		httputil.Error(w, http.StatusBadRequest, "comment_required")
		return
	}

	richiesta, err := h.fetchRichiesta(r.Context(), richiestaID)
	if errors.Is(err, sql.ErrNoRows) {
		httputil.Error(w, http.StatusNotFound, "richiesta_not_found")
		return
	}
	if err != nil {
		h.dbFailure(w, r, "post_comment_fetch_richiesta", err, "richiesta_id", richiestaID)
		return
	}

	mentions, err := h.validatedMentionedUsers(r.Context(), body.MentionedUsers)
	if err != nil {
		if errors.Is(err, errInvalidMentionedUser) {
			httputil.Error(w, http.StatusBadRequest, "invalid_mentioned_users")
			return
		}
		logging.FromContext(r.Context()).Warn(
			"rdf users lookup failed",
			"component", "rdf",
			"operation", "validate_comment_mentions",
			"richiesta_id", richiestaID,
			"error", err,
		)
		httputil.Error(w, http.StatusServiceUnavailable, "rdf_users_unavailable")
		return
	}

	author := rdfAuthorFromContext(r.Context())
	tx, err := h.anisettaDB.BeginTx(r.Context(), nil)
	if err != nil {
		h.dbFailure(w, r, "post_comment_begin", err, "richiesta_id", richiestaID)
		return
	}
	defer h.rollbackTx(r.Context(), tx, "post_comment_rollback", "richiesta_id", richiestaID)

	created, err := h.insertComment(r.Context(), tx, richiestaID, commentText, author)
	if err != nil {
		h.dbFailure(w, r, "post_comment_insert", err, "richiesta_id", richiestaID)
		return
	}
	for _, mention := range mentions {
		if err := h.insertCommentMention(r.Context(), tx, created.ID, richiestaID, mention); err != nil {
			h.dbFailure(w, r, "post_comment_insert_mention", err, "richiesta_id", richiestaID, "comment_id", created.ID)
			return
		}
		if err := h.upsertNotifiedUser(r.Context(), tx, richiestaID, created.ID, mention); err != nil {
			h.dbFailure(w, r, "post_comment_upsert_notified", err, "richiesta_id", richiestaID, "comment_id", created.ID)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		h.dbFailure(w, r, "post_comment_commit", err, "richiesta_id", richiestaID, "comment_id", created.ID)
		return
	}

	created.MentionedUsers = mentions
	if err := h.notifyRDFCommentCreated(r.Context(), richiesta, created); err != nil {
		logging.FromContext(r.Context()).Warn(
			"rdf comment notification failed",
			"component", "rdf",
			"operation", "comment_notification",
			"richiesta_id", richiestaID,
			"comment_id", created.ID,
			"error", err,
		)
	}
	httputil.JSON(w, http.StatusCreated, created)
}

func (h *Handler) handleListRDFUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.listRDFUsersByRoles(r.Context(), applaunch.RichiesteFattibilitaAccessRoles())
	if err != nil {
		logging.FromContext(r.Context()).Warn(
			"rdf users lookup failed",
			"component", "rdf",
			"operation", "list_rdf_users",
			"error", err,
		)
		httputil.Error(w, http.StatusServiceUnavailable, "rdf_users_unavailable")
		return
	}
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	filtered := make([]rdfUserRef, 0, min(len(users), 25))
	for _, user := range users {
		if search != "" && !rdfUserMatchesSearch(user, search) {
			continue
		}
		filtered = append(filtered, user)
		if len(filtered) == 25 {
			break
		}
	}
	httputil.JSON(w, http.StatusOK, rdfUsersResponse{Items: filtered})
}

func (h *Handler) listComments(ctx context.Context, richiestaID int) ([]rdfComment, error) {
	rows, err := h.anisettaDB.QueryContext(ctx, `
		SELECT id, richiesta_id, commento, autore_subject, autore_email, autore_nome, created_at
		FROM public.rdf_commenti
		WHERE richiesta_id = $1
		ORDER BY created_at, id
	`, richiestaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]rdfComment, 0)
	commentIDs := make([]int64, 0)
	for rows.Next() {
		var item rdfComment
		var authorSubject, authorEmail, authorName string
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.RichiestaID, &item.Comment, &authorSubject, &authorEmail, &authorName, &createdAt); err != nil {
			return nil, err
		}
		item.Author = rdfUserSnapshot(authorSubject, authorEmail, authorName)
		item.CreatedAt = formatTimestamp(createdAt)
		item.MentionedUsers = []rdfUserRef{}
		items = append(items, item)
		commentIDs = append(commentIDs, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return items, nil
	}

	mentions, err := h.listMentionsByComment(ctx, commentIDs)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if values := mentions[items[i].ID]; len(values) > 0 {
			items[i].MentionedUsers = values
		}
	}
	return items, nil
}

func (h *Handler) listMentionsByComment(ctx context.Context, commentIDs []int64) (map[int64][]rdfUserRef, error) {
	args := make([]any, 0, len(commentIDs))
	holders := make([]string, 0, len(commentIDs))
	for _, id := range commentIDs {
		holders = append(holders, placeholder(&args, id))
	}
	query := `
		SELECT commento_id, utente_subject, utente_email, utente_nome
		FROM public.rdf_commenti_menzioni
		WHERE commento_id IN (` + strings.Join(holders, ", ") + `)
		ORDER BY created_at, id
	`
	rows, err := h.anisettaDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]rdfUserRef)
	for rows.Next() {
		var commentID int64
		var subject, email, name string
		if err := rows.Scan(&commentID, &subject, &email, &name); err != nil {
			return nil, err
		}
		result[commentID] = append(result[commentID], rdfUserSnapshot(subject, email, name))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *Handler) listNotifiedUsers(ctx context.Context, richiestaID int) ([]rdfUserRef, error) {
	rows, err := h.anisettaDB.QueryContext(ctx, `
		SELECT utente_subject, utente_email, utente_nome
		FROM public.rdf_richieste_notificati
		WHERE richiesta_id = $1
		ORDER BY updated_at DESC, id DESC
	`, richiestaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]rdfUserRef, 0)
	for rows.Next() {
		var subject, email, name string
		if err := rows.Scan(&subject, &email, &name); err != nil {
			return nil, err
		}
		items = append(items, rdfUserSnapshot(subject, email, name))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (h *Handler) insertComment(ctx context.Context, tx *sql.Tx, richiestaID int, comment string, author rdfUserRef) (rdfComment, error) {
	var id int64
	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO public.rdf_commenti (
			richiesta_id,
			commento,
			autore_subject,
			autore_email,
			autore_nome
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, richiestaID, comment, rdfUserSubject(author), author.Email, author.Name).Scan(&id, &createdAt); err != nil {
		return rdfComment{}, err
	}
	return rdfComment{
		ID:             id,
		RichiestaID:    richiestaID,
		Comment:        comment,
		Author:         author,
		CreatedAt:      formatTimestamp(createdAt),
		MentionedUsers: []rdfUserRef{},
	}, nil
}

func (h *Handler) insertCommentMention(ctx context.Context, tx *sql.Tx, commentID int64, richiestaID int, mention rdfUserRef) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO public.rdf_commenti_menzioni (
			commento_id,
			richiesta_id,
			utente_subject,
			utente_email,
			utente_nome
		)
		VALUES ($1, $2, $3, $4, $5)
	`, commentID, richiestaID, rdfUserSubject(mention), mention.Email, mention.Name)
	return err
}

func (h *Handler) upsertNotifiedUser(ctx context.Context, tx *sql.Tx, richiestaID int, commentID int64, user rdfUserRef) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO public.rdf_richieste_notificati (
			richiesta_id,
			utente_subject,
			utente_email,
			utente_nome,
			source_commento_id
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (richiesta_id, (lower(utente_email))) DO UPDATE
		SET utente_subject = CASE
				WHEN EXCLUDED.utente_subject <> '' THEN EXCLUDED.utente_subject
				ELSE rdf_richieste_notificati.utente_subject
			END,
			utente_email = EXCLUDED.utente_email,
			utente_nome = CASE
				WHEN EXCLUDED.utente_nome <> '' THEN EXCLUDED.utente_nome
				ELSE rdf_richieste_notificati.utente_nome
			END,
			source_commento_id = EXCLUDED.source_commento_id,
			updated_at = now()
	`, richiestaID, rdfUserSubject(user), user.Email, user.Name, commentID)
	return err
}

func (h *Handler) validatedMentionedUsers(ctx context.Context, requested []rdfUserRef) ([]rdfUserRef, error) {
	normalized, err := normalizeRequestedRDFUsers(requested)
	if err != nil || len(normalized) == 0 {
		return normalized, err
	}
	allowed, err := h.listRDFUsersByRoles(ctx, applaunch.RichiesteFattibilitaAccessRoles())
	if err != nil {
		return nil, err
	}
	return validateMentionedRDFUsers(allowed, normalized)
}

func validateMentionedRDFUsers(allowed []rdfUserRef, requested []rdfUserRef) ([]rdfUserRef, error) {
	allowedByEmail := make(map[string]rdfUserRef, len(allowed))
	for _, user := range allowed {
		key := strings.ToLower(strings.TrimSpace(user.Email))
		if key == "" {
			continue
		}
		allowedByEmail[key] = user
	}

	result := make([]rdfUserRef, 0, len(requested))
	for _, requestUser := range requested {
		key := strings.ToLower(strings.TrimSpace(requestUser.Email))
		allowedUser, ok := allowedByEmail[key]
		if !ok {
			return nil, fmt.Errorf("%w: %s", errInvalidMentionedUser, key)
		}
		requestSubject := rdfUserSubject(requestUser)
		allowedSubject := rdfUserSubject(allowedUser)
		if requestSubject != "" && allowedSubject != "" && requestSubject != allowedSubject {
			return nil, fmt.Errorf("%w: %s", errInvalidMentionedUser, key)
		}
		result = append(result, allowedUser)
	}
	return result, nil
}

func normalizeRequestedRDFUsers(requested []rdfUserRef) ([]rdfUserRef, error) {
	if len(requested) == 0 {
		return nil, nil
	}
	result := make([]rdfUserRef, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, user := range requested {
		user = normalizeRDFUserRef(user)
		if user.Email == "" || !isValidEmail(user.Email) {
			return nil, fmt.Errorf("%w: invalid email", errInvalidMentionedUser)
		}
		key := strings.ToLower(user.Email)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, user)
	}
	return result, nil
}

func (h *Handler) listRDFUsersByRoles(ctx context.Context, roles []string) ([]rdfUserRef, error) {
	if h.roleResolver == nil {
		return nil, errRDFUsersUnavailable
	}
	byEmail := make(map[string]rdfUserRef)
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		users, err := h.roleResolver.UsersByRealmRole(ctx, role, keycloak.UsersByRealmRoleOptions{PageSize: 100})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errRDFUsersUnavailable, err)
		}
		for _, user := range users {
			ref := rdfUserFromKeycloak(user)
			if ref.Email == "" {
				continue
			}
			key := strings.ToLower(ref.Email)
			if existing, exists := byEmail[key]; exists && rdfUserSubject(existing) != "" {
				continue
			}
			byEmail[key] = ref
		}
	}
	result := make([]rdfUserRef, 0, len(byEmail))
	for _, user := range byEmail {
		result = append(result, user)
	}
	sortRDFUsers(result)
	return result, nil
}

func sortRDFUsers(users []rdfUserRef) {
	sort.SliceStable(users, func(i, j int) bool {
		leftName := strings.ToLower(strings.TrimSpace(users[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(users[j].Name))
		if leftName != rightName {
			return leftName < rightName
		}
		leftEmail := strings.ToLower(strings.TrimSpace(users[i].Email))
		rightEmail := strings.ToLower(strings.TrimSpace(users[j].Email))
		if leftEmail != rightEmail {
			return leftEmail < rightEmail
		}
		return rdfUserSubject(users[i]) < rdfUserSubject(users[j])
	})
}

func rdfUserMatchesSearch(user rdfUserRef, search string) bool {
	fields := []string{
		user.Email,
		user.Name,
		user.Username,
		user.FirstName,
		user.LastName,
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), search) {
			return true
		}
	}
	return false
}

func (h *Handler) notifyRDFCommentCreated(ctx context.Context, richiesta Richiesta, comment rdfComment) error {
	if h.notifier == nil {
		return nil
	}
	managers, err := h.listRDFUsersByRoles(ctx, applaunch.RichiesteFattibilitaManagerRoles())
	if err != nil {
		return err
	}
	notifiedUsers, err := h.listNotifiedUsers(ctx, richiesta.ID)
	if err != nil {
		return err
	}
	recipients := rdfCommentNotificationRecipients(managers, notifiedUsers, comment.MentionedUsers, comment.Author)
	if len(recipients) == 0 {
		return nil
	}

	code := firstNonEmpty(richiesta.CodiceDeal, fmt.Sprintf("#%d", richiesta.ID))
	authorLabel := firstNonEmpty(comment.Author.Name, comment.Author.Email, "Un utente")
	mentionedEmails := make([]string, 0, len(comment.MentionedUsers))
	for _, mention := range comment.MentionedUsers {
		mentionedEmails = append(mentionedEmails, mention.Email)
	}
	_, err = h.notifier.Notify(ctx, notifications.NotifyInput{
		TypeKey:    rdfCommentNotificationType,
		Title:      fmt.Sprintf("Nuovo commento su RDF %s", code),
		Body:       fmt.Sprintf("%s ha commentato la richiesta di fattibilita %s.", authorLabel, code),
		EntityType: rdfNotificationEntityType,
		EntityID:   strconv.Itoa(richiesta.ID),
		DedupeKey:  fmt.Sprintf("rdf:richiesta:%d:comment:%d", richiesta.ID, comment.ID),
		DeepLink:   h.rdfRichiestaDeepLink(richiesta.ID),
		Metadata: map[string]any{
			"richiesta_id":     richiesta.ID,
			"codice_deal":      richiesta.CodiceDeal,
			"comment_id":       comment.ID,
			"author_email":     comment.Author.Email,
			"mentioned_emails": mentionedEmails,
			"comment_preview":  truncateForNotificationMetadata(comment.Comment, 180),
		},
		PolicyOverride:   rdfCommentEmailPolicyOverride(time.Now()),
		Recipients:       recipients,
		CreatedBySubject: rdfUserSubject(comment.Author),
		CreatedByEmail:   comment.Author.Email,
	})
	return err
}

func rdfCommentNotificationRecipients(managers, notifiedUsers, mentionedUsers []rdfUserRef, author rdfUserRef) []notifications.Recipient {
	inputs := make([]rdfUserRef, 0, len(managers)+len(notifiedUsers)+len(mentionedUsers))
	inputs = append(inputs, managers...)
	inputs = append(inputs, notifiedUsers...)
	inputs = append(inputs, mentionedUsers...)

	return rdfNotificationRecipients(inputs, author)
}

func rdfRichiestaCreatedNotificationRecipients(managers []rdfUserRef, author rdfUserRef) []notifications.Recipient {
	return rdfNotificationRecipients(managers, author)
}

func rdfNotificationRecipients(users []rdfUserRef, author rdfUserRef) []notifications.Recipient {
	recipients := make([]notifications.Recipient, 0, len(users))
	seen := make(map[string]struct{}, len(users))
	authorEmail := strings.ToLower(strings.TrimSpace(author.Email))
	authorSubject := rdfUserSubject(author)
	for _, user := range users {
		user = normalizeRDFUserRef(user)
		if user.Email == "" {
			continue
		}
		key := strings.ToLower(user.Email)
		if key == authorEmail || (authorSubject != "" && rdfUserSubject(user) == authorSubject) {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		recipients = append(recipients, notifications.Recipient{
			Subject: rdfUserSubject(user),
			Email:   user.Email,
			Name:    user.Name,
		})
	}
	return recipients
}

func (h *Handler) notifyRDFRichiestaCreated(ctx context.Context, full RichiestaFull) error {
	if h.notifier == nil {
		return nil
	}
	managers, err := h.listRDFUsersByRoles(ctx, applaunch.RichiesteFattibilitaManagerRoles())
	if err != nil {
		return err
	}
	author := rdfAuthorFromContext(ctx)
	if author.Email == "" && full.CreatedBy != nil {
		author.Email = normalizeRDFEmail(*full.CreatedBy)
	}
	if author.Name == "" {
		author.Name = firstNonEmpty(author.Email, derefString(full.CreatedBy, ""))
	}
	recipients := rdfRichiestaCreatedNotificationRecipients(managers, author)
	if len(recipients) == 0 {
		return nil
	}

	code := firstNonEmpty(full.CodiceDeal, fmt.Sprintf("#%d", full.ID))
	authorLabel := firstNonEmpty(author.Name, author.Email, derefString(full.CreatedBy, ""), "Un utente")
	companyLabel := derefString(full.CompanyName, "cliente non disponibile")
	var dealID any
	if full.DealID != nil {
		dealID = *full.DealID
	}
	_, err = h.notifier.Notify(ctx, notifications.NotifyInput{
		TypeKey:    rdfRichiestaCreatedNotificationType,
		Title:      fmt.Sprintf("Nuova RDF %s", code),
		Body:       fmt.Sprintf("%s ha inserito una nuova richiesta di fattibilita per %s.", authorLabel, companyLabel),
		EntityType: rdfNotificationEntityType,
		EntityID:   strconv.Itoa(full.ID),
		DedupeKey:  fmt.Sprintf("rdf:richiesta:%d:created", full.ID),
		DeepLink:   h.rdfRichiestaDeepLink(full.ID),
		Metadata: map[string]any{
			"richiesta_id": full.ID,
			"codice_deal":  full.CodiceDeal,
			"deal_id":      dealID,
			"company_name": derefString(full.CompanyName, ""),
			"deal_name":    derefString(full.DealName, ""),
			"created_by":   derefString(full.CreatedBy, author.Email),
		},
		PolicyOverride:   rdfCommentEmailPolicyOverride(time.Now()),
		Recipients:       recipients,
		CreatedBySubject: rdfUserSubject(author),
		CreatedByEmail:   author.Email,
	})
	return err
}

func rdfCommentEmailPolicyOverride(now time.Time) map[string]any {
	firstDue := rdfCommentFirstEmailDueAt(now)
	steps := []map[string]any{
		{"step": "unread_after_business_4h", "delay": durationDelayString(firstDue.Sub(now))},
		{"step": "unread_after_24h", "delay": durationDelayString(firstDue.Add(24 * time.Hour).Sub(now))},
		{"step": "unread_after_48h", "delay": durationDelayString(firstDue.Add(48 * time.Hour).Sub(now))},
	}
	return map[string]any{
		"email": map[string]any{
			"enabled": true,
			"steps":   steps,
		},
	}
}

func rdfCommentFirstEmailDueAt(now time.Time) time.Time {
	loc := rdfRomeLocation()
	localNow := now.In(loc)
	return addRDFBusinessDuration(localNow, 4*time.Hour).UTC()
}

func addRDFBusinessDuration(localNow time.Time, remaining time.Duration) time.Time {
	loc := localNow.Location()
	current := localNow
	for remaining > 0 {
		start := time.Date(current.Year(), current.Month(), current.Day(), 8, 0, 0, 0, loc)
		end := time.Date(current.Year(), current.Month(), current.Day(), 20, 0, 0, 0, loc)
		if current.Before(start) {
			current = start
		}
		if !current.Before(end) {
			next := current.AddDate(0, 0, 1)
			current = time.Date(next.Year(), next.Month(), next.Day(), 8, 0, 0, 0, loc)
			continue
		}
		available := end.Sub(current)
		if remaining <= available {
			return current.Add(remaining)
		}
		remaining -= available
		next := current.AddDate(0, 0, 1)
		current = time.Date(next.Year(), next.Month(), next.Day(), 8, 0, 0, 0, loc)
	}
	return current
}

func rdfRomeLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		return time.FixedZone("Europe/Rome", 3600)
	}
	return loc
}

func durationDelayString(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	return duration.Round(time.Second).String()
}

func truncateForNotificationMetadata(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func (h *Handler) rdfRichiestaDeepLink(id int) string {
	escapedID := url.PathEscape(strconv.Itoa(id))
	appURL := strings.TrimRight(strings.TrimSpace(h.richiesteFattibilitaAppURL), "/")
	if appURL != "" {
		return appURL + "/richieste/" + escapedID + "/view"
	}
	if strings.TrimSpace(h.staticDir) == "" {
		return "http://localhost:5182/richieste/" + escapedID + "/view"
	}
	return "/apps/richieste-fattibilita/richieste/" + escapedID + "/view"
}

func rdfAuthorFromContext(ctx context.Context) rdfUserRef {
	claims, _ := auth.GetClaims(ctx)
	name := strings.TrimSpace(claims.Name)
	email := normalizeRDFEmail(claims.Email)
	if name == "" {
		name = email
	}
	subject := strings.TrimSpace(claims.Subject)
	return rdfUserRef{
		ID:      subject,
		Subject: subject,
		Name:    name,
		Email:   email,
	}
}

func rdfUserFromKeycloak(user keycloak.User) rdfUserRef {
	ref := rdfUserRef{
		ID:        strings.TrimSpace(user.ID),
		Subject:   strings.TrimSpace(user.ID),
		Username:  strings.TrimSpace(user.Username),
		FirstName: strings.TrimSpace(user.FirstName),
		LastName:  strings.TrimSpace(user.LastName),
		Name:      strings.TrimSpace(user.Name),
		Email:     normalizeRDFEmail(user.Email),
	}
	if ref.Name == "" {
		ref.Name = strings.TrimSpace(strings.Join([]string{ref.FirstName, ref.LastName}, " "))
	}
	if ref.Name == "" {
		ref.Name = firstNonEmpty(ref.Username, ref.Email)
	}
	return ref
}

func rdfUserSnapshot(subject, email, name string) rdfUserRef {
	subject = strings.TrimSpace(subject)
	return rdfUserRef{
		ID:      subject,
		Subject: subject,
		Email:   normalizeRDFEmail(email),
		Name:    strings.TrimSpace(name),
	}
}

func normalizeRDFUserRef(user rdfUserRef) rdfUserRef {
	user.ID = strings.TrimSpace(user.ID)
	user.Subject = strings.TrimSpace(user.Subject)
	user.Username = strings.TrimSpace(user.Username)
	user.FirstName = strings.TrimSpace(user.FirstName)
	user.LastName = strings.TrimSpace(user.LastName)
	user.Name = strings.TrimSpace(user.Name)
	user.Email = normalizeRDFEmail(user.Email)
	if user.Subject == "" {
		user.Subject = user.ID
	}
	if user.ID == "" {
		user.ID = user.Subject
	}
	if user.Name == "" {
		user.Name = strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " "))
	}
	if user.Name == "" {
		user.Name = firstNonEmpty(user.Username, user.Email)
	}
	return user
}

func rdfUserSubject(user rdfUserRef) string {
	return strings.TrimSpace(firstNonEmpty(user.Subject, user.ID))
}

func normalizeRDFEmail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	address, err := mail.ParseAddress(value)
	if err == nil && address.Address != "" && strings.Contains(address.Address, "@") {
		return strings.ToLower(address.Address)
	}
	if !strings.Contains(value, "@") {
		return value
	}
	return strings.ToLower(value)
}

func isValidEmail(value string) bool {
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	return err == nil && address.Address != "" && strings.Contains(address.Address, "@")
}
