# Grappa DCIM - Layout Step 1

## Stato

- Data: 2026-05-27
- Stato: specifica di transizione, pronta per review prodotto/tecnica.
- Database target: Grappa MySQL `5.6.29`.
- Scope: persistenza e rendering 2D delle mappe a griglia ricavate da `artifacts/mappe`.
- Nota terminologica: il modello descritto qui e il **layout corrente Step 1**. Non va chiamato “legacy” in tabelle, API, tipi o UI. La provenienza dal vecchio applicativo resta solo un'informazione di import/audit.

## Obiettivo

Ripristinare rapidamente la disposizione logica dei rack nelle sale/MMR/isole usando il materiale estratto dall'applicativo precedente, senza bloccare il lavoro piu ambizioso su canvas/3D.

Lo step 1 introduce un modello additivo:

- una tabella di blocchi layout collegata a `datacenter` e, quando risolvibile, a `islets`;
- un campo testuale JSON che conserva la griglia visuale completa;
- un endpoint read-only arricchito con stato live da `positions` e `racks`;
- una vista 2D a griglia nella pagina `Isole e posizioni`.

La source of truth operativa resta invariata:

```text
datacenter -> islets -> positions -> racks
```

Il layout salva solo la disposizione visuale. Occupazione, rack attivo, cliente, potenza e lifecycle si leggono sempre dalle tabelle Grappa esistenti.

## Evidenza analizzata

File analizzati:

- `artifacts/mappe/handoff.md`
- `artifacts/mappe/schema.json`
- `artifacts/mappe/totali.json`
- `artifacts/docs/dcim/canvas1.md`
- `artifacts/docs/dcim/grappa-add.sql`
- schema Grappa: `datacenter`, `islets`, `positions`, `racks`, `plenums`, `dc_rooms_positions`
- implementazione corrente: `backend/internal/grappadcim/facilities_map.go`, `layout.go`, `racks.go`, `apps/grappa-dcim/src/features/facilities/LayoutScene.tsx`

Sintesi `totali.json`:

| Metrica | Valore |
|---|---:|
| Datacenter/mappe | 14 |
| Mappe tipo `dc` | 12 |
| Mappe tipo `mmr` | 2 |
| Blocchi visuali | 37 |
| Celle `position` | 475 |
| Celle `empty` | 229 |
| Celle `plenum` | 14 |
| Celle `label` | 7 |

Mappe presenti:

```text
DC5, DC4, DC6, DC7, DC3, DC2, CED2A1, DC1,
Transito MMR, NAMEX, Galeria Roma, Novara Topix, MMRB, MMRA
```

## Decisioni di prodotto per Step 1

1. **Modello corrente, non temporaneo nella UI**  
   Questo layout e quello operativo finche non sara eventualmente sostituito da un modello diverso. Non va presentato agli utenti come “vecchio” o “legacy”.

2. **Modello a griglia, non CAD**  
   La griglia serve a riconoscere la disposizione della sala e a selezionare posizioni/rack. Non rappresenta misure fisiche certificate.

3. **Persistenza per blocco visuale**  
   Non basta una riga per isola: negli MMR lo stesso `islet_name` puo comparire in piu blocchi visuali. La granularita corretta e il blocco visuale dell'handoff.

4. **Binding live, non copia dati operativi**  
   Il JSON puo contenere `pos`, etichette, plenum e celle vuote. Non deve contenere stato rack calcolato, cliente o occupancy persistita.

5. **Rack move resta esplicito**  
   Cliccare una cella puo aprire inspector/dettaglio; non deve spostare rack via drag o click implicito.

6. **Vista 3D conservata come alternativa**  
   La vista 3D corrente resta disponibile, ma quando esiste un layout a griglia importato la vista 2D deve essere la rappresentazione primaria dello step 1.

## Vincoli emersi dall'handoff

### Blocco visuale

Nell'applicativo precedente:

- DC classici: ogni file isola diventa un blocco visuale.
- MMR: ogni tabella interna alla view diventa un blocco visuale.

Quindi il modello Step 1 deve salvare piu blocchi ordinati per sala/MMR.

### Celle supportate

