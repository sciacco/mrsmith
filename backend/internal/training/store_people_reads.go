package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Persone (#152, slice 1 del task 6): lista gestionale e scheda formativa.
// Includono le persone inattive (flag status); la lookup persone dei form
// resta invariata e a soli attivi (store.go). Le liste annidate (team,
// gruppi, iscrizioni, richieste) si aggregano in JSON lato query: stesso
// pattern di directory_sync.go e factorial_sync_local.go, un'unica riga per
// persona invece di N+1 letture.

// decodeJSONSlice decodifica una colonna aggregata con json_agg(...)::text
// in uno slice tipizzato.
func decodeJSONSlice[T any](raw string) ([]T, error) {
	result := make([]T, 0)
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("decode training json slice: %w", err)
	}
	return result, nil
}

const personTeamsJSON = `
    COALESCE((
      SELECT json_agg(json_build_object('id', t.id::text, 'name', t.name, 'role', COALESCE(tm.role, '')) ORDER BY t.name)
      FROM training.team_membership tm
      JOIN training.team t ON t.id = tm.team_id
      WHERE tm.employee_id = e.id AND tm.end_date IS NULL
    ), '[]')::text`

const personGroupsJSON = `
    COALESCE((
      SELECT json_agg(json_build_object('id', g.id::text, 'name', g.name) ORDER BY g.name)
      FROM training.custom_group_members m
      JOIN training.custom_groups g ON g.id = m.group_id
      WHERE m.employee_id = e.id
    ), '[]')::text`

