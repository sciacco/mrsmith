// Package fattureincloud wraps the Fatture in Cloud API v2 for the read-only
// use the backend needs: listing the received documents still to be recorded
// and downloading their XML.
//
// Limits (developers.fattureincloud.it/docs/basics/limits-and-quotas): 300
// requests per rolling 5 minutes (429 + Retry-After), 1000 per hour and 40000
// per month per company (403 + Retry-After). The token is shared with the
// billing CLI, so the client keeps a minimum cadence between calls, honors
// Retry-After up to a cap, retries transient failures with growing waits and
// stops before eating into a configured hourly reserve. Downloads go to an
// external store that does not count against the limits.
package fattureincloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL       = "https://api-v2.fattureincloud.it"
	PerPage              = 100
	defaultMinInterval   = 1100 * time.Millisecond
	defaultHourlyReserve = 100
	defaultMaxRetryAfter = 15 * time.Minute
	maxAttempts          = 5
	maxDownloadAttempts  = 3
	backoffUnit          = time.Second
	maxBody              = 8 << 20
)

// ErrHourlyReserve is returned once the hourly remaining requests reported by
// the server drop below the configured reserve. The caller stops and resumes
// on its next run.
var ErrHourlyReserve = errors.New("fattureincloud: riserva oraria di richieste raggiunta")

// Config configures a Client. Zero values take the defaults above.
type Config struct {
	Token         string
	CompanyID     int64
	BaseURL       string
	HTTPClient    *http.Client
	MinInterval   time.Duration
	HourlyReserve int
	MaxRetryAfter time.Duration
}

// Wait is one pause the client took, kept for the caller's diagnostics.
type Wait struct {
	At      time.Time `json:"at"`
	Seconds float64   `json:"seconds"`
	Reason  string    `json:"reason"`
}

// Stats is the client activity since the last ResetStats. Remaining quotas
// are -1 until the server reports them.
type Stats struct {
	Requests         int    `json:"requests"`
	HourlyRemaining  int    `json:"hourly_remaining"`
	MonthlyRemaining int    `json:"monthly_remaining"`
	Waits            []Wait `json:"waits"`
}

type Client struct {
	cfg Config

	mu         sync.Mutex
	lastCall   time.Time
	stats      Stats
	reserveHit bool
}

func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	if cfg.MinInterval <= 0 {
		cfg.MinInterval = defaultMinInterval
	}
	if cfg.HourlyReserve <= 0 {
		cfg.HourlyReserve = defaultHourlyReserve
	}
	if cfg.MaxRetryAfter <= 0 {
		cfg.MaxRetryAfter = defaultMaxRetryAfter
	}
	c := &Client{cfg: cfg}
	c.ResetStats()
	return c
}

// CompanyID is the configured company, 0 when it must be resolved.
func (c *Client) CompanyID() int64 { return c.cfg.CompanyID }

// ResetStats clears counters and waits and re-arms the hourly reserve check.
func (c *Client) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats = Stats{HourlyRemaining: -1, MonthlyRemaining: -1, Waits: []Wait{}}
	c.reserveHit = false
}

// Stats returns a copy of the activity since the last reset.
func (c *Client) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.stats
	out.Waits = append([]Wait{}, c.stats.Waits...)
	return out
}