| Tipo | Significato | Campi richiesti |
|---|---|---|
| `position` | Posizione rack logica nell'isola/fila | `pos` |
| `empty` | Vuoto/corridoio/spaziatore | nessuno |
| `label` | Testo statico, es. `Plenum` | `text` |
| `plenum` | Plenum A/B nel layout | `plenum_type` |

### Duplicati ammessi

Gli MMR hanno blocchi multipli sulla stessa isola logica:

- `MMRB`: `islet_name = side` compare due volte.
- `MMRA`: `islet_name = side` compare due volte; anche `title = Fila D` compare due volte.

Conseguenza: `islet_id` non puo essere unique nella nuova tabella.

### Mapping posizione

La cella JSON usa il numero posizione visuale:

```json
{ "type": "position", "pos": 8 }
```

Il binding operativo deve risolvere:

```text
block.datacenter_id + block.islet_id + cell.pos -> positions.num
```

Non bisogna interpretare `cell.pos` come `positions.id`.

## Compatibilita MySQL 5.6.29

Il DB Grappa attuale e MySQL `5.6.29`; la migration deve evitare feature non disponibili o rischiose in quella versione.

Regole per questa migration:

- non usare il tipo `JSON` introdotto in MySQL 5.7;
- non usare `CHECK`, generated columns, CTE, window functions o expression indexes;
- usare `LONGTEXT` per il payload JSON e validarlo lato applicazione/importer;
- tenere gli indici `utf8mb4` sotto il limite InnoDB classico di 767 byte; per questo `block_key` resta `VARCHAR(160)`;
- non specificare `ROW_FORMAT` o opzioni che dipendono da `innodb_large_prefix`;
- usare solo costrutti supportati da 5.6.29: `CREATE TABLE IF NOT EXISTS`, `DATETIME DEFAULT CURRENT_TIMESTAMP`, `ON UPDATE CURRENT_TIMESTAMP`, FK InnoDB standard.

## Modello dati proposto

Tabelle additive:

- `dcim_layout_blocks`: contiene i blocchi visuali e il JSON a griglia.
- `dcim_layout_block_plenums`: collega le celle `plenum` del blocco alla tabella operativa `plenums`.

I nomi evitano riferimenti a “legacy” perche questo e il modello layout corrente dello Step 1. Se in futuro arrivera il canvas semantico, potra affiancare o migrare questi blocchi senza cambiare la semantica operativa di oggi.

