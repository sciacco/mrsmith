package rdf

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sciacco/mrsmith/internal/platform/logging"
	"github.com/sciacco/mrsmith/internal/platform/openrouter"
)

const (
	analysisTextModel = "google/gemini-2.5-flash-lite-preview-09-2025"
	analysisJSONModel = "google/gemini-2.5-flash-lite-preview-06-17"
)

var scoreBudgetLabels = []string{
	"Non valutato",
	"Pessima",
	"Fuori budget",
	"Nella norma",
	"Ottima",
	"Eccezionale",
}

func (h *Handler) analyzeText(ctx context.Context, richiestaID int, full RichiestaFull) (string, error) {
	content, usage, err := h.runAICompletion(ctx, richiestaID, analysisTextModel, systemPromptText, false, buildAnalysisPayload(full))
	if err != nil {
		return "", err
	}
	logging.FromContext(ctx).Info(
		"rdf ai completion succeeded",
		"component", "rdf",
		"request_id", logging.RequestID(ctx),
		"richiesta_id", richiestaID,
		"model", analysisTextModel,
		"prompt_tokens", usage.PromptTokens,
		"completion_tokens", usage.CompletionTokens,
		"total_tokens", usage.TotalTokens,
	)
	return strings.TrimSpace(content), nil
}

func (h *Handler) analyzeJSON(ctx context.Context, richiestaID int, full RichiestaFull) (analysisJSON, error) {
	content, usage, err := h.runAICompletion(ctx, richiestaID, analysisJSONModel, systemPromptJSON, true, buildAnalysisPayload(full))
	if err != nil {
		return analysisJSON{}, err
	}

	var parsed analysisJSON
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		logging.FromContext(ctx).Error(
			"rdf ai json decode failed",
			"component", "rdf",
			"request_id", logging.RequestID(ctx),
			"richiesta_id", richiestaID,
			"model", analysisJSONModel,
			"error", err,
		)
		return analysisJSON{}, fmt.Errorf("decode ai json: %w", err)
	}

	logging.FromContext(ctx).Info(
		"rdf ai completion succeeded",
		"component", "rdf",
		"request_id", logging.RequestID(ctx),
		"richiesta_id", richiestaID,
		"model", analysisJSONModel,
		"prompt_tokens", usage.PromptTokens,
		"completion_tokens", usage.CompletionTokens,
		"total_tokens", usage.TotalTokens,
	)
	return parsed, nil
}

func (h *Handler) runAICompletion(ctx context.Context, richiestaID int, model, systemPrompt string, jsonMode bool, full RichiestaFull) (string, openrouter.Usage, error) {
	if h.ai == nil {
		return "", openrouter.Usage{}, errAIUnavailable
	}

	payload, err := json.MarshalIndent(buildAnalysisRecords(full), "", "  ")
	if err != nil {
		return "", openrouter.Usage{}, fmt.Errorf("marshal analysis payload: %w", err)
	}

	request := openrouter.ChatRequest{
		Model:       model,
		Temperature: 0,
		MaxTokens:   4096,
		Messages: []openrouter.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: string(payload)},
		},
	}
	if jsonMode {
		request.ResponseFormat = &openrouter.ResponseFormat{Type: "json_object"}
	}

	start := time.Now()
	response, err := h.ai.Chat(ctx, request)
	latencyMs := time.Since(start).Milliseconds()
	if err != nil {
		logging.FromContext(ctx).Error(
			"rdf ai completion failed",
			"component", "rdf",
			"request_id", logging.RequestID(ctx),
			"richiesta_id", richiestaID,
			"model", model,
			"latency_ms", latencyMs,
			"error", err,
		)
		return "", openrouter.Usage{}, err
	}

	logging.FromContext(ctx).Info(
		"rdf ai completion timing",
		"component", "rdf",
		"request_id", logging.RequestID(ctx),
		"richiesta_id", richiestaID,
		"model", response.Model,
		"latency_ms", latencyMs,
	)
	return response.Content, response.Usage, nil
}

func buildAnalysisPayload(full RichiestaFull) RichiestaFull {
	return full
}

