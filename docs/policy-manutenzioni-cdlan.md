# **Definizione**

Insieme delle operazioni necessarie a garantire efficienza, qualità e sicurezza degli apparati e dei servizi erogati.

<aside>
💡

**Tutte le manutenzioni devono essere pianificate tenendo conto degli impatti sui servizi erogati.**

</aside>

## **Manutenzione ordinaria**

Attività **prevista e pianificata** periodicamente a intervalli prestabiliti di tempo o dopo una assegnata durata di funzionamento.

Lo scopo delle manutenzioni ordinarie è:

- *mantenere l'integrità e le caratteristiche funzionali originarie / in essere di un apparato/servizio*
- *mantenere o ripristinare l'efficienza di un apparato/servizio*
- *contrastare il normale degrado*
- *assicurare la vita utile dell’apparato/servizio*

## **Manutenzione straordinaria**

Attività **non prevista, in cui è possibile pianificare l’intervento.**

Le manutenzione straordinaria può essere divisa in tre sottocategorie:

- [*Manutenzione migliorativa](https://it.wikipedia.org/wiki/Manutenzione_migliorativa) (Insieme delle azioni migliorative)*
- [*Manutenzione correttiva](https://it.wikipedia.org/wiki/Manutenzione_correttiva), quando l'intervento correttivo aumenta in modo significativo il valore residuo e/o la longevità dell'apparato/servizio, il cui scopo non è dettato da un'esigenza impellente di ripristinare il livello ottimale di funzionamento, ma piuttosto da una gestione economica, nel tempo, dell'apparato/servizio mantenuto.*
- [*Manutenzione preventiva](https://it.wikipedia.org/wiki/Manutenzione_preventiva) rilevante (quali ad esempio revisioni, che aumentano il valore dei sistemi e/o ne prolungano la longevità)*

## Manutenzione in emergenza

Manutenzione **non prevista**, che deve essere attuata nell’immediatezza dell’evento.

## **Procedura di Manutenzione**

Per ogni manutenzione il Team Leader deve individuare un **owner**.

L’owner della Manutenzione deve:

- schedulare la manutenzione nei Calendario Manutenzioni DC e/o Manutenzioni TLC/Cloud, considerando:
    - eventuali impatti con altre Manutenzioni
    - [preavviso](https://www.notion.so/Manutenzioni-28a1d70811088030a87de8a55cea3e37?pvs=21)
    - [fasce orarie](https://www.notion.so/Manutenzioni-28a1d70811088030a87de8a55cea3e37?pvs=21) di Manutenzione
- nel Calendar, indicare il numero di MOP e l’owner della manutenzione
- per le manutenzioni TLC/CLOUD, tracciare l’attività su progetto [Manutenzioni TLC/Cloud](https://projects.zoho.eu/portal/caldera21#todomilestones/156936000001577071/customview/156936000001705003)
- redigere un [MOP](https://docs.google.com/document/d/15WZpgQbhs8ZG98A5aZ1wtliRw_sEm0Wh/edit?tab=t.0#heading=h.zhr8ez50h7wu) (documento con descrizione dettagliata di ciascun passaggio di una specifica attività)
- se l'owner non è il Team Leader, chiedere l’approvazione del MOP al Team Leader
- se la manutenzione coinvolge altre persone (interne o esterne), organizzare un Briefing iniziale di coordinamento e riportarne esito sul MOP
    - per le manutenzioni di DC, redigere il [Verbale di Coordinamento](https://drive.google.com/drive/folders/1Q-8xJXkZTeLEVRQxtedFvAjgpEMRjCTC)
- comunicare la Manutenzione secondo quanto previsto in [Informazioni - Comunicazioni](https://www.notion.so/1ec1d708110880ad9c13d102dffa1497?pvs=21)
- inviare il MOP ai Clienti impattati, se contrattualmente dovuto
- organizzare un Debriefing finale durante il quale annotare su MOP l’esito della manutenzione e analizzare l’intervento a 360°utile ad apportare gli eventuali correttivi al MOP in ottica di miglioramento continuo.

### Preavviso

|  | DC | TLC/CLOUD - Impatto su servizi ESTERNI | TLC/CLOUD - Impatto su servizi INTERNI | impatto su AWS |
| --- | --- | --- | --- | --- |
| Ordinarie | 30 gg | 7 gg | 3 gg | 14gg |
| Straordinarie | TBD | TBD | TBD |  |

### Fasce Orarie

|  | DC | TLC/CLOUD - Impatto su servizi ESTERNI | TLC/CLOUD - Impatto su servizi INTERNI | CLOUD/APPLICATIONS - Impatto su servizi INTERNI |
| --- | --- | --- | --- | --- |
| Ordinarie | BH | dopo le ore 22:00 nei giorni feriali;  durante il weekend o nei giorni festivi, salvo diversi accordi con i clienti. | dopo le ore 20:00 nei giorni feriali;  durante il weekend o nei giorni festivi. | dopo le ore 20:00 nei giorni feriali;  durante il weekend o nei giorni festivi.

BH - applicativo n8n* |
| Straordinarie | BH | in base a gravità  | in base a gravità  |  |

*****applicativo n8n**:** l’applicativo n8n gestisce l’invio di notifiche ad iLert per i ticket di Incident. La sospensione del servizio deve essere programmata in BH per permettere il monitoraggio dei ticket direttamente dall’applicativo per il ticket management.

### Nomenclatura MOP

Il MOP avrà come nome file 

- il prefisso del Team (DC, TLC, CLOUD)
- # + numero progressivo da inizio anno
- anno
- descrizione attività
    
    esempio: DC#27_25 GE1 Maintenance
    

Per le manutenzioni ricorrenti i Team preparano dei MOP Standard, identificati in nomenclatura con STD, esempio DC-STD GE Maintenance.

### **Archiviazione**

Tutti i MOP, prodotti internamente o da tecnici esterni,  devono essere firmati digitalmente dall'owner della manutenzione ed archiviati in Arxivar utilizzando il [Tool Manutenzioni](https://manutenzioni.cdlan.net/).

⇒ il Tool al momento è pronto solo per le manutenzioni DC. TLC/Cloud nel frattempo archiviano in [Drive TLC](https://drive.google.com/drive/folders/165CLeJTkJZmVZmNGH1jDcRi2DczeDsRI) / [Drive Cloud](https://drive.google.com/drive/folders/1-V8LLh8KrMlDwvBl37vAMOias5B34X5E) / [Drive Applications](https://drive.google.com/drive/folders/1addHlz4oN3vurslV6-ZlyI2EUjmbPFTS)

## **Classificazione delle Manutenzioni**

Le manutenzioni sono classificate in base a 4 parametri:

- INT/EST: a carico interno/esterno
- Difficoltà di esecuzione: da 1 a 3
- Difficoltà di Roll-back: da 1 a 3
- Complessità: media tra esecuzione e roll-back

Sulla base di questa classificazione si definiscono i ruoli che possono essere owner della manutenzione.

# DATA CENTER

## **Classificazione delle Manutenzioni DC**

| TIPOLOGIA | INT/EXT | Esecuzione | RollBack | Complessità | Owner |
| --- | --- | --- | --- | --- | --- |
| CDZ Sale | EXT | 1 | 2 | 1,5 | DC Management, DC Operation |
| Impianti meccanici DC | EXT | 1 | 1 | 1 | DC Management, DC Operation |
|  |  |  |  |  |  |
| Switching Test | INT | 1 | 2 | 1,5 | DC Management, DC Operation |
| PTP | INT | 2 | 2 | 2 | DC Management, DC Operation |
| MT QEMT.x | EXT | 3 | 3 | 3 | DC Management |
| MT/GE | EXT | 2 | 2 | 2 | DC Management, DC Operation |
| GE | EXT | 1 | 1 | 1 | DC Management, DC Operation |
| UPS IT | EXT | 3 | 2 | 2,5 | DC Management, DC Operation |
| UPS MEC | EXT | 2 | 2 | 2 | DC Management, DC Operation |
| QEMT | EXT | 2 | 3 | 2,5 | DC Management, DC Operation |
| QEGBT | INT | 3 | 3 | 3 | DC Management |
| QEUPS | INT | 2 | 3 | 2,5 | DC Management, DC Operation |
| QECPx | INT | 3 | 3 | 3 | DC Management |
| QEMPx | INT | 3 | 3 | 3 | DC Management |
| QEPDUPx (Isole condivise) | INT | 2 | 2 | 2 | DC Management, DC Operation |
| QEPDUPx (Cage) | INT | 2 | 3 | 2,5 | DC Management, DC Operation |
| Impianto rivelazione fumi | EXT | 1 | 1 | 1 | DC Management, DC Operation |
| Impianto spegnimento | EXT | 1 | 1 | 1 | DC Management, DC Operation |
| Impianto antintrusione | EXT | 1 | 1 | 1 | DC Management, DC Operation |
| Test Differenziali | INT | 2 | 1 | 1,5 | DC Management, DC Operation |
| Manutenzione di Ramo | INT/EXT | 3 | 3 | 3 | DC Management |

# **TLC**

## **Classificazione delle Manutenzioni TLC**

| TIPOLOGIA | INT/EXT | Esecuzione | RollBack | Complessità | Owner |
| --- | --- | --- | --- | --- | --- |
| Aggiornamenti sistemi operativi apparati rete | INT | 1 | 1 | 1 | TLC Assurance  |
| Aggiornamenti sistemi operativi firewall | INT | 2 | 2 | 2 | TLC Security |
| Sostituzione o aggiunta componenti hardware ed apparati | INT | 1 | 1 | 1 | TLC Assurance |
| Aggiornamento console firewall Forcepoint | INT | 1 | 1 | 1 | TLC Security |
| Manutenzione dei carrier che impattano i servizi | EXT |  |  |  | TLC Assurance |
| Sostituzione hardware e migrazione dei servizi | INT | * | * | * | TLC Engineering |

* il livello di complessità varia in base all’hardware da sostituire ed ai servizi da migrare

# **CLOUD**

## **Classificazione delle Manutenzioni CLOUD**

| TIPOLOGIA | INT/EXT | Esecuzione | RollBack | Complessità | Owner |
| --- | --- | --- | --- | --- | --- |
| Aggiornamenti sistemi operativi | INT | 1 | 1 | 1 |  |
| modifica configurazione dei sistemi operativi | INT | * | * | * |  |
| patching di sicurezza dei sistemi operativi | INT | 1 | 1 | 1 |  |
| aggiornamenti versione degli applicativi | INT | 1 | 1 | 1 |  |
| modifica configurazioni degli applicativi | INT | * | * | * |  |
| patching di sicurezza delle componenti applicative | INT | 1 | 1 | 1 |  |
| sostituzione o aggiunta di componenti hardware ad apparati | INT | 1 | 1 | 1 |  |
| sostituzione apparati hardware | INT | * | * | * |  |
| migrazioni di sistemi | INT | * | * | * |  |

* il livello di complessità varia in base ai sistemi/applicativi , all’hardware da sostituire, ed ai servizi da migrare