```sql
CREATE TABLE IF NOT EXISTS dcim_layout_blocks (
  id INT NOT NULL AUTO_INCREMENT,
  datacenter_id INT(10) NOT NULL,
  islet_id INT NULL,

  datacenter_name_snapshot VARCHAR(250) NOT NULL,
  datacenter_kind VARCHAR(20) NOT NULL COMMENT 'room, mmr',
  islet_name_snapshot VARCHAR(80) NOT NULL,
  block_key VARCHAR(160) NOT NULL,
  block_title VARCHAR(160) NOT NULL,
  display_order INT NOT NULL DEFAULT 0,
  layout_width VARCHAR(80) NULL,

  schema_version VARCHAR(40) NOT NULL DEFAULT 'layout-grid-v1',
  layout_json LONGTEXT NOT NULL COMMENT 'layout-grid-v1 JSON: grid, source metadata, render hints.',
  source_checksum CHAR(64) NULL,
  active TINYINT(1) NOT NULL DEFAULT 1,

  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_dcim_layout_blocks_datacenter_block (datacenter_id, block_key),
  KEY idx_dcim_layout_blocks_datacenter (datacenter_id, active, display_order),
  KEY idx_dcim_layout_blocks_islet (islet_id),
  KEY idx_dcim_layout_blocks_kind (datacenter_kind),

  CONSTRAINT fk_dcim_layout_blocks_datacenter
    FOREIGN KEY (datacenter_id)
    REFERENCES datacenter (id_datacenter)
    ON DELETE NO ACTION
    ON UPDATE NO ACTION,

  CONSTRAINT fk_dcim_layout_blocks_islet
    FOREIGN KEY (islet_id)
    REFERENCES islets (id)
    ON DELETE SET NULL
    ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dcim_layout_block_plenums (
  id INT NOT NULL AUTO_INCREMENT,
  layout_block_id INT NOT NULL,
  datacenter_id INT(10) NOT NULL,
  plenum_id INT NOT NULL,
  row_index INT NOT NULL,
  col_index INT NOT NULL,
  plenum_type VARCHAR(45) NULL,
  label VARCHAR(80) NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_dcim_layout_block_plenums_cell (layout_block_id, row_index, col_index),
  KEY idx_dcim_layout_block_plenums_block (layout_block_id),
  KEY idx_dcim_layout_block_plenums_plenum (plenum_id, datacenter_id),

  CONSTRAINT fk_dcim_layout_block_plenums_block
    FOREIGN KEY (layout_block_id)
    REFERENCES dcim_layout_blocks (id)
    ON DELETE CASCADE
    ON UPDATE NO ACTION,

  CONSTRAINT fk_dcim_layout_block_plenums_plenum
    FOREIGN KEY (plenum_id, datacenter_id)
    REFERENCES plenums (id, datacenter_id)
    ON DELETE RESTRICT
    ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

Note:

- `layout_json` e `LONGTEXT`, non tipo `JSON`, per compatibilita MySQL 5.6.29 e con la richiesta di un campo testuale.
- La validazione JSON deve stare nel codice/importer.
- `islet_id` e nullable per import parziali o nomi isola non risolti.
- `datacenter_name_snapshot` e `islet_name_snapshot` servono per audit e re-import, non come chiavi operative.
- `datacenter_kind` usa valori di dominio nuovi: `room` per sale/cage, `mmr` per Meet-Me Room. Durante l'import, il tipo sorgente `dc` viene normalizzato a `room`.
- `block_key` distingue blocchi duplicati della stessa isola.
- `dcim_layout_block_plenums` usa FK composita verso `plenums(id, datacenter_id)`, coerente con la PK documentata in `docs/grappa/grappa_plenums.json` e compatibile con MySQL 5.6.
- Le celle `plenum` restano presenti nel JSON per il rendering, ma il collegamento operativo e nella tabella dedicata con FK.

## Contratto JSON `layout-grid-v1`

Ogni riga tabella contiene un blocco visuale completo.

Esempio:

```json
{
  "schemaVersion": "layout-grid-v1",
  "source": {
    "artifact": "artifacts/mappe/totali.json",
    "datacenterName": "DC1",
    "datacenterType": "room",
    "isletName": "1",
    "sourceBlockIndex": 0
  },
  "block": {
    "title": "Isola 1",
    "layoutWidth": "col-12",
    "displayOrder": 0
  },
  "grid": [
    [
      { "type": "position", "pos": 8 },
      { "type": "empty" },
      { "type": "position", "pos": 9 }
    ],
    [
      { "type": "empty" },
      { "type": "label", "text": "Plenum" },
      { "type": "empty" }
    ],
    [
      { "type": "plenum", "plenum_type": "A" },
      { "type": "empty" },
      { "type": "plenum", "plenum_type": "B" }
    ]
  ],
  "renderHints": {
    "layoutWidth": "col-12"
  }
}
```

### Validazioni importer

- `schemaVersion` deve essere `layout-grid-v1`.
- `grid` deve essere array bidimensionale.
- `position` richiede `pos` intero positivo.
- `label` richiede `text` non vuoto.
- `plenum` richiede `plenum_type` in `A`, `B` oppure valore sorgente tollerato e renderizzato come sconosciuto.
- Tipi sconosciuti devono bloccare import, non essere ignorati silenziosamente.
- Le righe possono avere lunghezze diverse in futuro, anche se `totali.json` oggi e regolare nei blocchi analizzati.

## Import da `artifacts/mappe/totali.json`

### Strategia

L'import Step 1 e una **CLI tecnica**, non un endpoint utente e non una migration dati automatica nascosta.

La CLI deve usare lo schema Grappa documentato in `docs/grappa/` come contratto strutturale. I valori effettivi di `datacenter.name`, `islets.name` e `plenums.*` vanno comunque verificati dal dry-run contro il DB reale, perche i file schema documentano colonne/FK ma non l'anagrafica completa.

1. Leggere `datacenters[]`.
2. Risolvere `datacenter.name` contro `datacenter.name` nel DB:
   - match esatto trim;
   - fallback case-insensitive;
   - se ambiguo o assente, saltare la mappa e produrre warning.
3. Normalizzare `datacenter.type`:
   - `dc` -> `room`;
   - `mmr` -> `mmr`.
4. Per ogni `blocks[]`, calcolare:
   - `display_order`: indice nel file;
   - `block_key`: chiave stabile, ad esempio `grid:{display_order}:{slug(islet_name)}:{slug(title)}`;
   - `source_checksum`: SHA-256 canonico del JSON del blocco.
5. Risolvere `islet_id`:
   - `islets.datacenter_id = datacenter_id`;
   - `TRIM(islets.name) = islet_name`;
   - fallback case-insensitive;
   - se assente, salvare `islet_id = NULL` e warning.
6. Validare celle `position`:
   - se `islet_id` risolto, ogni `cell.pos` dovrebbe avere una `positions.num` corrispondente;
   - se mancano posizioni, importare comunque il blocco ma segnare `incomplete` nella risposta endpoint.
7. Risolvere celle `plenum` verso `plenums`:
   - `plenums.datacenter_id = datacenter_id`;
   - preferire match su `plenums.isle = islet_name` e `plenums.type = cell.plenum_type`;
   - fallback case-insensitive/trim su `isle` e `type`;
   - se esiste un match univoco, scrivere `dcim_layout_block_plenums` con `row_index`, `col_index`, `plenum_id`, `datacenter_id`;
   - se il match e assente o ambiguo, importare comunque il blocco ma produrre warning e lasciare la cella non collegata.
8. Upsert su `(datacenter_id, block_key)`.
9. Per ogni upsert di blocco, sostituire le righe plenum collegate per quel blocco in modo transazionale.

### Report import CLI

Il report serve a distinguere due casi operativi diversi:

- **log operativo**: messaggi su stdout/stderr utili a chi lancia la CLI in quel momento;
- **report persistente**: file conservabile, utile per sapere dopo giorni quali mappe sono state importate, quali nomi non hanno fatto match e quali celle sono rimaste incomplete.

Decisione Step 1: niente tabella audit dedicata. La CLI deve produrre output leggibile a console e due report salvabili come artifact operativi: uno JSON per consumo automatico e uno Markdown sintetico per review umana.

Contenuto minimo del report:

- input usato (`artifacts/mappe/totali.json` o path alternativo);
- modalità `dry-run` o `apply`;
- timestamp, versione importer, checksum input;
- datacenter importati;
- datacenter non risolti;
- blocchi inseriti, aggiornati, invariati per checksum;
- blocchi con `islet_id` non risolto;
- celle position senza `positions.num` corrispondente;
- celle plenum collegate a `plenums`;
- celle plenum non collegate per match assente o ambiguo;
- riepilogo finale con exit code consigliato.

Exit code consigliati:

- `0`: import completato senza warning bloccanti;
- `2`: dry-run/apply completato con warning da verificare;
- `1`: errore bloccante, nessuna modifica parziale non transazionale.

## Backend API

Prefisso esistente: `/api/grappa-dcim/v1`.

### Lettura layout a griglia

```text
GET /grappa-dcim/v1/facilities/datacenters/{id}/layout-grid
```

Ruolo: Viewer o Operativo.

Risposta proposta:

```ts
interface LayoutGridResponse {
  datacenter: Datacenter;
  blocks: LayoutGridBlock[];
  positions: Position[];
  racks: RackListItem[];
  incomplete: boolean;
  warnings: string[];
}

