# Manutenzioni - Service Taxonomy, Reference e Target

## Stato

- Ambito: ricostruzione dal modello dati, dal seed, dal backend, dal frontend e dalla documentazione esistente.
- Fonte dati: solo repository, senza query su database live.
- Output: regole operative e semantiche da usare per evolvere `apps/manutenzioni`.

Questo documento chiarisce il reale utilizzo di `service_taxonomy` e dei riferimenti nel modulo Manutenzioni. La regola principale e': `service_taxonomy` non e' un elenco generico di label, ma il catalogo stabile degli oggetti manutenibili; `maintenance_target` rappresenta invece le istanze concrete o libere della singola manutenzione.

## Modello Concettuale

| Layer | Risponde a | Esempio |
| --- | --- | --- |
| `technical_domain` | Chi pianifica, esegue o possiede operativamente l'oggetto? | Cloud, TLC, Applications |
| `service_taxonomy` | Quale oggetto aziendale stabile e' coinvolto? | Cluster k8s, Switching core datacenter |
| `target_type` | Che natura ha l'oggetto o target? | service, platform, asset, tenant |
| `maintenance_service_taxonomy` | Come quella voce entra nella manutenzione specifica? | operata, dipendente, severity, audience |
| `service_dependency` | Quali impatti strutturali sono noti tra oggetti catalogo? | Cluster k8s -> Customer Portal |
| `maintenance_target` | Quale istanza concreta e' coinvolta oggi? | nodo worker X, tenant ACME, coppia C21 A/B |
| `ref_table` / `ref_id` / `external_key` | C'e' un aggancio opzionale a una sorgente esterna? | ordine, asset, circuito, codice servizio |

Il termine "servizio" resta accettabile nella UI per compattezza, ma nel modello va letto come "servizio / oggetto manutenibile". Le voci possono essere applicazioni, piattaforme, prodotti, apparati, impianti, asset o fallback operativi.

## Service Taxonomy

`maintenance.service_taxonomy` e' il catalogo stabile. Ogni voce ha:

- un `code` stabile e univoco;
- un dominio operativo proprietario (`technical_domain_id`);
- una natura (`target_type_id`);
- un'audience di default (`internal`, `external`, `both`, `maintenance`);
- label, descrizione, ordinamento, stato attivo e metadata.

Regole:

- Una voce catalogo deve rappresentare un oggetto ricorrente e utile per ownership, impatti, comunicazioni o reportistica.
- Non creare voci catalogo per ogni istanza puntuale. L'istanza vive in `maintenance_target`.
- I siti fisici come C21/E100 non sono voci `service_taxonomy`; sono `site` o target concreti.
- L'audience `maintenance` significa "dipende dal perimetro concreto"; prima dell'avanzamento operativo deve essere risolta con `maintenance_service_taxonomy.expected_audience`.
- Le voci inattive possono restare visibili se gia' collegate a manutenzioni esistenti.

Esempi corretti:

| Caso | Modellazione |
| --- | --- |
| Manutenzione cluster Kubernetes | `service_taxonomy = Cluster k8s`, eventuali nodi/namespace in `maintenance_target` |
| Upgrade switching core C21 | `service_taxonomy = Switching core datacenter`, istanza "coppia C21 A/B" in `maintenance_target` |
| Tenant firewall cliente | `service_taxonomy = Firewall Multitenant`, target concreto con `target_type = tenant` |
| Data center fisico C21 | `site = C21` o target concreto, non voce catalogo |

## Maintenance Service Taxonomy

`maintenance.maintenance_service_taxonomy` e' la relazione tra una manutenzione e le voci del catalogo. Non e' solo una junction table: contiene la lettura operativa della voce nella manutenzione corrente.

Campi chiave:

- `role`: `operated` quando il team interviene direttamente sulla voce, `dependent` quando la voce subisce impatto a valle.
- `expected_severity`: `none`, `degraded`, `unavailable`.
- `expected_audience`: override per la manutenzione specifica; `NULL` usa il default catalogo.
- `source`: `manual`, `import`, `rule`, `ai_extracted`, `catalog_mapping`, `dependency_graph`.
- `is_primary`: una sola voce primaria quando serve sintesi.

Regole:

- Una voce catalogo puo' comparire una sola volta per manutenzione.
- `operated` vince su `dependent` in caso di conflitto.
- Le voci `operated` devono appartenere al dominio tecnico della manutenzione.
- Le voci `dependent` possono essere cross-dominio.
- I suggerimenti accettati dal grafo usano `source = dependency_graph`.
- Una bozza puo' essere incompleta; prima di approvare o avanzare di stato serve almeno un servizio catalogo o un target manuale.

## Service Dependency

`maintenance.service_dependency` modella il grafo strutturale tra voci del catalogo.

La direzione e' upstream -> downstream:

- upstream: cio' che viene operato o da cui altri dipendono;
- downstream: cio' che puo' subire impatto.

Questa direzione serve al workflow principale: dato un servizio operato, l'app propone i servizi impattati. La lettura inversa resta possibile per analisi, ma non cambia la semantica dell'arco.

Regole:

- Il grafo suggerisce, non applica silenziosamente.
- L'app attuale usa downstream diretti; le dipendenze transitive sono contesto futuro, non selezione automatica.
- `default_severity` e' una proposta iniziale, non una verita' immutabile.
- Gli archi si disattivano con `is_active = false`; non si cancellano fisicamente come azione ordinaria.
- L'ownership operativa di un arco e' il dominio upstream.

## Target e Reference Esterne

`maintenance.maintenance_target` e' il layer delle istanze concrete della singola manutenzione. Serve quando l'oggetto reale e' piu' specifico del catalogo o quando il catalogo non copre ancora il caso.

