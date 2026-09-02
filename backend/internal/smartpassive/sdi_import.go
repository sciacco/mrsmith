package smartpassive

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/fattureincloud"
)

// SDIImporter is the periodic import of the SDI invoices from Fatture in
// Cloud into smartpassive.sdi_invoice. Nobody watches it, so it is built to
// leave no gaps on its own:
//
//   - every run is idempotent (source_id is unique; existing rows are skipped);
//   - an incremental run reads from the highest reception date already stored,
//     minus an overlap, up to today: a skipped or interrupted run is recovered
//     by the next one without extra state;
//   - a full run re-reads the whole box (which is never emptied) and downloads
//     only what is missing; it happens weekly and whenever the table has no
//     reception date yet, when it also fills the reception date of the rows
//     loaded before the column existed.
//
// Every run, successful or not, is recorded in smartpassive.sdi_import_run
// with the complete diagnostics in the details column.
type SDIImporter struct {
	db     *sql.DB
	client *fattureincloud.Client
	logger *slog.Logger

	overlap   time.Duration
	fullEvery time.Duration
}

const (
	sdiImportOverlap   = 3 * 24 * time.Hour
	sdiImportFullEvery = 7 * 24 * time.Hour
	sdiImportInterval  = 8 * time.Hour

	sdiImportModeIncremental = "incremental"
	sdiImportModeFull        = "full"

	sdiImportOutcomeOK      = "ok"
	sdiImportOutcomeFailed  = "failed"
	sdiImportOutcomeStopped = "stopped"
)

func NewSDIImporter(db *sql.DB, client *fattureincloud.Client, logger *slog.Logger) *SDIImporter {
	if logger == nil {
		logger = slog.Default()
	}
	return &SDIImporter{
		db:        db,
		client:    client,
		logger:    logger.With("component", component, "worker", "sdi_import"),
		overlap:   sdiImportOverlap,
		fullEvery: sdiImportFullEvery,
	}
}