interface LayoutGridBlock {
  id: number;
  datacenterId: number;
  isletId?: number;
  isletName: string;
  title: string;
  layoutWidth?: string;
  displayOrder: number;
  schemaVersion: 'layout-grid-v1';
  grid: LayoutGridCell[][];
}

interface LayoutGridCell {
  type: 'position' | 'empty' | 'label' | 'plenum';
  pos?: number;
  text?: string;
  plenumType?: string;

  // arricchimento live, non persistito nel JSON
  positionId?: number;
  positionStatus?: string;
  positionType?: string;
  rackId?: number;
  rackName?: string;
  rackType?: string;
  rackPos?: string;

  // collegamento operativo per celle plenum
  plenumId?: number;
  plenumName?: string;
  plenumStatus?: string;
}
```

Regole backend:

- leggere solo blocchi `active = 1`;
- ordinare per `display_order`, poi `id`;
- arricchire le celle `position` tramite `positions.num` nello stesso `islet_id`;
- arricchire rack usando lo stesso criterio gia usato da `positionsSelectSQL()`;
- arricchire le celle `plenum` tramite `dcim_layout_block_plenums -> plenums` usando `layout_block_id`, `row_index`, `col_index`;
- non mutare `positions.status`, `racks` o `plenums`;
- se una cella non trova `positions.num`, mantenerla renderizzabile con warning;
- se una cella `plenum` non ha binding, mantenerla renderizzabile con warning;
- se non esistono blocchi per la sala, rispondere `blocks: []`, `incomplete: true`.

### Import amministrativo

Per Step 1 l'import e una CLI tecnica. Non e previsto un endpoint utente per importare o modificare i layout a griglia.

Comando indicativo:

```text
grappa-dcim-layout-import \
  --source artifacts/mappe/totali.json \
  --dry-run \
  --report artifacts/mappe/import-report.json
