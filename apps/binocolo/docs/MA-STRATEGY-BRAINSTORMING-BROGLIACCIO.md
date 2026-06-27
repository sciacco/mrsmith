# M&A Strategy Brainstorming - Brogliaccio

Documento di lavoro per tenere memoria della sessione di brainstorming sull'uso degli LLM nella seconda parte della strategia M&A di Binocolo.

## Contesto

- Gli ultimi commit hanno spostato il focus sull'ottimizzazione della trasformazione dalla richiesta utente ai parametri di ricerca.
- E' emerso che non tutti i dati acquisiti o dedotti dalla richiesta utente vengono poi usati nei filtri, nella classificazione o nello scoring.
- La pipeline corrente V2 separa: estrazione intent LLM, resolver/canonicalizer deterministici, stima dry-run OpenAPI.it, esecuzione target, scoring deterministico, deep dive successivo.

## Stato discusso

- `Stima ricerca` accoda un job asincrono di estimate e puo' generare molte probe OpenAPI.it `dryRun=1`.
- Le probe sono cache-backed tramite `company_search_cache` con TTL 30 giorni e lock per cache key; non sono accodate singolarmente.
- La selezione della card di stima e' solo stato UI; l'esecuzione reale parte con `Conferma e cerca target`.
- `Conferma e cerca target` accoda un job `execute`, valida stima/costo/superficie, recupera aziende con enrichment `advanced`, deduplica e poi applica scoring deterministico.
- Lo scoring base non usa LLM e non chiama servizi esterni.

## Punti critici aperti

- Capire quali campi della richiesta utente influenzano retrieval, stima, scoring, post-filtri o solo UI/audit.
- Rendere esplicita la differenza tra stima lorda OpenAPI.it e criteri verificabili solo dopo il fetch pagato.
- Chiarire all'utente quando uno score e' alto ma con bassa copertura dati.
- Valutare l'effetto dei segnali percentile, che sono stabili a parita' di popolazione ma cambiano se cambia il campione della run.
- Valutare quanto la strategia proposta dall'LLM, pur editabile, influenzi indirettamente lo scoring deterministico.

## Keyword match e web evidence