Campi chiave:

- `target_type_id`: natura del target concreto;
- `service_taxonomy_id`: collegamento opzionale alla voce catalogo;
- `display_name`: nome leggibile dell'istanza;
- `ref_table`, `ref_id`, `external_key`: riferimenti opzionali a sorgenti esterne;
- `source`, `confidence`, `is_primary`, `metadata`: provenienza e qualita' del dato.

Regole:

- `maintenance_target.service_taxonomy_id` e' opzionale.
- Il target puo' restare manuale anche senza voce catalogo.
- `target_type_id` del target non deve per forza coincidere con `service_taxonomy.target_type_id`: una voce catalogo `platform` puo' avere target concreti `tenant`, `asset` o altro.
- `ref_table`, `ref_id` ed `external_key` non sono FK garantite dal modello; sono agganci tecnici opzionali e non vanno esposti come copy utente.
- La UI deve parlare di "Istanza", "Target", "Riferimento" o label business equivalenti, non di `ref_table`.

## Reference Data

Nel backend `ReferenceItem` e `ReferenceData` sono shape API generiche per inviare lookup e configurazioni alla UI. Questo uso di "reference" non va confuso con le istanze concrete.

`ReferenceData` include:

- `sites`
- `technical_domains`
- `maintenance_kinds`
- `customer_scopes`
- `service_taxonomy`
- `reason_classes`
- `impact_effects`
- `quality_flags`
- `target_types`
- `notice_channels`

Regole:

- I selector standard caricano valori attivi.
- Quando una manutenzione usa valori inattivi, il backend li reinserisce nel bundle tramite `maintenance_id`.
- Le pagine di configurazione gestiscono create, update, deactivate e reactivate.
- La disattivazione e' il comportamento normale quando un valore puo' essere gia' referenziato.
- `service_taxonomy` e' l'unico resource standard arricchito con dominio, tipo target e audience.

## Scenari Di Accettazione

### Upgrade switching core C21

- Dominio manutenzione: TLC.
- `service_taxonomy` operata: Switching core datacenter.
- Target concreto: "coppia C21 A/B" con natura `asset`.
- Dipendenti suggeriti: Cluster k8s, Cloudstack, VMware IaaS, Proxmox Privato o altri downstream del grafo.
- Severity proposta: tipicamente `degraded` se ridondato, modificabile dall'operatore.

### Manutenzione cluster k8s

- Dominio manutenzione: Cloud.
- `service_taxonomy` operata: Cluster k8s.
- Target concreti: nodo, namespace, cluster prod o altro dettaglio puntuale.
- Dipendenti: Customer Portal, Keycloak (k8s), Mistra Gateway se presenti nel grafo.
- Audience: derivata dal catalogo o risolta per manutenzioni con audience `maintenance`.

### Tenant firewall cliente

- Catalogo: Firewall Multitenant.
- Target concreto: tenant o cliente specifico.
- `target_type` del target puo' essere `tenant`, anche se la voce catalogo e' una `platform`.
- Non creare una voce catalogo per ogni tenant.

### Ambito non ancora modellato

- Se manca una voce catalogo affidabile, la manutenzione resta creabile come bozza.
- L'utente usa un target manuale con `target_type` e `display_name`.
- Il caso alimenta backlog di promozione a catalogo o grafo, ma non forza classificazioni false.

## Incongruenze Da Tenere Presenti

- `ReferenceItem` e' un nome tecnico sovraccarico: rappresenta lookup/config, non necessariamente un riferimento esterno.
- La tab `Target` e l'Impact Workbench lavorano sullo stesso layer `maintenance_target`; future modifiche UI devono evitare duplicazioni confuse.
- I campi `ref_table`, `ref_id` ed `external_key` oggi sono conservati, ma non esiste ancora una registry tipizzata delle sorgenti esterne ammesse.
- Il catalogo e' intenzionalmente incompleto: i target manuali senza `service_taxonomy_id` devono restare visibili per migliorare il catalogo nel tempo.

## Repo Fit

Comparable screens verificati durante la pianificazione:

- `apps/richieste-fattibilita/src/pages/RequestViewPage.tsx`: detail workspace a tab con header compatto, stati empty/error e copy business.
- `apps/budget/src/views/gruppi/GruppiPage.tsx`: CRUD compatto di configurazione con master/detail, modal e stati di errore.

Archetipo di riferimento: `master_detail_crud`. Questo documento non introduce modifiche UI, API, auth, build o deploy.

## Evidenze Nel Repo

- `docs/manutenzioni_schema.sql`: definizione e seed di `service_taxonomy`, `maintenance_service_taxonomy`, `service_dependency` e `maintenance_target`.
- `backend/internal/manutenzioni/reference.go`: bundle reference e caricamento valori attivi piu' valori gia' selezionati.
- `backend/internal/manutenzioni/children_read.go`: lettura classificazioni, target e arricchimenti `service_taxonomy`.
- `backend/internal/manutenzioni/mutations_impact.go`: validazione ruoli, severity, audience, dominio operato e target.
- `backend/internal/manutenzioni/service_dependencies.go`: API del grafo upstream -> downstream.
- `backend/internal/manutenzioni/cockpit.go`: readiness su impatto e audience.
- `apps/manutenzioni/src/components/ImpactWorkbench.tsx`: separazione UI tra servizi in manutenzione e servizi impattati.
- `apps/manutenzioni/src/pages/MaintenanceCreatePage.tsx`: create flow con selezioni catalogo, target manuali e suggerimenti grafo.
- `apps/manutenzioni/NEXT1.md`: decisioni di modello su catalogo, target, audience e grafo.