func (s *SQLStore) ListPeople(ctx context.Context) ([]PersonListRow, error) {
	q := `
SELECT
  e.id::text, e.first_name, e.last_name, e.email::text, e.status::text, e.directory_exempt,` +
		personTeamsJSON + "," + personGroupsJSON + `
FROM training.employee e
ORDER BY e.last_name, e.first_name
LIMIT 1000`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training people: %w", err)
	}
	defer rows.Close()

	result := make([]PersonListRow, 0)
	for rows.Next() {
		var row PersonListRow
		var teams, groups string
		if err := rows.Scan(&row.ID, &row.FirstName, &row.LastName, &row.Email, &row.Status, &row.DirectoryExempt, &teams, &groups); err != nil {
			return nil, fmt.Errorf("scan training person: %w", err)
		}
		if row.Teams, err = decodeJSONSlice[PersonTeamRef](teams); err != nil {
			return nil, err
		}
		if row.Groups, err = decodeJSONSlice[PersonGroupRef](groups); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// personAwardsJSON aggrega i conseguimenti della persona dalla vista viva
// v_employee_certifications (#160, slice 1 del task 7): stesso appiattimento
// documento della LATERAL di ListCertifications (store.go), ma qui il
// documento resta un oggetto nullable (nessun conseguimento => null) e non
// campi appiattiti, cosi come enrollmentId, assente quando il conseguimento
// non discende da un'iscrizione. certification_id non e' nella vista, si
// recupera unendo certification_award sull'award_id.
const personAwardsJSON = `
    COALESCE((
      SELECT json_agg(json_build_object(
        'awardId', vc.award_id::text, 'certificationId', ca.certification_id::text,
        'certificationCode', vc.cert_code, 'certificationName', vc.cert_name,
        'outcome', vc.outcome::text, 'awardedOn', vc.awarded_on::text,
        'expiresOn', COALESCE(vc.expires_on::text, ''), 'currentStatus', vc.current_status,
        'validationSource', vc.validation_source::text,
        'enrollmentId', ca.enrollment_id::text,
        'document', CASE WHEN doc.id IS NULL THEN NULL ELSE
          json_build_object('id', doc.id, 'filename', doc.filename, 'isValidated', doc.is_validated)
        END
      ) ORDER BY vc.awarded_on DESC)
      FROM training.v_employee_certifications vc
      JOIN training.certification_award ca ON ca.id = vc.award_id
      LEFT JOIN LATERAL (
        SELECT d.id::text, d.filename, d.is_validated
        FROM training.document d
        WHERE d.certification_award_id = vc.award_id
        ORDER BY d.uploaded_at DESC
        LIMIT 1
      ) doc ON true
      WHERE vc.employee_id = e.id
    ), '[]')::text`

// personAssessmentsJSON aggrega lo storico completo delle valutazioni di
// competenza della persona (#161, slice 2 del task 7), ordine area/data
// decrescente come richiesto dalla scheda.
const personAssessmentsJSON = `
    COALESCE((
      SELECT json_agg(json_build_object(
        'id', asmt.id::text, 'skillAreaId', asmt.skill_area_id::text, 'skillAreaName', sa.name,
        'level', asmt.level, 'assessedOn', asmt.assessed_on::text, 'source', asmt.source::text,
        'notes', COALESCE(asmt.notes, '')
      ) ORDER BY sa.name, asmt.assessed_on DESC)
      FROM training.skill_assessment asmt
      JOIN training.skill_area sa ON sa.id = asmt.skill_area_id
      WHERE asmt.employee_id = e.id
    ), '[]')::text`

// GetPersonDetail e la scheda formativa: dati persona, appartenenze,
// gruppi, iscrizioni cross-evento (CancelledAt e quello dell'evento,
// l'iscrizione non ne ha uno proprio), richieste (Outcome nil se aperta),
// conseguimenti (data decrescente), copertura sulle regole attive la cui
// platea la include, valutazioni di competenza e percorsi assegnati con
// progresso calcolato (#161, slice 2 del task 7).
func (s *SQLStore) GetPersonDetail(ctx context.Context, id string) (PersonDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PersonDetail{}, validationError("missing_id", "id persona obbligatorio")
	}
	q := `
SELECT
  e.id::text, e.first_name, e.last_name, e.email::text, e.status::text, e.directory_exempt,` +
		personTeamsJSON + "," + personGroupsJSON + `,
  COALESCE((
    SELECT json_agg(json_build_object(
      'enrollmentId', en.id::text, 'eventId', en.event_id::text, 'courseTitle', c.title,
      'deliveryStatus', en.delivery_status, 'learningOutcome', COALESCE(en.learning_outcome, ''),
      'actualStart', COALESCE(en.actual_start::text, ''), 'actualEnd', COALESCE(en.actual_end::text, ''),
      'cancelledAt', COALESCE(ev.cancelled_at::text, ''), 'createdAt', en.created_at::text
    ) ORDER BY en.created_at DESC)
    FROM training.enrollment en
    JOIN training.training_event ev ON ev.id = en.event_id
    JOIN training.course c ON c.id = ev.course_id
    WHERE en.employee_id = e.id
  ), '[]')::text,
  COALESCE((
    SELECT json_agg(json_build_object(
      'id', r.id::text, 'courseTitle', COALESCE(rc.title, ''),
      'outcome', r.outcome, 'createdAt', r.created_at::text
    ) ORDER BY r.created_at DESC)
    FROM training.training_request r
    LEFT JOIN training.course rc ON rc.id = r.course_id
    WHERE r.employee_id = e.id
  ), '[]')::text,` +
		personAwardsJSON + "," + personAssessmentsJSON + `
FROM training.employee e
WHERE e.id = $1::uuid`
	var detail PersonDetail
	var teams, groups, enrollments, requests, awards, assessments string
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&detail.ID, &detail.FirstName, &detail.LastName, &detail.Email, &detail.Status, &detail.DirectoryExempt,
		&teams, &groups, &enrollments, &requests, &awards, &assessments,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PersonDetail{}, notFoundError("employee_not_found", "persona non trovata")
	}
	if err != nil {
		return PersonDetail{}, fmt.Errorf("load training person detail: %w", err)
	}
	if detail.Teams, err = decodeJSONSlice[PersonTeamRef](teams); err != nil {
		return PersonDetail{}, err
	}
	if detail.Groups, err = decodeJSONSlice[PersonGroupRef](groups); err != nil {
		return PersonDetail{}, err
	}
	if detail.Enrollments, err = decodeJSONSlice[PersonEnrollmentRef](enrollments); err != nil {
		return PersonDetail{}, err
	}
	if detail.Requests, err = decodeJSONSlice[PersonRequestRef](requests); err != nil {
		return PersonDetail{}, err
	}
	if detail.Awards, err = decodeJSONSlice[PersonAwardRef](awards); err != nil {
		return PersonDetail{}, err
	}
	if detail.Assessments, err = decodeJSONSlice[PersonAssessmentRef](assessments); err != nil {
		return PersonDetail{}, err
	}
	if detail.RuleCoverage, err = s.personRuleCoverage(ctx, id); err != nil {
		return PersonDetail{}, err
	}
	if detail.Paths, err = s.personPaths(ctx, id); err != nil {
		return PersonDetail{}, err
	}
	return detail, nil
}

