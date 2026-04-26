# Manutenzioni — punti aperti

Documento di lavoro: raccoglie le decisioni rimandate e le incongruenze da
chiarire prima di chiudere V1. Aggiornare appena emergono nuovi punti.

## Quality gate per `approve`

L'action `approve` certifica che il piano è pronto: la sua disponibilità è
gestita dai blocker calcolati in `cockpitBlockersForAction`
(`backend/internal/manutenzioni/cockpit.go:190-204`). Oggi i requisiti per
`approve` sono: `customer_scope`, `window`, `impact`, `audience`. È una
lista di partenza, va consolidata.

Candidati da valutare/aggiungere:

- `customer_scope_id` valorizzato (già vincolato anche dallo schema,
  `docs/manutenzioni_schema.sql:530-533`).
- Almeno una `service_taxonomy` assegnata.
- Almeno una `maintenance_window` con `window_status='planned'` (già nei
  blocker).
- Almeno un `impact_effect` definito (già nei blocker tramite la voce
  `impact`).
- Audience risolta su tutte le `service_taxonomy` (`expected_audience` non
  null) — già nei blocker.
- Almeno un `target` (manuale o derivato): tenerlo nei blocker o lasciarlo
  facoltativo?
- Eventuale `notice` in stato `ready` quando la manutenzione richiede
  comunicazione esterna: kind-by-kind?

Domande aperte:

- La lista varia per `maintenance_kind`? Es. monitoring interno
  (`checkmk`/`tlc`) può non richiedere notice esterne.
- Il gate è solo "presenza" o anche "qualità" del dato (es. notice con
  testo non vuoto, finestra futura non sovrapposta ad altre, ecc.)?
- Cosa serve specificamente per `announce` e `start`? Oggi hanno la stessa
  lista di `approve` — verificare se servono blocker più stretti.

## Audience all'announce

L'announce è obbligatorio anche per audience interna o subset di tecnici:
serve come checkpoint, non come push automatico. Va deciso dove vive il
dato "chi è stato informato".

Opzioni:

- Campo per-manutenzione `announcement_audience`
  (`internal`/`external`/`subset`) sulla riga `maintenance.maintenance`.
- Derivato da `maintenance_service_taxonomy.expected_audience` (già
  esistente: `internal`/`external`/`both`, schema riga 564-565).
- Tabella laterale `maintenance_announcement` con righe per gruppo
  informato (più estendibile, più costoso da modellare).

Da chiarire anche se in V1 l'announce è solo un timestamp + actor o se
deve includere già una scelta esplicita dell'audience.

## Tightening dello schema CHECK

Lo schema (`docs/manutenzioni_schema.sql:510-512`) ammette ancora
`'approved'` nel constraint sul `maintenance.status`. L'app non scrive più
questo valore, ma il CHECK resta permissivo per non rompere eventuali
righe esistenti.

Da fare:

- Verificare se in produzione ci sono righe con `status='approved'`. Se
  sì, decidere come migrare (UPDATE → `scheduled` se hanno una window
  planned, altrimenti → `cancelled`).
- Stringere il CHECK a 7 valori rimuovendo `'approved'`.
- Scrivere la migration SQL idempotente.
- `activeMaintenanceStatusesSQL` (`config.go:559`) già non include più
  `approved`: la stretta del CHECK la rende coerente.

## Action `schedule` rimossa

L'action `schedule` non esiste più nel grafo (rimossa con la riforma del
lifecycle). Da verificare:

- Nessun client esterno (test E2E futuri, integrazioni server-to-server)
  si aspetta che esista.
- Eventuale alias retrocompat che proxy sull'`approve`? Probabilmente non
  serve in V1 (no dati prod).

Se dovesse arrivare una request con `action: schedule`, il backend
risponde 400 `status_transition_not_allowed`. Decidere se vogliamo un
errore più specifico tipo `action_not_supported`.

## Convergenza announce ↔ notice.send_status

In V1 `announced` è un checkpoint manuale, distinto dal
`notice.send_status='sent'` (che è documento-specifico).

In V2, quando arriverà l'auto-send dei notice, valutare se `announced`
debba diventare derivato (= "tutti i notice 'sent'") o restare manuale.
Vantaggi del manuale: l'operatore conferma esplicitamente di aver
informato chi di dovere anche per canali fuori dal sistema (chat,
telefono, email manuale). Vantaggi del derivato: niente doppio click,
allineamento automatico con lo stato dei notice.

Da decidere prima di V2.

## Copy esatte e UX next-step

Per il next-step su `draft`:

- `canApprove`: pulsante "Approva la pianificazione" + summary
  "Approva la pianificazione disponibile."
- `!canApprove` (operator senza approve): summary "Da approvare." e
  nessun pulsante.

Per gli altri step (`announce`, `start`, `complete`) la copy è univoca
lato `canOperate`. Decidere se mostrare un badge informativo anche per
chi non ha `canOperate` (utente solo `app_manutenzioni_access`,
read-only) o lasciare il cockpit silenzioso in quel caso.

Verificare anche che la riga del cockpit "Runway operativa — ${summary}"
funzioni quando `nextAction` è null (ciclo chiuso o stato terminale):
oggi mostra "Ciclo operativo chiuso." — tenerla o variare per stati
specifici (es. "Manutenzione completata.")?

## Action labels nel button strip vs cockpit summary

Il `lifecycleButtons` (`MaintenanceDetailPage.tsx`) e
`cockpitNextActionFor` (`backend/internal/manutenzioni/cockpit.go:161`)
producono label indipendenti. Oggi il backend produce
"Approva la pianificazione" su `draft`, e il frontend produce la stessa
label sul pulsante. Restano sincronizzati a mano.

Decidere se centralizzare (es. il frontend legge sempre il label dal
payload del cockpit) o lasciare la duplicazione (più chiara ma fragile).

## Altre incongruenze

TBD — placeholder per i punti che stanno emergendo dalla revisione del
modello. Aggiungere bullet appena identificati con un titolo descrittivo
e una breve nota di contesto.
