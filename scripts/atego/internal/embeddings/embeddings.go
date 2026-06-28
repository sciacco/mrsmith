// Package embeddings implementa un client per endpoint di embedding
// compatibile OpenAI (/v1/embeddings), con batching e retry.
// È la traduzione di src/ingest/embeddings.py.
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/cenkalti/backoff/v4"

	"github.com/sciacco/atego/internal/config"
)

// embeddingReq è il body POST verso {base}/embeddings.
type embeddingReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResp è la risposta OpenAI-shaped.
type embeddingResp struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

// Client è il client per gli embedding.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	batchSize  int
	maxRetries int
	http       *http.Client
	log        *slog.Logger
}

// New costruisce il client a partire dalla configurazione.
func New(cfg *config.Config, log *slog.Logger) *Client {
	return &Client{
		baseURL:    cfg.EmbeddingAPIBase,
		apiKey:     cfg.EmbeddingAPIKey,
		model:      cfg.EmbeddingModel,
		batchSize:  cfg.BatchSize,
		maxRetries: cfg.MaxRetries,
		http: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		log: log,
	}
}

// Model restituisce il nome del modello configurato.
func (c *Client) Model() string { return c.model }

// ProbeDimension embedda un testo di prova e restituisce la lunghezza del vettore.
func (c *Client) ProbeDimension(ctx context.Context) (int, error) {
	c.log.Info("probing embedding dimension", "model", c.model)
	vecs, err := c.embedBatch(ctx, []string{"dimension test"})
	if err != nil {
		return 0, fmt.Errorf("cannot determine embedding dimension: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return 0, fmt.Errorf("cannot determine embedding dimension: empty vector")
	}
	dim := len(vecs[0])
	c.log.Info("embedding dimension detected", "model", c.model, "dimension", dim)
	return dim, nil
}

// EmbedBatch embedda un batch (≤ BatchSize). I risultati sono in ordine di input.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return c.embedBatch(ctx, texts)
}

// EmbedAll suddivide i testi in batch, logga l'avanzamento e appiattisce i risultati.
func (c *Client) EmbedAll(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, 0, len(texts))
	total := len(texts)
	if total == 0 {
		return results, nil
	}
	totalBatches := (total + c.batchSize - 1) / c.batchSize
	for start := 0; start < total; start += c.batchSize {
		end := start + c.batchSize
		if end > total {
			end = total
		}
		batch := texts[start:end]
		batchIdx := start/c.batchSize + 1
		c.log.Info("embedding batch", "batch", batchIdx, "total_batches", totalBatches, "texts", len(batch))
		vecs, err := c.embedBatch(ctx, batch)
		if err != nil {
			return nil, err
		}
		results = append(results, vecs...)
	}
	return results, nil
}

// embedBatch esegue la chiamata API con retry e ordina per index.
func (c *Client) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embeddingReq{Model: c.model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	var vectors [][]float32
	var lastErr error
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 1 * time.Second
	bo.MaxInterval = 60 * time.Second
	bo.Reset()
	retryBO := backoff.WithMaxRetries(bo, uint64(c.maxRetries))

	err = backoff.Retry(func() error {
		vecs, callErr := c.doPost(ctx, body)
		if callErr != nil {
			if isTransient(callErr) {
				c.log.Warn("embedding call failed, retrying", "err", callErr)
				lastErr = callErr
				return callErr
			}
			return backoff.Permanent(callErr)
		}
		vectors = vecs
		return nil
	}, backoff.WithContext(retryBO, ctx))
	if err != nil {
		if lastErr != nil {
			return nil, fmt.Errorf("embedding call failed after retries: %w", err)
		}
		return nil, err
	}
	return vectors, nil
}

// doPost esegue una singola POST, decodifica e ordina per index.
func (c *Client) doPost(ctx context.Context, body []byte) ([][]float32, error) {
	url := c.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20)) // 256MB cap
	if err != nil {
		return nil, err
	}

	// errori transitori → statusErr rilevato da isTransient
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &httpError{Status: resp.StatusCode, Body: string(respBody)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &httpError{Status: resp.StatusCode, Body: string(respBody)}
	}

	var parsed embeddingResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("empty embedding data in response")
	}

	// ordina per index come fa il Python (l'API non garantisce l'ordine)
	sort.Slice(parsed.Data, func(i, j int) bool {
		return parsed.Data[i].Index < parsed.Data[j].Index
	})

	vectors := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		// cast float64 → float32 (richiesto dal client gRPC Qdrant)
		v := make([]float32, len(d.Embedding))
		for j, x := range d.Embedding {
			v[j] = float32(x)
		}
		vectors[i] = v
	}
	return vectors, nil
}

// httpError rappresenta un errore HTTP con status code.
type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("embedding API HTTP %d: %s", e.Status, e.Body) }

// isTransient dice se l'errore è transitorio (retryabile).
func isTransient(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var he *httpError
	if errors.As(err, &he) {
		if he.Status == http.StatusTooManyRequests || he.Status >= 500 {
			return true
		}
	}
	return false
}