func buildAnalysisRecords(full RichiestaFull) []map[string]any {
	rows := make([]map[string]any, 0, len(full.Fattibilita))
	for _, item := range full.Fattibilita {
		row := map[string]any{
			"id":                      full.ID,
			"deal_id":                 full.DealID,
			"data_richiesta":          full.DataRichiesta,
			"descrizione":             full.Descrizione,
			"indirizzo":               full.Indirizzo,
			"stato":                   full.Stato,
			"annotazioni_richiedente": full.AnnotazioniRichiedente,
			"annotazioni_carrier":     full.AnnotazioniCarrier,
			"created_by":              full.CreatedBy,
			"created_at":              full.CreatedAt,
			"updated_at":              full.UpdatedAt,
			"fornitori_preferiti":     full.FornitoriPreferiti,
			"codice_deal":             full.CodiceDeal,
			"tecnologia":              item.TecnologiaNome,
			"fornitore":               item.FornitoreNome,
			"profilo_fornitore":       item.ProfiloFornitore,
			"nrc":                     item.NRC,
			"mrc":                     item.MRC,
			"durata_mesi":             item.DurataMesi,
			"aderenza_budget":         item.AderenzaBudget,
			"copertura":               boolToInt(item.Copertura),
			"if_ff":                   item.ID,
			"richiesta_id":            item.RichiestaID,
			"fornitore_id":            item.FornitoreID,
			"data_richiesta_ff":       item.DataRichiesta,
			"tecnologia_id":           item.TecnologiaID,
			"descrizione_ff":          item.Descrizione,
			"contatto_fornitore":      item.ContattoFornitore,
			"riferimento_fornitore":   item.RiferimentoFornitore,
			"stato_ff":                item.Stato,
			"annotazioni":             item.Annotazioni,
			"esito_ricevuto_il":       item.EsitoRicevutoIl,
			"da_ordinare":             item.DaOrdinare,
			"giorni_rilascio":         item.GiorniRilascio,
		}
		rows = append(rows, row)
	}
	return rows
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

var systemPromptText = strings.TrimSpace(`
### System Prompt - Analisi di Fattibilita per Circuiti di Accesso

Il tuo compito e analizzare un insieme di dati JSON generati da una query SQL sul database delle richieste di fattibilita. Ogni voce rappresenta una richiesta inoltrata a uno o piu fornitori per l'attivazione di un circuito di accesso.

Obiettivo: valutare lo stato e l'idoneita delle risposte ricevute, tenendo conto delle specifiche fornite nella richiesta e dei vincoli indicati.

---

### 1. Requisiti Generali

- Analizza le risposte dei fornitori per ogni richiesta.
- Fornisci in cima una sezione "Azioni raccomandate" con suggerimenti operativi.
- Non includere mai direttamente i valori di 'nrc' o 'mrc' nelle risposte: sono confidenziali e devono essere usati solo per valutazioni.
- Utilizza un tono tecnico e sintetico, ma comprensibile anche in ambito commerciale.

---

### 2. Interpretazione dei Campi Chiave

#### Stato della richiesta ('stato_ff')
- Valori possibili: 'bozza', 'inviata', 'sollecitata', 'completata', 'annullata'
- 'bozza': non inviata, ignorare per ora
- 'inviata' / 'sollecitata': in attesa di risposta
- 'completata': risposta ricevuta, valutare

#### Esito ricevuto ('esito_ricevuto_il')
- 'null' o vuoto: non e stata ricevuta risposta dal fornitore
- Valorizzato: risposta disponibile

#### Copertura ('copertura')
- '1' = copertura presente
- '0' = assente o non indicata

#### Aderenza al budget ('aderenza_budget')
Valori ammessi e relativi giudizi:
0 = Non valutato
1 = Pessima
2 = Fuori budget
3 = Nella norma
4 = Ottima
5 = Eccezionale

#### Durata del contratto ('durata_mesi')
- Per tecnologie FTTH / FTTC / VDSL: massimo consigliato = 3 mesi
- Altre tecnologie: accettabili fino a 24 mesi
- Oltre 24 mesi: evidenziare come criticita

#### Tempi di rilascio ('giorni_rilascio')
- '0': valore non indicato o entro SLA standard
- Valori positivi: specificano i giorni di attesa
- Evidenzia tempi > 60 giorni come ritardi rilevanti

#### Preferenze del richiedente ('fornitori_preferiti')
- Campo opzionale che elenca uno o piu 'fornitore_id'
- Se presenti, privilegia nelle valutazioni i fornitori inclusi
- Se non presenti non includere nell'output la riga relativa

---

### 3. Formato dell'Output

#### Azioni raccomandate e criticita (inizio)
- Elenco puntato delle attivita da compiere (es. sollecitare fornitore X, privilegiare offerta Y, escludere proposta Z per assenza copertura).
- Evidenziare eventuali criticita riscontrate

#### Valutazioni per ogni risposta ricevuta (a seguire)
Per ogni combinazione tecnologia-fornitore:
### <Nome Fornitore> - <Tecnologia>
- Stato: ...
- Copertura: ...
- Aderenza al budget: ...
- Durata proposta: ...
- Tempi di attivazione: ...
- Preferito dal richiedente: SI
- Criticita: (solo se presenti)

Evita ripetizioni inutili, riassumi i punti salienti.

---

### 4. Restrizioni
- Non citare mai esplicitamente 'nrc' o 'mrc'.
- Ignora i record in stato 'bozza', salvo che servano per segnalare mancanza di invio.
- L'output deve essere autoesplicativo e pronto per un lettore tecnico-commerciale.

---

### 5. Esempio di output sintetico

## Azioni raccomandate
- Sollecitare FASTWEB per FIBRA GPON BIZ (nessun esito ricevuto)
- Valutare positivamente FIBERCOP per FIBRA DEDICATA, se accettabile la durata
- Escludere proposte con copertura assente o non verificata

## Valutazioni

* FIBERCOP / TIM - FIBRA DEDICATA
- Stato: Completata
- Copertura: Assente
- Aderenza al budget: Nella norma
- Durata proposta: 30 mesi -> attenzione, superiore al limite consigliato
- Tempi di attivazione: non indicati (entro SLA)
- Preferito dal richiedente: SI
- Criticita: Nessuna copertura

* FASTWEB - FTTC / VDSL
- Stato: Bozza
- Nessun esito inviato, sollecitare
- Preferito: SI

---

### Fine Prompt
`)

var systemPromptJSON = strings.TrimSpace(`
System Prompt - Analisi di Fattibilita per Circuiti di Accesso (Output JSON)

Il tuo compito e analizzare un insieme di dati JSON generati da una query SQL sul database delle richieste di fattibilita. Ogni voce rappresenta una richiesta inoltrata a uno o piu fornitori per l'attivazione di un circuito di accesso.

Obiettivo: valutare lo stato e l'idoneita delle risposte ricevute, tenendo conto delle specifiche fornite nella richiesta e dei vincoli indicati.

1. Requisiti Generali
- Analizza le risposte dei fornitori per ogni richiesta.
- Produce un oggetto JSON conforme alle specifiche del paragrafo 3.
- Valuta e ordina le azioni raccomandate per priorita operativa.
- Non includere mai direttamente i valori di nrc o mrc nelle risposte: sono confidenziali e devono essere usati solo per valutazioni interne.
- Utilizza un tono tecnico e sintetico, ma comprensibile anche in ambito commerciale.

2. Interpretazione dei Campi Chiave

Stato della richiesta ("stato_ff")
- Valori possibili: bozza, inviata, sollecitata, completata, annullata
- bozza: non inviata, ignorare per ora
- inviata / sollecitata: in attesa di risposta
- completata: risposta ricevuta, valutare

Esito ricevuto ("esito_ricevuto_il")
- null o vuoto: non e stata ricevuta risposta dal fornitore
- Valorizzato: risposta disponibile

Copertura ("copertura")
- 1 = copertura presente
- 0 = assente o non indicata

Aderenza al budget ("aderenza_budget")
0 = Non valutato
1 = Pessima
2 = Fuori budget
3 = Nella norma
4 = Ottima
5 = Eccezionale

Durata del contratto ("durata_mesi")
- Per tecnologie FTTH / FTTC / VDSL: massimo consigliato = 3 mesi
- Altre tecnologie: accettabili fino a 24 mesi
- Oltre 24 mesi: evidenziare come criticita

Tempi di rilascio ("giorni_rilascio")
- 0: valore non indicato o entro SLA standard
- Valori positivi: specificano i giorni di attesa
- Evidenzia tempi > 60 giorni come ritardi rilevanti

Preferenze del richiedente ("fornitori_preferiti")
- Campo opzionale che elenca uno o piu fornitore_id
- Se presenti, privilegia nelle valutazioni i fornitori inclusi
- Se non presenti non includere nell'output il campo preferito

3. Formato dell'Output (JSON)
Il risultato deve essere un singolo oggetto JSON con le seguenti chiavi di primo livello:
{
  "azioni_raccomandate": [],
  "valutazioni": []
}

3.1 azioni_raccomandate
Ogni elemento descrive un'attivita da intraprendere con i campi:
- azione: string, obbligatorio
- fornitore: string, obbligatorio
- tecnologia: string, opzionale
- motivo: string, obbligatorio

3.2 valutazioni
Un oggetto per ogni combinazione fornitore-tecnologia con i campi:
- fornitore: string, obbligatorio
- tecnologia: string, obbligatorio
- stato: string, obbligatorio
- copertura: "Presente", "Assente", "Non indicata", opzionale
- aderenza_budget: "Pessima", "Fuori budget", "Nella norma", "Ottima", "Eccezionale", opzionale
- durata_mesi: integer, opzionale
- giorni_rilascio: integer oppure null, opzionale
- preferito: boolean, opzionale
- criticita: string, opzionale

Omettere il campo se l'informazione non e disponibile o irrilevante.

Ordinamento:
- azioni_raccomandate: ordine di priorita
- valutazioni: ordine alfabetico per fornitore, poi tecnologia

4. Restrizioni
- Non citare mai esplicitamente i valori di nrc o mrc.
- Ignora i record in stato "bozza" salvo che servano per segnalare la mancanza di invio.
- L'oggetto JSON prodotto deve essere valido secondo lo standard ECMAScript 2020.

5. Note Operative
- Evita ripetizioni inutili; riassumi sempre i punti salienti.
- Se un fornitore non ha fornito esito entro i tempi, inserisci nelle azioni_raccomandate un'azione "sollecitare".
- Evidenzia nei campi criticita tutte le condizioni che violano i limiti indicati nel paragrafo 2.
- Se fornitori_preferiti e vuoto o assente, ometti il campo preferito.

Fine Prompt
`)
