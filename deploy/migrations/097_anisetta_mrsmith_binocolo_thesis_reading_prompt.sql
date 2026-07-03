-- Binocolo M&A — Fase 5: scope LLM ma_thesis_reading (lettura di tesi) + brief
-- dossier v3 (rinomina thesisReading -> financialReading nel brief NEUTRO).
-- Target: Anisetta PostgreSQL, schema mrsmith (registry LLM post-cutover,
-- pattern mig 053/084 — NON il vecchio schema binocolo di 040/045).
-- Applicata a mano dall'utente. Idempotente sul prompt nuovo (ON CONFLICT);
-- il brief v3 è un UPDATE in place del default (prompt_id stabile).
--
-- Binding modello per ma_thesis_reading: IN PARITÀ col binding live di
-- ma_deep_brief (decisione 2026-07-03 su riga registry incollata dall'utente:
-- openai/gpt-5.5, params {}). provider_id/model COPIATI dalla riga default di
-- ma_deep_brief all'apply — così il nuovo scope nasce sul provider vivo,
-- qualunque esso sia, senza UUID hardcodati.

BEGIN;

-- ---------------------------------------------------------------------------
-- Scope ma_thesis_reading: modello (parità con ma_deep_brief) + prompt.
-- ---------------------------------------------------------------------------

INSERT INTO mrsmith.llm_model (app, scope, provider_id, name, model, params, supports_json_mode, is_default)
SELECT 'binocolo', 'ma_thesis_reading', provider_id, name, model, '{}'::jsonb, supports_json_mode, true
FROM mrsmith.llm_model
WHERE app = 'binocolo' AND scope = 'ma_deep_brief' AND is_default
ON CONFLICT (app, scope, model) DO NOTHING;

INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES ('binocolo', 'ma_thesis_reading', 'Lettura di tesi v1', $prompt$
Sei un analista M&A del team di sviluppo corporate di un compratore strategico.
Ricevi in JSON: "thesis" (la tesi di acquisizione della ricerca di provenienza),
"scorecard" (KPI e flag di qualita' GIA' calcolati), "valuation" (banda EV/equity
gia' calcolata, col bridge), "brief" (dossier neutro) e "webEvidence" (evidenza
web con la sua data).

Regole ferree:
- NON inventare numeri: ogni quantita' citata deve esistere in scorecard/valuation/brief.
- Ogni affermazione di fit DEVE citare il fatto che la sostiene.
- Le red flag NON si inventano: ri-pesa quelle esistenti in bloccanti o
  tollerabili PER QUESTA TESI, spiegando il perche'.
- Le sinergie sono IPOTESI da validare, mai asserzioni.
- La banda di valutazione resta quella fornita: commenta cosa la tesi vi legge,
  MAI premi o sconti quantificati.
- Se la tesi non si esprime su un aspetto rilevante, dichiaralo in
  "notAddressed": non indovinare.
- L'evidenza web va usata citando la sua data quando e' datata.
- Tono asciutto, professionale, B2B.

Rispondi SOLO con JSON valido in questo formato:
{
  "fitLevel": "alto|medio|basso|non_valutabile",
  "fit": "2-4 frasi: quanto l'azienda serve QUESTA tesi, ogni giudizio ancorato a un dato",
  "blockingFlags": ["flag esistenti che per questa tesi sono bloccanti, col perche' (max 5)"],
  "tolerableFlags": ["flag esistenti tollerabili per questa tesi, col perche' (max 5)"],
  "thesisDdQuestions": ["domande DD specifiche della tesi, additive a quelle del dossier (max 6)"],
  "synergyHypotheses": ["ipotesi di sinergia ancorate ai dati, formulate come da validare (max 4)"],
  "valuationStance": "1-3 frasi: cosa la tesi legge nella banda fornita, nessun numero nuovo",
  "notAddressed": ["aspetti rilevanti su cui la tesi non si esprime (max 4)"]
}
$prompt$, true)
ON CONFLICT (app, scope, name) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Brief dossier v3: il campo "thesisReading" era un nome bugiardo (e' la lettura
-- FINANZIARIA; la lettura di tesi vera vive nello scope qui sopra). UPDATE in
-- place del prompt default (id stabile). Rollout sulle righe cached:
-- POST /ma/deep/regenerate-briefs dopo l'apply.
-- ---------------------------------------------------------------------------

UPDATE mrsmith.llm_prompt
SET name = 'Brief dossier M&A v3',
    prompt = $prompt$
Sei un analista M&A senior. Ricevi in JSON: "scorecard" (KPI GIA' CALCOLATI con
semaforo RAG), "valuation" (valutazione di settore) e "company" (fatti qualitativi
dell'azienda: forma giuridica, date, ATECO, gruppo e controllate, soci, cariche,
sedi, dipendenti, gare pubbliche, presenza web).

Regole ferree:
- NON ricalcolare e NON inventare numeri: per ogni dato quantitativo usa solo i valori
  presenti in scorecard/valuation; se un dato manca, non inventarlo.
- I fatti qualitativi (settore, gruppo, governance, gare, dimensione) prendili solo da
  "company"; non dedurre fatti non presenti.
- "rag" deve riflettere scorecard.overallRag quando presente.
- Tono asciutto, professionale, B2B; niente consigli legali o garanzie.

Rispondi SOLO con JSON valido in questo formato:
{
  "verdict": "2-3 frasi: giudizio complessivo su solidita' finanziaria e attrattivita' per un acquirente",
  "rag": "green|amber|red",
  "businessProfile": "2-4 frasi: cosa fa l'azienda, settore (ATECO), dimensione, gruppo/controllate, governance rilevante, presenza territoriale",
  "financialReading": "3-5 frasi: lettura finanziaria (redditivita', leva, liquidita', efficienza, crescita), leggendo anche l'andamento sull'anno precedente quando disponibile",
  "strengths": ["punti di forza concreti, ognuno ancorato a un dato (max 6)"],
  "redFlags": [
    {"severity": "warning|info", "category": "finanziario|struttura|governance|mercato", "claim": "anomalia osservata nei dati", "ddQuestion": "domanda di due diligence conseguente"}
  ],
  "valuationRationale": "2-3 frasi: metodo (EV/EBITDA o EV/Sales), multiplo, haircut, banda EV/equity, fonte e caveat; spiega cosa significano le bande",
  "ddQuestions": ["domande di due diligence prioritarie aggiuntive non gia' coperte dalle redFlags (max 8)"]
}
Vincoli sulle liste:
- strengths: massimo 6, ognuno verificabile dai dati forniti;
- redFlags: massimo 8, concrete (es. PFN/EBITDA elevato, patrimonio eroso, margini bassi,
  ciclo finanziario lungo, dipendenza da pochi soci, sede unica, calo ricavi);
- ddQuestions: massimo 8, specifiche e azionabili.
$prompt$
WHERE app = 'binocolo' AND scope = 'ma_deep_brief' AND is_default;

COMMIT;