// ResolveCompany returns the configured company or, when none is configured,
// the first company associated with the token.
func (c *Client) ResolveCompany(ctx context.Context) (int64, error) {
	if c.cfg.CompanyID != 0 {
		return c.cfg.CompanyID, nil
	}
	var res struct {
		Data struct {
			Companies []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"companies"`
		} `json:"data"`
	}
	if err := c.get(ctx, c.cfg.BaseURL+"/user/companies", &res); err != nil {
		return 0, err
	}
	if len(res.Data.Companies) == 0 {
		return 0, errors.New("fattureincloud: nessuna azienda associata al token")
	}
	return res.Data.Companies[0].ID, nil
}

// ReceivedDocument is a document in the "to be recorded" box.
type ReceivedDocument struct {
	ID            int64   `json:"id"`
	Date          string  `json:"date"`          // reception date, possibly with time
	EmissionDate  string  `json:"emission_date"` // document date
	Filename      string  `json:"filename"`      // original SDI file name, decides the extension
	DocumentType  string  `json:"document_type"` // expense, passive_credit_note, ...
	EINumber      string  `json:"ei_number"`
	SupplierName  string  `json:"supplier_name"`
	AttachmentURL string  `json:"attachment_url"` // signed, expires after about two hours
	AmountGross   float64 `json:"amount_gross"`
}

// ReceivedOn is the reception date without time.
func (d ReceivedDocument) ReceivedOn() (time.Time, bool) {
	if len(d.Date) < 10 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", d.Date[:10])
	return t, err == nil
}

// Extension is the lower-case extension of the original SDI file name
// (".xml", ".p7m", ".pdf", ".zip"; ".xml" when unknown).
func (d ReceivedDocument) Extension() string {
	n := strings.ToLower(d.Filename)
	for _, ext := range []string{".xml", ".p7m", ".pdf", ".zip"} {
		if strings.HasSuffix(n, ext) {
			return ext
		}
	}
	if i := strings.LastIndex(n, "."); i >= 0 && len(n)-i <= 5 {
		return n[i:]
	}
	return ".xml"
}

// Page is one page of the pending received documents list.
type Page struct {
	CurrentPage int                `json:"current_page"`
	LastPage    int                `json:"last_page"`
	NextPageURL *string            `json:"next_page_url"`
	Data        []ReceivedDocument `json:"data"`
}

// HasNext reports whether another page follows.
func (p Page) HasNext() bool {
	return len(p.Data) > 0 && p.NextPageURL != nil && *p.NextPageURL != ""
}

// ListPendingReceived lists one page of the documents still to be recorded,
// sorted by reception date. A non-zero from/to restricts the reception date
// to [from, to).
func (c *Client) ListPendingReceived(ctx context.Context, companyID int64, page int, from, to time.Time) (Page, error) {
	params := url.Values{}
	params.Set("fieldset", "detailed")
	params.Set("sort", "date")
	params.Set("per_page", strconv.Itoa(PerPage))
	params.Set("page", strconv.Itoa(page))
	if !from.IsZero() && !to.IsZero() {
		params.Set("q", fmt.Sprintf("date >= '%s' and date < '%s'", from.Format("2006-01-02"), to.Format("2006-01-02")))
	}
	u := fmt.Sprintf("%s/c/%d/received_documents/pending?%s", c.cfg.BaseURL, companyID, params.Encode())
	var out Page
	if err := c.get(ctx, u, &out); err != nil {
		return Page{}, fmt.Errorf("pagina %d: %w", page, err)
	}
	return out, nil
}

// Download fetches an attachment. The signed URL lives on an external store
// outside the API limits: only network and server errors are retried.
func (c *Client) Download(ctx context.Context, u string) ([]byte, error) {
	var last string
	for attempt := 1; attempt <= maxDownloadAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.cfg.HTTPClient.Do(req)
		if err != nil {
			last = err.Error()
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBody))
			resp.Body.Close()
			switch {
			case readErr != nil:
				last = readErr.Error()
			case resp.StatusCode == http.StatusOK:
				return body, nil
			case resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests:
				return nil, fmt.Errorf("download: HTTP %s", resp.Status)
			default:
				last = "HTTP " + resp.Status
			}
		}
		if attempt == maxDownloadAttempts {
			break
		}
		if err := c.wait(ctx, backoff(attempt), "download fallito ("+last+")"); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("download: %s", last)
}

// get runs one authenticated GET and decodes the JSON body into out.
func (c *Client) get(ctx context.Context, u string, out any) error {
	var last error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if c.reserveReached() {
			return ErrHourlyReserve
		}
		if err := c.pace(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
		req.Header.Set("Accept", "application/json")
		resp, err := c.cfg.HTTPClient.Do(req)
		c.countRequest()
		if err != nil {
			last = err
			if werr := c.wait(ctx, backoff(attempt), "errore di rete: "+err.Error()); werr != nil {
				return werr
			}
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		resp.Body.Close()
		c.recordQuotas(resp.Header)

		switch {
		case resp.StatusCode == http.StatusOK:
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("risposta non valida: %w", err)
			}
			return nil
		case resp.StatusCode == http.StatusTooManyRequests,
			resp.StatusCode == http.StatusForbidden && resp.Header.Get("Retry-After") != "":
			pause := retryAfter(resp.Header, backoff(attempt))
			if pause > c.cfg.MaxRetryAfter {
				return fmt.Errorf("HTTP %s: il server chiede di attendere %s, oltre il massimo (%s)",
					resp.Status, pause.Round(time.Second), c.cfg.MaxRetryAfter)
			}
			last = fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
			if err := c.wait(ctx, pause, "limite di richieste raggiunto (HTTP "+resp.Status+")"); err != nil {
				return err
			}
		case resp.StatusCode >= 500:
			last = fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
			if err := c.wait(ctx, backoff(attempt), "errore del server (HTTP "+resp.Status+")"); err != nil {
				return err
			}
		default:
			return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
	}
	return fmt.Errorf("tentativi esauriti: %w", last)
}

func (c *Client) reserveReached() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reserveHit
}

func (c *Client) countRequest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.Requests++
}

func (c *Client) recordQuotas(h http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n, err := strconv.Atoi(h.Get("RateLimit-HourlyRemaining")); err == nil {
		c.stats.HourlyRemaining = n
		if n < c.cfg.HourlyReserve {
			c.reserveHit = true
		}
	}
	if n, err := strconv.Atoi(h.Get("RateLimit-MonthlyRemaining")); err == nil {
		c.stats.MonthlyRemaining = n
	}
}

// pace waits until the minimum interval since the previous call has passed.
func (c *Client) pace(ctx context.Context) error {
	c.mu.Lock()
	d := c.cfg.MinInterval - time.Since(c.lastCall)
	c.lastCall = time.Now().Add(max(d, 0))
	c.mu.Unlock()
	if d <= 0 {
		return nil
	}
	return sleep(ctx, d)
}

func (c *Client) wait(ctx context.Context, d time.Duration, reason string) error {
	c.mu.Lock()
	c.stats.Waits = append(c.stats.Waits, Wait{At: time.Now(), Seconds: d.Seconds(), Reason: reason})
	c.mu.Unlock()
	return sleep(ctx, d)
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// backoff grows 2, 4, 8, 16, 32 s.
func backoff(attempt int) time.Duration {
	return time.Duration(1<<uint(attempt)) * backoffUnit
}

// retryAfter reads Retry-After (seconds or HTTP date), falling back to def.
func retryAfter(h http.Header, def time.Duration) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return def
	}
	if sec, err := strconv.Atoi(v); err == nil {
		return time.Duration(sec)*time.Second + time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d + time.Second
		}
	}
	return def
}
