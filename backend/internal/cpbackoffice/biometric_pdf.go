package cpbackoffice

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/sciacco/mrsmith/internal/platform/httputil"
)

const activeBiometricUsersFilename = "accessi-biometrici-utenti-attivi.pdf"

// activeBiometricUsersQuery returns users whose biometric request history has
// more completed activations than completed deactivations. This is the business
// definition of "active" for the PDF export; it is intentionally independent
// from the visible tab/search state in the browser.
const activeBiometricUsersQuery = `WITH request_balance AS (
    SELECT
        br.user_struct_id,
        SUM(CASE br.request_type::text
            WHEN 'activation' THEN 1
            WHEN 'deactivation' THEN -1
            ELSE 0
        END) AS biometric_balance
    FROM customers.biometric_request br
    WHERE br.request_completed IS TRUE
    GROUP BY br.user_struct_id
    HAVING SUM(CASE br.request_type::text
        WHEN 'activation' THEN 1
        WHEN 'deactivation' THEN -1
        ELSE 0
    END) > 0
),
latest_activation AS (
    SELECT DISTINCT ON (br.user_struct_id)
        br.user_struct_id,
        br.request_approval_date AS data_conferma
    FROM customers.biometric_request br
    WHERE br.request_type::text = 'activation'
      AND br.request_completed IS TRUE
    ORDER BY br.user_struct_id, br.request_date DESC, br.id DESC
)
SELECT
    COALESCE(c.name, '') AS azienda,
    COALESCE(us.first_name, '') AS nome,
    COALESCE(us.last_name, '') AS cognome,
    COALESCE(us.primary_email, '') AS email,
    la.data_conferma
FROM request_balance rb
JOIN customers.user_struct us ON us.id = rb.user_struct_id
LEFT JOIN customers.customer c ON c.id = us.customer_id
LEFT JOIN latest_activation la ON la.user_struct_id = rb.user_struct_id
ORDER BY azienda ASC, nome ASC, cognome ASC, email ASC`

type activeBiometricUser struct {
	Azienda      string
	Nome         string
	Cognome      string
	Email        string
	DataConferma *time.Time
}

type activeBiometricCompanySection struct {
	Azienda string
	Users   []activeBiometricUser
}

func handleDownloadActiveBiometricUsersPDF(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireMistra(deps) {
			writeDatabaseUnavailable(w)
			return
		}

		users, err := listActiveBiometricUsers(r, deps.Mistra)
		if err != nil {
			dbFailure(w, r, err, "biometric.activeUsersPDF.list")
			return
		}

		pdfBytes, err := renderActiveBiometricUsersPDF(users, time.Now())
		if err != nil {
			httputil.InternalError(w, r, err, "pdf generation failed",
				"component", "cpbackoffice", "operation", "biometric.activeUsersPDF.render")
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, activeBiometricUsersFilename))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdfBytes)
	}
}

