# Report Analisi Policy Manutenzioni CDLAN

Documento sorgente analizzato: [`docs/policy-manutenzioni-cdlan.md`](policy-manutenzioni-cdlan.md).

## Sintesi

La policy definisce le manutenzioni come l'insieme delle operazioni necessarie a garantire efficienza, qualita' e sicurezza degli apparati e dei servizi erogati. La regola guida e' che tutte le manutenzioni devono essere pianificate tenendo conto degli impatti sui servizi erogati.

La policy e' utilizzabile come fonte per processo, tassonomie e vincoli base, ma non e' ancora sufficiente da sola per automatizzare tutte le validazioni senza decisioni aggiuntive: contiene regole chiare su processo, preavvisi, fasce orarie, MOP e owner, ma anche diversi `TBD`, campi vuoti e dipendenze da documenti esterni.

## Tipologie Di Manutenzione

### Manutenzione ordinaria

Attivita' prevista e pianificata periodicamente a intervalli prestabiliti di tempo o dopo una durata di funzionamento assegnata.

Scopi principali:

- mantenere l'integrita' e le caratteristiche funzionali originarie o in essere di un apparato/servizio;
- mantenere o ripristinare l'efficienza di un apparato/servizio;
- contrastare il normale degrado;
- assicurare la vita utile dell'apparato/servizio.

### Manutenzione straordinaria

Attivita' non prevista, ma per cui e' possibile pianificare l'intervento.

La policy la divide in tre sottocategorie:

- manutenzione migliorativa;
- manutenzione correttiva;
- manutenzione preventiva rilevante.

### Manutenzione in emergenza

Attivita' non prevista che deve essere attuata nell'immediatezza dell'evento.

## Procedura Operativa

Per ogni manutenzione il Team Leader deve individuare un owner.

L'owner della manutenzione deve:

- schedulare la manutenzione nei calendari Manutenzioni DC e/o Manutenzioni TLC/Cloud;
- considerare eventuali impatti con altre manutenzioni;
- considerare preavviso e fasce orarie applicabili;
- indicare nel Calendar il numero di MOP e l'owner della manutenzione;
- per le manutenzioni TLC/CLOUD, tracciare l'attivita' sul progetto Manutenzioni TLC/Cloud;
- redigere un MOP con descrizione dettagliata dei passaggi dell'attivita';
- se l'owner non e' il Team Leader, chiedere l'approvazione del MOP al Team Leader;
- se sono coinvolte altre persone interne o esterne, organizzare un briefing iniziale di coordinamento e riportarne l'esito sul MOP;
- per le manutenzioni DC, redigere il Verbale di Coordinamento;
- comunicare la manutenzione secondo quanto previsto dalla policy Informazioni - Comunicazioni;
- inviare il MOP ai clienti impattati, se contrattualmente dovuto;
- organizzare un debriefing finale, annotare sul MOP l'esito della manutenzione e analizzare l'intervento per miglioramento continuo.

## Preavvisi

Per manutenzioni ordinarie:

| Ambito | Preavviso |
| --- | ---: |
| DC | 30 giorni |
| TLC/CLOUD con impatto su servizi esterni | 7 giorni |
| TLC/CLOUD con impatto su servizi interni | 3 giorni |
| AWS | 14 giorni |

Per manutenzioni straordinarie:

| Ambito | Preavviso |
| --- | --- |
| DC | TBD |
| TLC/CLOUD con impatto su servizi esterni | TBD |
| TLC/CLOUD con impatto su servizi interni | TBD |
| AWS | Non definito |

## Fasce Orarie

`BH` significa `Business Hours`.

Per manutenzioni ordinarie:

| Ambito | Fascia |
| --- | --- |
| DC | Business Hours |
| TLC/CLOUD con impatto su servizi esterni | Dopo le 22:00 nei giorni feriali; durante weekend o festivi, salvo diversi accordi con i clienti |
| TLC/CLOUD con impatto su servizi interni | Dopo le 20:00 nei giorni feriali; durante weekend o festivi |
| CLOUD/APPLICATIONS con impatto su servizi interni | Dopo le 20:00 nei giorni feriali; durante weekend o festivi |

Per manutenzioni straordinarie:

| Ambito | Fascia |
| --- | --- |
| DC | Business Hours |
| TLC/CLOUD con impatto su servizi esterni | In base a gravita' |
| TLC/CLOUD con impatto su servizi interni | In base a gravita' |
| CLOUD/APPLICATIONS con impatto su servizi interni | Non definita |

Eccezione n8n: la sospensione del servizio deve essere programmata in Business Hours, per permettere il monitoraggio dei ticket Incident direttamente dall'applicativo di ticket management.

## Nomenclatura MOP

Il nome file del MOP deve includere:

- prefisso del team: `DC`, `TLC`, `CLOUD`;
- `#` piu' numero progressivo da inizio anno;
- anno;
- descrizione attivita'.

Esempio:

```text
DC#27_25 GE1 Maintenance
```

Per manutenzioni ricorrenti, i team preparano MOP standard identificati con `STD`.

Esempio:

```text
DC-STD GE Maintenance
```

## Archiviazione

Tutti i MOP, prodotti internamente o da tecnici esterni, devono essere firmati digitalmente dall'owner della manutenzione e archiviati in Arxivar usando il Tool Manutenzioni.