// Run executes one import at start, then one every interval until ctx ends.
func (im *SDIImporter) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = sdiImportInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := im.RunOnce(ctx); err != nil {
			im.logger.Warn("sdi import run failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// sdiImportDoc identifies one document in the run diagnostics.
type sdiImportDoc struct {
	SourceID   int64  `json:"source_id"`
	FileName   string `json:"file_name"`
	Supplier   string `json:"supplier"`
	Number     string `json:"number"`
	ReceivedOn string `json:"received_on"`
}

// sdiImportProblem is one document the run could not import, with the reason.
type sdiImportProblem struct {
	sdiImportDoc
	Reason string `json:"reason"`
}

// sdiImportDetails is the complete diagnostics of one run, stored as JSON.
type sdiImportDetails struct {
	Mode             string               `json:"mode"`
	ModeReason       string               `json:"mode_reason"`
	AnchorReceivedOn *string              `json:"anchor_received_on"`
	WindowFrom       *string              `json:"window_from"`
	WindowTo         *string              `json:"window_to"`
	CompanyID        int64                `json:"company_id"`
	Pages            int                  `json:"pages"`
	Listed           int                  `json:"listed"`
	AlreadyPresent   int                  `json:"already_present"`
	Inserted         []sdiImportDoc       `json:"inserted"`
	ReceivedOnFilled []int64              `json:"received_on_filled"`
	Skipped          []sdiImportProblem   `json:"skipped"`
	Errors           []sdiImportProblem   `json:"errors"`
	API              fattureincloud.Stats `json:"api"`
	StopReason       string               `json:"stop_reason,omitempty"`
	Error            string               `json:"error,omitempty"`
	DurationSeconds  float64              `json:"duration_seconds"`
}

// RunOnce performs one import and records it.
func (im *SDIImporter) RunOnce(ctx context.Context) error {
	if im == nil || im.db == nil || im.client == nil {
		return errors.New("sdi import: database o client non configurati")
	}
	started := time.Now()
	details := sdiImportDetails{
		Inserted:         []sdiImportDoc{},
		ReceivedOnFilled: []int64{},
		Skipped:          []sdiImportProblem{},
		Errors:           []sdiImportProblem{},
	}
	im.client.ResetStats()

	from, to, err := im.plan(ctx, started, &details)
	if err != nil {
		return im.finish(ctx, 0, started, sdiImportOutcomeFailed, &details, fmt.Errorf("pianificazione: %w", err))
	}
	runID, err := im.openRun(ctx, started, details.Mode)
	if err != nil {
		return fmt.Errorf("sdi import: apertura esecuzione: %w", err)
	}

	companyID, err := im.client.ResolveCompany(ctx)
	if err != nil {
		return im.finish(ctx, runID, started, sdiImportOutcomeFailed, &details, fmt.Errorf("azienda: %w", err))
	}
	details.CompanyID = companyID

	outcome := sdiImportOutcomeOK
	var runErr error
	for page := 1; ; page++ {
		pg, err := im.client.ListPendingReceived(ctx, companyID, page, from, to)
		if err != nil {
			outcome, runErr = classify(err)
			break
		}
		details.Pages++
		stopped := false
		for _, doc := range pg.Data {
			details.Listed++
			if err := im.importDocument(ctx, doc, &details); err != nil {
				outcome, runErr = classify(err)
				stopped = true
				break
			}
		}
		if stopped || !pg.HasNext() {
			break
		}
	}
	return im.finish(ctx, runID, started, outcome, &details, runErr)
}

// classify maps a run-stopping error to the outcome recorded.
func classify(err error) (string, error) {
	if errors.Is(err, fattureincloud.ErrHourlyReserve) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return sdiImportOutcomeStopped, err
	}
	return sdiImportOutcomeFailed, err
}

// plan chooses the mode and the reception-date window of the run.
func (im *SDIImporter) plan(ctx context.Context, now time.Time, details *sdiImportDetails) (from, to time.Time, err error) {
	var anchor sql.NullTime
	if err := im.db.QueryRowContext(ctx, `SELECT max(received_on) FROM smartpassive.sdi_invoice`).Scan(&anchor); err != nil {
		return from, to, err
	}
	var lastFull sql.NullTime
	if err := im.db.QueryRowContext(ctx, `SELECT max(started_at) FROM smartpassive.sdi_import_run WHERE mode = $1 AND outcome = $2`,
		sdiImportModeFull, sdiImportOutcomeOK).Scan(&lastFull); err != nil {
		return from, to, err
	}

	switch {
	case !anchor.Valid:
		details.Mode = sdiImportModeFull
		details.ModeReason = "nessuna data di ricezione in tabella"
	case !lastFull.Valid:
		details.Mode = sdiImportModeFull
		details.ModeReason = "nessuna passata completa riuscita"
	case now.Sub(lastFull.Time) >= im.fullEvery:
		details.Mode = sdiImportModeFull
		details.ModeReason = "ultima passata completa del " + lastFull.Time.Format("2006-01-02")
	default:
		details.Mode = sdiImportModeIncremental
		details.ModeReason = "finestra dalla ricezione massima meno la sovrapposizione"
	}
	if anchor.Valid {
		s := anchor.Time.Format("2006-01-02")
		details.AnchorReceivedOn = &s
	}
	if details.Mode == sdiImportModeIncremental {
		from = anchor.Time.Add(-im.overlap)
		to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
		f, t := from.Format("2006-01-02"), to.Format("2006-01-02")
		details.WindowFrom, details.WindowTo = &f, &t
	}
	return from, to, nil
}

// importDocument brings one listed document into the table. Documents already
// present are not downloaded; those without a reception date get it filled.
// Per-document failures are recorded and do not stop the run; the error
// returned is only a run-stopping one (hourly reserve, context).
func (im *SDIImporter) importDocument(ctx context.Context, doc fattureincloud.ReceivedDocument, details *sdiImportDetails) error {
	entry := sdiImportDoc{SourceID: doc.ID, FileName: doc.Filename, Supplier: doc.SupplierName, Number: doc.EINumber}
	var receivedOn *time.Time
	if d, ok := doc.ReceivedOn(); ok {
		receivedOn = &d
		entry.ReceivedOn = d.Format("2006-01-02")
	}

	var hasReceived sql.NullBool
	err := im.db.QueryRowContext(ctx, `SELECT received_on IS NOT NULL FROM smartpassive.sdi_invoice WHERE source_id = $1`, doc.ID).Scan(&hasReceived)
	switch {
	case err == nil && hasReceived.Valid && hasReceived.Bool:
		details.AlreadyPresent++
		return nil
	case err == nil:
		details.AlreadyPresent++
		if receivedOn == nil {
			return nil
		}
		if _, err := im.db.ExecContext(ctx, `UPDATE smartpassive.sdi_invoice SET received_on = $2 WHERE source_id = $1 AND received_on IS NULL`, doc.ID, *receivedOn); err != nil {
			details.Errors = append(details.Errors, sdiImportProblem{entry, "aggiornamento data di ricezione: " + err.Error()})
			return ctxErr(ctx)
		}
		details.ReceivedOnFilled = append(details.ReceivedOnFilled, doc.ID)
		return nil
	case !errors.Is(err, sql.ErrNoRows):
		details.Errors = append(details.Errors, sdiImportProblem{entry, "lettura: " + err.Error()})
		return ctxErr(ctx)
	}

	if ext := doc.Extension(); ext != ".xml" {
		details.Skipped = append(details.Skipped, sdiImportProblem{entry, "formato non gestito: " + ext})
		return nil
	}
	if doc.AttachmentURL == "" {
		details.Skipped = append(details.Skipped, sdiImportProblem{entry, "documento senza allegato"})
		return nil
	}
	raw, err := im.client.Download(ctx, doc.AttachmentURL)
	if err != nil {
		details.Errors = append(details.Errors, sdiImportProblem{entry, err.Error()})
		return ctxErr(ctx)
	}
	header, err := parseSDIHeader(raw)
	if err != nil {
		details.Errors = append(details.Errors, sdiImportProblem{entry, err.Error()})
		return nil
	}
	fileName := doc.Filename
	if fileName == "" {
		fileName = strconv.FormatInt(doc.ID, 10) + ".xml"
	}
	tag, err := im.db.ExecContext(ctx, `
		INSERT INTO smartpassive.sdi_invoice
		    (source_id, file_name, supplier_vat, supplier_name, document_type, document_number, document_date, total_amount, xml, received_on)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (source_id) DO NOTHING`,
		doc.ID, fileName, header.supplierVAT, header.supplierName, header.docType, header.docNumber, header.docDate, header.total, string(raw), receivedOn)
	if err != nil {
		details.Errors = append(details.Errors, sdiImportProblem{entry, "inserimento: " + err.Error()})
		return ctxErr(ctx)
	}
	if n, _ := tag.RowsAffected(); n == 0 {
		details.AlreadyPresent++
		return nil
	}
	details.Inserted = append(details.Inserted, entry)
	return nil
}

// ctxErr turns a database failure into a run stop only when the context is
// gone; otherwise the document is skipped and the run goes on.
func ctxErr(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (im *SDIImporter) openRun(ctx context.Context, started time.Time, mode string) (int64, error) {
	var id int64
	err := im.db.QueryRowContext(ctx,
		`INSERT INTO smartpassive.sdi_import_run (started_at, mode) VALUES ($1, $2) RETURNING id`,
		started, mode).Scan(&id)
	return id, err
}

// finish records the outcome and the diagnostics. When runID is 0 the run
// failed before it could be opened and is inserted whole here.
func (im *SDIImporter) finish(ctx context.Context, runID int64, started time.Time, outcome string, details *sdiImportDetails, runErr error) error {
	if runErr != nil {
		details.Error = runErr.Error()
		if outcome == sdiImportOutcomeStopped {
			details.StopReason = runErr.Error()
		}
	}
	details.API = im.client.Stats()
	details.DurationSeconds = time.Since(started).Seconds()
	payload, err := json.Marshal(details)
	if err != nil {
		payload = []byte(`{"error":"diagnostica non serializzabile"}`)
	}
	// The run must be recorded even when the context that drove it is gone.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	mode := details.Mode
	if mode == "" {
		mode = sdiImportModeIncremental
	}
	if runID == 0 {
		_, err = im.db.ExecContext(wctx, `
			INSERT INTO smartpassive.sdi_import_run (started_at, finished_at, mode, outcome, inserted, details)
			VALUES ($1, now(), $2, $3, $4, $5::jsonb)`,
			started, mode, outcome, len(details.Inserted), string(payload))
	} else {
		_, err = im.db.ExecContext(wctx, `
			UPDATE smartpassive.sdi_import_run
			SET finished_at = now(), outcome = $2, inserted = $3, details = $4::jsonb
			WHERE id = $1`,
			runID, outcome, len(details.Inserted), string(payload))
	}
	if err != nil {
		im.logger.Error("sdi import run not recorded", "error", err, "outcome", outcome)
	}
	im.logger.Info("sdi import run finished",
		"mode", details.Mode, "outcome", outcome, "listed", details.Listed,
		"inserted", len(details.Inserted), "already_present", details.AlreadyPresent,
		"skipped", len(details.Skipped), "errors", len(details.Errors),
		"api_requests", details.API.Requests, "hourly_remaining", details.API.HourlyRemaining)
	if runErr != nil {
		return fmt.Errorf("sdi import: %w", runErr)
	}
	return nil
}

// sdiHeader holds the header keys of one FatturaPA file, as stored in the
// table. Tags without namespace accept every schema version.
type sdiHeader struct {
	supplierVAT  string
	supplierName string
	docType      string
	docNumber    string
	docDate      *time.Time
	total        *float64
}

type fatturaPAHeader struct {
	Header struct {
		Cedente struct {
			Dati struct {
				IdFiscaleIVA struct {
					IdPaese  string `xml:"IdPaese"`
					IdCodice string `xml:"IdCodice"`
				} `xml:"IdFiscaleIVA"`
				CodiceFiscale string `xml:"CodiceFiscale"`
				Anagrafica    struct {
					Denominazione string `xml:"Denominazione"`
					Nome          string `xml:"Nome"`
					Cognome       string `xml:"Cognome"`
				} `xml:"Anagrafica"`
			} `xml:"DatiAnagrafici"`
		} `xml:"CedentePrestatore"`
	} `xml:"FatturaElettronicaHeader"`
	Body []struct {
		Generali struct {
			Documento struct {
				TipoDocumento string `xml:"TipoDocumento"`
				Numero        string `xml:"Numero"`
				Data          string `xml:"Data"`
				ImportoTotale string `xml:"ImportoTotaleDocumento"`
			} `xml:"DatiGeneraliDocumento"`
		} `xml:"DatiGenerali"`
	} `xml:"FatturaElettronicaBody"`
}

func parseSDIHeader(raw []byte) (sdiHeader, error) {
	var t fatturaPAHeader
	if err := xml.Unmarshal(raw, &t); err != nil {
		return sdiHeader{}, fmt.Errorf("XML non valido: %w", err)
	}
	if len(t.Body) == 0 {
		return sdiHeader{}, errors.New("XML senza corpo fattura")
	}
	ced := t.Header.Cedente.Dati
	doc := t.Body[0].Generali.Documento
	h := sdiHeader{
		supplierVAT:  strings.TrimSpace(ced.IdFiscaleIVA.IdPaese) + strings.TrimSpace(ced.IdFiscaleIVA.IdCodice),
		supplierName: strings.TrimSpace(ced.Anagrafica.Denominazione),
		docType:      strings.TrimSpace(doc.TipoDocumento),
		docNumber:    strings.TrimSpace(doc.Numero),
	}
	if h.supplierName == "" {
		h.supplierName = strings.TrimSpace(ced.Anagrafica.Nome + " " + ced.Anagrafica.Cognome)
	}
	if h.supplierVAT == "" {
		h.supplierVAT = strings.TrimSpace(ced.CodiceFiscale)
	}
	// The date may carry a zone suffix ("2026-05-31Z", "2026-06-30+02:00").
	if data := strings.TrimSpace(doc.Data); len(data) >= 10 {
		if d, err := time.Parse("2006-01-02", data[:10]); err == nil {
			h.docDate = &d
		}
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(doc.ImportoTotale), 64); err == nil {
		h.total = &v
	}
	return h, nil
}

// SDIImportRun is one recorded run, as exposed to the diagnostics page.
type SDIImportRun struct {
	ID         int64      `json:"id"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Mode       string     `json:"mode"`
	Outcome    *string    `json:"outcome"`
	Inserted   int        `json:"inserted"`
}

// SDIImportStatus is the single datum the diagnostics page shows: when the
// import last succeeded, plus the last run whatever its outcome.
type SDIImportStatus struct {
	LastSuccess *SDIImportRun `json:"last_success"`
	LastRun     *SDIImportRun `json:"last_run"`
}

const sdiImportRunSelect = `SELECT id, started_at, finished_at, mode, outcome, inserted
FROM smartpassive.sdi_import_run`

func (h *Handler) loadSDIImportStatus(ctx context.Context) (SDIImportStatus, error) {
	var status SDIImportStatus
	last, err := h.scanSDIImportRun(ctx, sdiImportRunSelect+` ORDER BY started_at DESC LIMIT 1`)
	if err != nil {
		return status, err
	}
	status.LastRun = last
	ok, err := h.scanSDIImportRun(ctx, sdiImportRunSelect+` WHERE outcome = $1 ORDER BY started_at DESC LIMIT 1`, sdiImportOutcomeOK)
	if err != nil {
		return status, err
	}
	status.LastSuccess = ok
	return status, nil
}

func (h *Handler) scanSDIImportRun(ctx context.Context, query string, args ...any) (*SDIImportRun, error) {
	var (
		run      SDIImportRun
		finished sql.NullTime
		outcome  sql.NullString
	)
	err := h.anisettaDB.QueryRowContext(ctx, query, args...).Scan(&run.ID, &run.StartedAt, &finished, &run.Mode, &outcome, &run.Inserted)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if finished.Valid {
		run.FinishedAt = &finished.Time
	}
	if outcome.Valid {
		run.Outcome = &outcome.String
	}
	return &run, nil
}
