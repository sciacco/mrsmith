package training

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ArchiveCourse sets is_active=false. Historic enrollments stay intact.
func (s *SQLStore) ArchiveCourse(ctx context.Context, principal Principal, courseID string) (ActionResponse, error) {
	if !principal.IsPeopleAdmin {
		return ActionResponse{}, forbiddenError("people_role_required", "azione riservata a People")
	}
	if strings.TrimSpace(courseID) == "" {
		return ActionResponse{}, validationError("missing_id", "id corso obbligatorio")
	}
	var resp ActionResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		before, err := entitySnapshot(ctx, tx, "course", courseID)
		if err != nil {
			return err
		}
		const stmt = `UPDATE training.course SET is_active = false, updated_at = now() WHERE id = $1::uuid RETURNING id::text`
		if err := tx.QueryRowContext(ctx, stmt, courseID).Scan(&resp.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return notFoundError("course_not_found", "corso non trovato")
			}
			return fmt.Errorf("archive course: %w", err)
		}
		after, err := entitySnapshot(ctx, tx, "course", courseID)
		if err != nil {
			return err
		}
		if err := s.audit(ctx, tx, principal, "course", courseID, "archive", before, after); err != nil {
			return err
		}
		resp.OK = true
		resp.Status = "archived"
		return nil
	})
	return resp, err
}
