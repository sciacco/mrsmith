# Template Carbone — Offerta aenad

Template DOCX per generare via [Carbone Cloud](https://carbone.io) i PDF delle
offerte aenad, replicando le stampe legacy Easyfatt "SHELLI CDLAN offerta"
(esempi di riferimento in `artifacts/aenad/`).

## Struttura

| File | Ruolo |
|---|---|
| `generate.mjs` | genera `offerta-template.docx` (libreria [`docx`](https://docx.js.org)) |
| `offerta-template.docx` | il template Carbone (committato; rigenerabile) |
| `assets/` | loghi SHELLI/CDLAN e blocco footer legale, estratti dai PDF originali con `pdfimages -png` |
| `samples/*.json` | 4 payload che replicano i PDF d'esempio (3 da dati DB reali, 1038-rimond ricostruito dal PDF) |
| `render-test.mjs` | carica il template su Carbone e renderizza i sample in `artifacts/claude/aenad-renders/` |

## Workflow di modifica

```sh
pnpm install --ignore-workspace   # solo la prima volta
node generate.mjs                 # rigenera offerta-template.docx
node render-test.mjs              # upload + render dei sample (CARBONE_API_KEY da backend/.env)
```

`render-test.mjs` stampa il **templateId** (hash del contenuto: cambia a ogni
upload). Il backend lo legge a ogni richiesta da `mrsmith.runtime_config`
(DB Anisetta), quindi per promuovere un nuovo template **non serve riavviare**:

```sql
UPDATE mrsmith.runtime_config
SET value = '{"template_id": "<nuovo id>"}'::jsonb
WHERE namespace = 'aenad' AND key = 'carbone_offerta';
```

(seed iniziale in `deploy/migrations/022_aenad_offerta_carbone_config.sql`;
fallback compilato: `DefaultOffertaTemplateID` in
`backend/internal/aenad/carbone.go`, da tenere allineato nei rilasci).

Confrontare visivamente i PDF in `artifacts/claude/aenad-renders/` con gli
originali in `artifacts/aenad/` prima di promuovere un nuovo template ID.

## Contratto dati (payload `data`)

Tutti i valori sono stringhe display già formattate dal backend (it-IT);
stringa vuota quando assenti. Le righe vuote sono spaziatori intenzionali
(così è strutturato il documento in `aenad."TDocRighe"`).

```json
{
  "titolo": "Offerta",
  "numero": "1038",
  "data": "03/06/2026",
  "destinatario": {
    "nome": "RIMOND S.R.L.",
    "indirizzo": "Corso Italia, 17",
    "capCittaProv": "20122  Milano  (MI)",
    "rigaFiscale": "P.Iva 07594680964"
  },
  "righe": [
    {
      "codice": "1414",
      "descrizione": "CANONE NOLEGGIO APPARECCHIATURE",
      "udm": "nr", "qta": "2",
      "prezzo": "€ 120,00", "sconto": "", "importo": "€ 240,00", "iva": "22"
    }
  ],
  "totali": { "imponibile": "€ 240,00", "iva": "€ 52,80", "documento": "€ 292,80" },
  "pagamento": "Bonifico 30 gg F.M.",
  "mostraCondizioni": false
}
```

- `righe[].descrizione` è **HTML** (renderizzata col formatter Carbone `:html`):
  il backend converte i marker Easyfatt di `TDocRighe.Desc` — `**` grassetto,
  `//` corsivo, spesso prefissi non chiusi che stilano il resto della riga —
  in `<b>/<i>`, e i newline in `<br>`.
- `mostraCondizioni` pilota la chiusura: `true` = blocco condizioni di
  fornitura + clausole 1341-1342 + doppia firma + segmento "Acconto"
  (equivalente del report Easyfatt "SHELLI CDLAN offerta"); `false` = solo
  privacy + firma singola ("SHELLI CDLAN offerta noleggio-servizi").
  Le tabelle si auto-rimuovono con il formatter `:drop(table)`.
- `sconto` e `iva` sono passthrough dal DB (`Sconti` è già "30%"/"35+5%",
  la colonna Iva stampata è `CodIva`).

Dettagli completi sulle semantiche legacy: voce "Aenad Offer Print Semantics"
in `docs/IMPLEMENTATION-KNOWLEDGE.md`.

## Deviazioni accettate rispetto a Easyfatt

- Layout a flusso: la tabella righe non si estende a tutta pagina e i box
  totali/pagamento/firme seguono il contenuto invece di essere ancorati al
  fondo pagina.
- Il report "SHELLI preventivo" (layout diverso, il più usato per i
  preventivi interni) non è coperto da questo template.
- Il blocco footer legale è l'immagine originale estratta dal PDF: per
  cambiare i dati societari va sostituita `assets/footer-legale.png`.