Stato indicato dalla policy:

- il Tool Manutenzioni e' pronto solo per le manutenzioni DC;
- TLC/Cloud archiviano temporaneamente su Drive TLC, Drive Cloud o Drive Applications.

## Classificazione

Le manutenzioni sono classificate in base a quattro parametri:

- `INT/EST`: a carico interno/esterno;
- difficolta' di esecuzione: da 1 a 3;
- difficolta' di rollback: da 1 a 3;
- complessita': media tra esecuzione e rollback.

Questa classificazione determina quali ruoli possono essere owner della manutenzione.

## Classificazione DATA CENTER

La sezione DC e' la piu' completa. Per ogni tipologia sono indicati:

- natura interna o esterna;
- difficolta' di esecuzione;
- difficolta' di rollback;
- complessita';
- owner ammessi.

Osservazioni principali:

- molte manutenzioni con complessita' 1 o 1,5 possono essere ownerate da `DC Management` o `DC Operation`;
- alcune manutenzioni con complessita' 3 sono limitate a `DC Management`;
- `Manutenzione di Ramo` e' classificata `INT/EXT`, con esecuzione 3, rollback 3, complessita' 3 e owner `DC Management`.

## Classificazione TLC

La sezione TLC include tipologie e owner, ma alcuni valori di complessita' sono incompleti o variabili.

Esempi:

- aggiornamenti sistemi operativi apparati rete: `INT`, esecuzione 1, rollback 1, complessita' 1, owner `TLC Assurance`;
- aggiornamenti sistemi operativi firewall: `INT`, esecuzione 2, rollback 2, complessita' 2, owner `TLC Security`;
- manutenzione carrier con impatto sui servizi: `EXT`, owner `TLC Assurance`, ma difficolta' e complessita' non compilate;
- sostituzione hardware e migrazione servizi: `INT`, complessita' variabile, owner `TLC Engineering`.

La policy specifica che per alcune attivita' TLC il livello di complessita' varia in base all'hardware da sostituire e ai servizi da migrare.

## Classificazione CLOUD

La sezione Cloud elenca varie tipologie, ma non assegna owner.

Tipologie con complessita' definita:

- aggiornamenti sistemi operativi: `INT`, esecuzione 1, rollback 1, complessita' 1;
- patching di sicurezza dei sistemi operativi: `INT`, esecuzione 1, rollback 1, complessita' 1;
- aggiornamenti versione degli applicativi: `INT`, esecuzione 1, rollback 1, complessita' 1;
- patching di sicurezza delle componenti applicative: `INT`, esecuzione 1, rollback 1, complessita' 1;
- sostituzione o aggiunta di componenti hardware ad apparati: `INT`, esecuzione 1, rollback 1, complessita' 1.

Tipologie con complessita' variabile:

- modifica configurazione dei sistemi operativi;
- modifica configurazioni degli applicativi;
- sostituzione apparati hardware;
- migrazioni di sistemi.

La policy specifica che la complessita' varia in base ai sistemi/applicativi, all'hardware da sostituire e ai servizi da migrare.

## Punti Aperti E Ambiguita'

- I preavvisi per manutenzioni straordinarie sono `TBD` o non definiti.
- Le manutenzioni in emergenza non hanno regole tabellari su preavviso, calendario, comunicazioni o MOP.
- `BH` e' stato chiarito come `Business Hours`, ma il documento sorgente non lo esplicita.
- La sezione Cloud non assegna owner alle tipologie elencate.
- Alcune classificazioni TLC/Cloud sono incomplete o variabili.
- La policy comunicazioni e' esterna e non e' incorporata nel documento.
- La regola "inviare il MOP ai clienti impattati se contrattualmente dovuto" richiede dati contrattuali non presenti nella policy.
- C'e' incoerenza terminologica tra `INT/EST` e `INT/EXT`.
- La procedura dipende da strumenti esterni: Calendar, Zoho, Google Drive, Arxivar, Tool Manutenzioni.
- L'attuale Tool Manutenzioni e' dichiarato pronto solo per DC, mentre TLC/Cloud usano archiviazione temporanea su Drive.

## Implicazioni Per Automazione O Prodotto

Per trasformare questa policy in regole applicative servono decisioni aggiuntive su:

- definizione operativa esatta di `Business Hours`, se deve essere validata automaticamente;
- gestione delle manutenzioni straordinarie con valori `TBD`;
- gestione delle emergenze;
- mappatura dei ruoli Cloud;
- normalizzazione `INT/EST` vs `INT/EXT`;
- fonte dati per obblighi contrattuali di invio MOP ai clienti;
- fonte dati e workflow per la policy comunicazioni;
- regole di validazione per complessita' variabile `*`;
- differenza tra vincoli bloccanti e semplici warning.

## Conclusione

La policy fornisce una base solida per descrivere il ciclo operativo delle manutenzioni e per avviare una modellazione di prodotto. Le parti piu' robuste sono procedura, preavvisi ordinari, fasce ordinarie e classificazione DC. Le parti da completare prima di un'automazione completa sono straordinarie, emergenze, owner Cloud, comunicazioni, obblighi contrattuali e regole per complessita' variabile.