- Il keyword match corrente usa solo `AtecoDescription + CompanyName` contro settore/keyword positive.
- Brave Search e' gia' presente come esperimento separato: ricerca `site:dominio keyword`, filtro dominio, ranking opzionale LLM degli snippet.
- Possibile direzione: introdurre un segnale separato `web_keyword_match`, cache-backed, applicato solo a top N o su richiesta, con evidenze citabili da snippet.
- Nodo da risolvere: estrazione affidabile del dominio ufficiale dal payload OpenAPI.it.
- OpenAPI.it non fornisce un dominio ufficiale affidabile per i target; prima della keyword search serve quindi una fase separata di domain resolution, con soglia di confidenza e fallback esplicito a "non disponibile".
- Le funzioni sperimentali non devono sporcare la pagina utenti `WebSearchPage`; il laboratorio va nella pagina `test`, con tab separati per domain resolution, keyword evidence e funzioni future.
- Implementato laboratorio nella pagina `test`: tab per Company search, Domain resolver, Keyword evidence e Manutenzione. Aggiunto endpoint sperimentale `POST /binocolo/v1/test/domain-resolution` che usa Brave per trovare domini candidati e li classifica con euristiche deterministiche.
- Nuovo nodo: keyword enrichment. Non basta riusare letteralmente la richiesta utente ("MSP"); serve derivare un set strutturato di termini positivi, sinonimi/espansioni e termini negativi/esclusioni, mantenendo sempre traccia dell'origine dalla richiesta.
- Ipotesi da validare: introdurre un LLM piccolo per keyword enrichment nella pipeline web evidence, con output JSON vincolato e separato dalla strategia principale. Il suo ruolo sarebbe espandere/normalizzare il linguaggio utente, non classificare aziende.
- Primo test del domain resolver su Coherency SRL: Brave trova pagine molto pertinenti sull'azienda, ma quasi tutte sono directory/banche dati aziendali. Il ranking attuale confonde "evidenza anagrafica sulla societa'" con "dominio ufficiale". Servono classificazione della sorgente, blacklist/graylist piu' ampia per directory italiane e una soglia che promuova un candidato solo se l'host e' compatibile con il brand o ci sono segnali espliciti di sito proprietario.
- Secondo test su Coherency SRL con query senza P.IVA e con "contattaci": emerge `coherency.eu`, host compatibile e snippet con indirizzo/contatto. Questo conferma che gli identificativi fiscali sono utili per validare, ma peggiorano la discovery perche' attirano directory; la discovery dovrebbe partire da brand + localita' + intenti da sito ufficiale/contatti.
- Test keyword evidence su `site:coherency.eu cloud`: risultato positivo sulla homepage con snippet coerente su hybrid/cloud native architecture. Questo valida la catena domain resolution -> site-restricted keyword evidence. La keyword `cloud` va trattata come termine adiacente/contesto, non come prova sufficiente di match per categorie piu' specifiche come MSP.
- Direzione operativa proposta: sostituire il keyword match monolitico con pipeline a quattro passi: keyword intent enrichment LLM, domain resolution ufficiale, site-restricted keyword evidence, scoring aggregato con pesi diversi per termini core/adiacenti/negativi. La web evidence dovrebbe produrre un segnale separato e citabile, non sovrascrivere il punteggio deterministico base.
- Chiarimento importante: Coherency e' emersa da una ricerca che non citava MSP, ma criteri geografici/finanziari/organizzativi e ATECO IT specifici. In questi casi l'arricchimento keyword non deve inventare una categoria come MSP; deve partire dalla tesi esplicita e dai codici ATECO, producendo un intento ampio tipo "servizi IT / infrastrutture / data processing / tecnologie informatiche" e distinguendo segnali core, adiacenti e fuori tesi.
- Per il laboratorio e' preferibile poter scegliere una sessione M&A esistente e un target gia' emerso, invece di reinserire manualmente i dati. Questo permette di testare la web evidence sullo stesso perimetro usato da retrieval/scoring e di confrontare segnale deterministico e segnale web.
- Implementata nella pagina `test` una tab sperimentale Evidence pipeline: pick sessione/target M&A, keyword set deterministico derivato da strategia/ATECO, domain resolution senza identificativi fiscali di default, ricerche `site:` separate per termini core/adiacenti/negativi e score web non persistito.
- Primo test pipeline su ETC S.R.L.: il resolver ha promosso un dominio di fonte terza a bassa confidenza e la keyword evidence ha poi cercato su tutta una directory, producendo falsi positivi su aziende diverse. Correzioni necessarie: mai eseguire site evidence su dominio `bassa`, contare come match solo risultati rilevanti e distinguere fonte terza da dominio ufficiale citato.
- Chiarimento metodologico: non creare blacklist/esclusioni directory a partire da pochi risultati. La correzione deve essere generale: classificare il ruolo della fonte, estrarre eventuali domini citati come `sameAs`/sito ufficiale/email aziendale, promuoverli solo se validati da soglia e segnali compatibili, e fermarsi quando non c'e' un dominio ufficiale credibile.
- Test pipeline su NETX64 SRL: il risultato conferma la distinzione fonte/candidato. `atoka.io` contiene evidenza anagrafica, ma cita `netx64.com` come `sameAs`; la pipeline deve usare il dominio citato e validato, non il repertorio che lo contiene.
- Passo successivo implementato: candidate match analyst LLM come secondo stadio stateless. Riceve solo target, keyword set, dominio selezionato, evidence runs e summary; non fa browsing, non sceglie domini e non sovrascrive lo score deterministico. Produce verdict/action/rationale, segnali pro/contro e alias concettuali utili a colmare il limite delle keyword letterali.
- Taratura web evidence: evitare clamp a 100 quando ci sono pochi match core e nessun negativo. Lo score sperimentale deve separare domainScore, sectorEvidenceScore, coverageScore e negativePenalty; il verdict LLM puo' confermare qualitativamente un match forte, ma non deve trasformare copertura parziale in punteggio perfetto.
- Caso COHERENCY SRL: la query legale/settoriale puo' restituire solo directory e quindi nessun dominio credibile. Il resolver deve fare fallback generici brand-first (`brand + sito ufficiale/contatti/chi siamo`, poi `brand + contattaci + intenti`) e fermarsi appena emerge un dominio credibile, senza introdurre blacklist ad hoc per le directory osservate.
- Passo successivo implementato nel laboratorio: final reconciliation deterministica sopra score camerale, domain resolution, web evidence e analyst LLM. L'esito operativo separa `confirm`, `deprioritize`, `reject`, `needs_domain_review` e `needs_business_validation`, evitando che uno score camerale alto resti automaticamente prioritario quando web/LLM declassano il candidato.
- Evoluzione successiva: la final reconciliation non vive piu' solo in `TestPage`; l'endpoint dell'analyst restituisce `finalDecision` come artifact backend. La UI lo usa come fonte primaria e mantiene il calcolo locale solo come fallback quando l'LLM/endpoint non e' disponibile.
- Persistenza introdotta: `ma_target_web_validation` salva l'ultima validazione web/LLM per `(session_id, company_key)`, con campi denormalizzati per `final_action`, `web_validation_state`, dominio e score, piu' artifact JSON completi. `GET /ma/sessions/{id}` ricarica la validazione nel target, mentre il laboratorio fa un upsert best-effort dopo ogni pipeline run.
- Cache read-through nel laboratorio: se il target ha gia' `webValidation` e il keyword set attuale coincide con quello salvato, la pipeline riusa l'artifact senza rifare Brave/LLM. Il toggle `Forza ricalcolo` rigenera e sovrascrive l'ultima validazione.
- Freshness introdotta: la validazione salvata ora porta `pipeline_version`, `input_hash`, `keyword_set_hash`, metadata modello/prompt LLM, `stale_after`, `expires_at` e stato runtime `fresh/stale/expired`. Il lab riusa la cache solo se versione, hash e freshness sono compatibili; altrimenti ricalcola e sovrascrive.
- Arricchimento web asincrono introdotto nel backend: nuovo job `ma_job.job_type = web_validation`, protetto dall'indice unico `(session_id, job_type)` per righe queued/running e dal lease DB. Il worker processa top target in sequenza, salta artifact fresh con stesso hash e salva l'esito in `ma_target_web_validation`; la pagina Target puo' avviarlo e visualizzare chip/stato senza passare dal laboratorio.

## Domande per brainstorming

- Dove conviene inserire LLM nella seconda parte: scoring, critica qualitativa, arricchimento web, spiegazione, o solo deep-dive?
- Quali dati devono restare filtri duri e quali ranking signal?
- Come mostrare all'utente criteri usati/non usati senza appesantire il workflow?
- Come evitare che un segnale web costoso o volatile contamini la lista base?