func listActiveBiometricUsers(r *http.Request, db *sql.DB) ([]activeBiometricUser, error) {
	rows, err := db.QueryContext(r.Context(), activeBiometricUsersQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]activeBiometricUser, 0)
	for rows.Next() {
		var (
			item         activeBiometricUser
			azienda      sql.NullString
			nome         sql.NullString
			cognome      sql.NullString
			email        sql.NullString
			dataConferma sql.NullTime
		)
		if err := rows.Scan(&azienda, &nome, &cognome, &email, &dataConferma); err != nil {
			return nil, err
		}
		if azienda.Valid {
			item.Azienda = azienda.String
		}
		if nome.Valid {
			item.Nome = nome.String
		}
		if cognome.Valid {
			item.Cognome = cognome.String
		}
		if email.Valid {
			item.Email = email.String
		}
		if dataConferma.Valid {
			t := dataConferma.Time
			item.DataConferma = &t
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func renderActiveBiometricUsersPDF(users []activeBiometricUser, generatedAt time.Time) ([]byte, error) {
	title := fmt.Sprintf("Accessi biometrici - %d utenti attivi al %s", len(users), generatedAt.Format("02/01/2006"))

	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithLeftMargin(16).
		WithRightMargin(16).
		WithTopMargin(18).
		WithBottomMargin(16).
		WithTitle(title, true).
		WithCreator("MrSmith CP Backoffice", true).
		WithCreationDate(generatedAt).
		Build()

	m := maroto.New(cfg)
	m.AddAutoRow(text.NewCol(12, title, props.Text{
		Style: fontstyle.Bold,
		Size:  15,
		Color: pdfColor(20, 24, 33),
	}))
	m.AddRow(8)

	if len(users) == 0 {
		m.AddAutoRow(text.NewCol(12, "Nessun utente attivo.", props.Text{
			Size:  10,
			Color: pdfColor(55, 65, 81),
		}))
		doc, err := m.Generate()
		if err != nil {
			return nil, err
		}
		return doc.GetBytes(), nil
	}

	for _, section := range groupActiveBiometricUsers(users) {
		m.AddAutoRow(
			col.New(12).Add(text.New(section.Azienda, props.Text{
				Style:  fontstyle.Bold,
				Size:   10,
				Left:   2,
				Top:    1.2,
				Bottom: 2.4,
				Color:  pdfColor(22, 30, 46),
			})).WithStyle(&props.Cell{
				BackgroundColor: pdfColor(238, 241, 246),
			}),
		)
		m.AddRow(2)

		for _, user := range section.Users {
			m.AddAutoRow(
				text.NewCol(9, activeBiometricRequesterName(user), props.Text{
					Size:  9,
					Left:  5,
					Color: pdfColor(20, 24, 33),
				}),
				text.NewCol(3, formatPDFTimestamp(user.DataConferma), props.Text{
					Size:  9,
					Align: align.Right,
					Color: pdfColor(20, 24, 33),
				}),
			)

			if strings.TrimSpace(user.Email) != "" && activeBiometricHasName(user) {
				m.AddAutoRow(
					text.NewCol(9, strings.TrimSpace(user.Email), props.Text{
						Size:  7.5,
						Left:  5,
						Color: pdfColor(107, 114, 128),
					}),
					text.NewCol(3, ""),
				)
			}
			m.AddRow(1.5)
		}

		m.AddRow(5)
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, err
	}
	return doc.GetBytes(), nil
}

func groupActiveBiometricUsers(users []activeBiometricUser) []activeBiometricCompanySection {
	sortedUsers := append([]activeBiometricUser(nil), users...)
	sort.SliceStable(sortedUsers, func(i, j int) bool {
		leftCompany := activeBiometricCompanyName(sortedUsers[i])
		rightCompany := activeBiometricCompanyName(sortedUsers[j])
		if cmp := strings.Compare(strings.ToLower(leftCompany), strings.ToLower(rightCompany)); cmp != 0 {
			return cmp < 0
		}

		leftName := activeBiometricRequesterName(sortedUsers[i])
		rightName := activeBiometricRequesterName(sortedUsers[j])
		if cmp := strings.Compare(strings.ToLower(leftName), strings.ToLower(rightName)); cmp != 0 {
			return cmp < 0
		}

		return strings.ToLower(strings.TrimSpace(sortedUsers[i].Email)) < strings.ToLower(strings.TrimSpace(sortedUsers[j].Email))
	})

	sections := make([]activeBiometricCompanySection, 0)
	for _, user := range sortedUsers {
		azienda := activeBiometricCompanyName(user)
		if len(sections) == 0 || sections[len(sections)-1].Azienda != azienda {
			sections = append(sections, activeBiometricCompanySection{Azienda: azienda})
		}
		sections[len(sections)-1].Users = append(sections[len(sections)-1].Users, user)
	}
	return sections
}

func activeBiometricCompanyName(user activeBiometricUser) string {
	if strings.TrimSpace(user.Azienda) == "" {
		return "Senza azienda"
	}
	return strings.TrimSpace(user.Azienda)
}

func activeBiometricRequesterName(user activeBiometricUser) string {
	name := strings.TrimSpace(strings.Join([]string{user.Nome, user.Cognome}, " "))
	if name == "" {
		return nonEmptyPDFValue(user.Email)
	}
	return name
}

func activeBiometricHasName(user activeBiometricUser) bool {
	return strings.TrimSpace(strings.Join([]string{user.Nome, user.Cognome}, " ")) != ""
}

func formatPDFTimestamp(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("02/01/2006 15:04")
}

func nonEmptyPDFValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return strings.TrimSpace(value)
}

func pdfColor(red, green, blue int) *props.Color {
	return &props.Color{Red: red, Green: green, Blue: blue}
}