// personPaths carica gli assegnamenti percorso della persona con il
// progresso calcolato in lettura (#161, decisioni #159 punto 8): nessuna
// persistenza, un'assegnazione con completedOn valorizzato non si riapre
// ma il progresso resta informativo.
func (s *SQLStore) personPaths(ctx context.Context, employeeID string) ([]PersonPathRef, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT elp.path_id::text, lp.name, elp.started_on::text,
  COALESCE(elp.target_completion::text, ''), COALESCE(elp.completed_on::text, ''), COALESCE(elp.notes, '')
FROM training.employee_learning_path elp
JOIN training.learning_path lp ON lp.id = elp.path_id
WHERE elp.employee_id = $1::uuid
ORDER BY lp.name`, employeeID)
	if err != nil {
		return nil, fmt.Errorf("list training person paths: %w", err)
	}
	defer rows.Close()

	result := make([]PersonPathRef, 0)
	for rows.Next() {
		var ref PersonPathRef
		if err := rows.Scan(&ref.PathID, &ref.PathName, &ref.StartedOn, &ref.TargetCompletion, &ref.CompletedOn, &ref.Notes); err != nil {
			return nil, fmt.Errorf("scan training person path: %w", err)
		}
		result = append(result, ref)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range result {
		steps, err := s.pathSteps(ctx, s.db, result[i].PathID)
		if err != nil {
			return nil, err
		}
		progress, err := s.assignmentProgress(ctx, s.db, employeeID, result[i].CompletedOn, steps)
		if err != nil {
			return nil, err
		}
		result[i].PathProgress = progress
	}
	return result, nil
}

// personRuleCoverage riusa il nucleo condiviso di store_coverage.go (D3): per
// ogni regola attiva a platea che include la persona espone bisogno,
// copertura e scadenza. Le regole a posizioni non hanno platea per persona e
// restano escluse; un'area senza gruppo collegato non blocca la scheda, la
// regola coinvolta viene semplicemente saltata.
func (s *SQLStore) personRuleCoverage(ctx context.Context, employeeID string) ([]PersonRuleCoverageRef, error) {
	rules, err := s.activeQueueRules(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]PersonRuleCoverageRef, 0)
	for _, rule := range rules {
		if rule.Target == nil {
			continue
		}
		status, err := s.ruleCoverageStatus(ctx, s.db, rule.ruleFacts)
		if err != nil {
			if _, ok := asAppError(err); ok {
				continue
			}
			return nil, err
		}
		member := false
		for _, candidate := range status.Population {
			if candidate == employeeID {
				member = true
				break
			}
		}
		if !member {
			continue
		}
		result = append(result, PersonRuleCoverageRef{
			RuleID:   rule.ID,
			RuleName: rule.Name,
			Need:     rule.Need,
			Covered:  status.Covered[employeeID],
			Deadline: rule.Deadline.Format("2006-01-02"),
		})
	}
	return result, nil
}