```

## Frontend

Pagina interessata: `apps/grappa-dcim/src/features/facilities/FacilitiesPages.tsx`, route `Isole e posizioni`.

### UX Step 1

Aggiungere uno switch di vista nella pagina layout:

- `Vista 2D`
- `Vista 3D`

Comportamento:

- se `layout-grid.blocks.length > 0`, default su `Vista 2D`;
- se non ci sono blocchi a griglia, mostrare l'attuale fallback/3D basata su `islets` e `positions`;
- Viewer puo selezionare e aprire dettagli, ma non vede azioni di modifica;
- Operativo mantiene le azioni esistenti su isole/posizioni/rack, ma nessuna modifica diretta al JSON in Step 1.

### Renderer 2D a griglia

Componente proposto:

```text
apps/grappa-dcim/src/features/facilities/LayoutGrid.tsx
```

Responsabilita:

- renderizzare blocchi in ordine;
- usare `layoutWidth` solo come hint di larghezza, non come classe Bootstrap letterale;
- mostrare celle `empty`, `label`, `plenum`, `position`;
- colorare `position` con lo stesso linguaggio di stato gia presente:
  - libera;
  - occupata;
  - riservata;
  - sconosciuta/incompleta;
- click su cella position:
  - seleziona `positionId` se presente;
  - aggiorna inspector esistente;
  - permette `Apri rack` se `rackId` presente.

### Layout visuale

- Ogni blocco ha titolo (`Isola 1`, `Fila A`, `Sezione 4`, ecc.).
- Le celle position devono mostrare almeno:
  - numero posizione;
  - rack name se occupata;
  - badge compatto per alta/bassa se half rack (`A`/`B`).
- `plenum` renderizzato come elemento tecnico collegato a `plenums` quando il binding e risolto; se il binding manca, la cella resta visibile come incompleta.
- `empty` renderizzato come spazio/corridoio non cliccabile.
- Su viewport stretto, la griglia puo scrollare orizzontalmente dentro il pannello.

### Copy

Copy ammessa:

- `Vista 2D`
- `Vista 3D`
- `Mappa non configurata`
- `Posizione non trovata nei dati Grappa`
- `Plenum A`, `Plenum B`
- `posizione alta`, `posizione bassa`

Copy da evitare:

- `legacy`, `JSON`, `record`, `Bootstrap`, `VisualBlock`, `server-side`, `drag rack` nella UI utente.

## Sicurezza e ruoli

- Lettura: stessi ruoli viewer della mini-app Grappa DCIM.
- Scrittura/import: solo processo tecnico o ruolo Operativo/admin, non Viewer.
- Il JSON non deve contenere HTML da renderizzare. `label.text` va trattato come testo escaped.
- Nessun link diretto non autenticato ad artifact o immagini.

## Coesistenza con il canvas futuro

Step 1 e compatibile con il modello canvas di `artifacts/docs/dcim/grappa-add.sql`, ma la terminologia utente deve trattare Step 1 come layout corrente.

| Layout Step 1 | Canvas futuro |
|---|---|
| Blocchi tabellari importati da mappe sorgente | Oggetti semantici liberi su canvas |
| Coordinate implicite da matrice righe/colonne | Coordinate normalizzate `x/y/width/height` |
| Read-only utente | Editor bozza/pubblica |
| Campo JSON per blocco | Versioni ed elementi normalizzati |
| Ottimo per parity veloce | Ottimo per ricalco immagini e layout nuovi |

Quando il canvas sara pronto, i layout Step 1 potranno essere:

- usati come fallback;
- convertiti in elementi canvas iniziali;
- mantenuti come storico/audit della migrazione.

## Rollout consigliato

### Fase 1 - DB e import

- Creare tabelle `dcim_layout_blocks` e `dcim_layout_block_plenums` con SQL compatibile MySQL 5.6.29.
- Implementare CLI importer da `artifacts/mappe/totali.json`.
- Eseguire dry-run con report file.
- Correggere mapping nomi datacenter/isole/plenum dove necessario.

### Fase 2 - API read-only

- Aggiungere tipi Go `LayoutGrid*`.
- Endpoint `GET /facilities/datacenters/{id}/layout-grid`.
- Arricchimento live con `positions`/`racks`.
- Warning per blocchi incompleti o posizioni non risolte.

### Fase 3 - Frontend read-only operativo

- Aggiungere query React Query.
- Implementare `LayoutGrid`.
- Collegare selezione cella all'inspector esistente.
- Default su 2D quando disponibile.

### Fase 4 - Rifiniture operative

- Badge incompleto per celle non risolte.
- Azione `Apri rack` dalla cella occupata.
- Link/apertura dettaglio plenum dalle celle collegate.
- Documentare import report e anomalie residue.

## Verifica manuale

Checklist minima:

- DC1 mostra plenum A/B e label `Plenum` sotto le isole.
- Le celle plenum risolte espongono il collegamento a `plenums`.
- MMRB mostra due blocchi distinti per `side` senza sovrascriverli.
- MMRA mostra due blocchi `Fila D` senza collisione di chiave.
- Una posizione occupata mostra il rack live dalla tabella `racks`.
- Una posizione libera resta libera anche se nel JSON e presente come cella position.
- Una cella con `pos` senza `positions.num` resta visibile ma segnalata incompleta.
- Viewer non vede azioni di import/edit layout.
- Operativo continua a spostare rack solo tramite flusso esplicito esistente.
- Refresh/deep-link della mini-app continua a funzionare sotto `/apps/grappa-dcim/`.

## Test automatici

Non aggiungere test senza approvazione esplicita.

Se approvati, i test piu utili sarebbero:

- backend importer: mapping `cell.pos -> positions.num` e gestione duplicati MMR;
- backend endpoint: arricchimento live, binding plenum e warning incomplete;
- frontend: render celle `position`, `empty`, `label`, `plenum`, selezione posizione e apertura plenum;
- Playwright: `Vista 2D` popolata, selezione cella, apertura rack.

## Decisioni chiuse e punti aperti

Decisioni chiuse:

1. Schema DB: usare come riferimento strutturale i file in `docs/grappa/`, in particolare `grappa_datacenter.json`, `grappa_islets.json`, `grappa_positions.json`, `grappa_racks.json`, `grappa_plenums.json`.
2. Import: CLI tecnica, non endpoint utente.
3. Report CLI: produrre entrambi i formati, JSON per consumo automatico e Markdown sintetico per review umana.
4. Plenum: collegamento operativo alla tabella `plenums` tramite `dcim_layout_block_plenums`.
5. MySQL: migration compatibile con Grappa MySQL `5.6.29`.

Punti aperti:

- Nessuno al momento.
