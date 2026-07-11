# Handoff — stile delle email HTML

Questo documento descrive il design system delle email di notifica **MrSmith · RDA** in modo che possa essere riprodotto in un altro progetto, indipendentemente dal linguaggio o dal motore di template usato.

## 1. Obiettivo visivo

Lo stile deve risultare:

- sobrio, operativo e immediatamente riconoscibile;
- leggibile anche nei client email con supporto CSS limitato;
- privo di dipendenze da immagini o font remoti;
- adatto sia a una singola entità sia a un digest di più entità;
- coerente con il brand tramite fascia blu, hairline rossa e accenti cromatici per categoria.

Il contenuto corrente riguarda richieste di acquisto, ma la struttura può essere riusata sostituendo brand, tassonomia, copy, campi e URL.

## 2. Vincoli tecnici

Implementare l'HTML secondo queste regole:

- layout esclusivamente a tabelle con `role="presentation"`;
- CSS inline, senza dipendere da classi, fogli esterni o JavaScript;
- contenitore centrato con larghezza massima di `600px`;
- tabelle con `cellpadding="0"` e `cellspacing="0"`;
- nessun `float`: usare tabelle annidate per allineamenti a colonne;
- CTA “bulletproof”, applicando lo sfondo alla cella `<td>` e il padding al link;
- nessuna immagine remota necessaria al branding;
- font con fallback di sistema;
- link e contenuti dinamici correttamente escapati/sanitizzati dal sistema ospite.

Font stack:

```css
'Work Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif
```

`Work Sans` è solo una preferenza: non viene caricato un webfont. Outlook e gli altri client possono usare il fallback disponibile.

## 3. Design token

| Token | Valore | Uso |
|---|---:|---|
| Blu primario | `#1A3E6E` | header, titoli, codici, link, CTA |
| Rosso brand | `#E1251B` | sola hairline superiore, non per azioni |
| Giallo | `#FFBA1F` | categoria budget |
| Arancio | `#FF6600` | categoria pagamenti |
| Viola | `#7D00FF` | categoria fornitori |
| Verde | `#00AA46` | categoria ERP/invio |
| Testo | `#444444` | corpo principale |
| Testo secondario | `#767676` | label e metadati |
| Testo footer | `#8A8A8A` | note di servizio |
| Bordo | `#E0E0E0` | bordo contenitore e card |
| Divider | `#EEF1F4` | separatori interni |
| Bianco | `#FFFFFF` | sfondo e testo su blu |
| Separatore header | `#AEBFD6` | breadcrumb secondario |

### Mappa categorie

| Categoria semantica | Colore accento |
|---|---:|
| Approvazione | blu `#1A3E6E` |
| Budget | giallo `#FFBA1F` |
| ERP / invio | verde `#00AA46` |
| Fornitore | viola `#7D00FF` |
| Pagamento | arancio `#FF6600` |

L'accento compare nel piccolo indicatore accanto all'eyebrow. Il rosso resta riservato alla hairline.

## 4. Anatomia dell'email

Il contenitore principale è una tabella bianca centrata:

```css
max-width:600px;
margin:0 auto;
background:#ffffff;
border:1px solid #E0E0E0;
border-radius:10px;
overflow:hidden;
```

La struttura verticale, nell'ordine, è:

1. **Hairline brand** — altezza `3px`, sfondo rosso.
2. **Header** — fascia blu con breadcrumb testuale.
3. **Testata del messaggio** — eyebrow, titolo e introduzione.
4. **Contenuto operativo** — card singola oppure lista digest.
5. **Footer** — separatore e nota automatica.

### 4.1 Header

- sfondo `#1A3E6E`;
- padding `15px 24px`;
- breadcrumb testuale corrente: **MrSmith · RDA · Richieste di Acquisto**;
- nomi principali bianchi, `14px`, peso `700`;
- separatori e descrizione `#AEBFD6`, `13px`.

Nel nuovo contesto mantenere il pattern `Prodotto · Modulo · Area`, adattando le etichette.

### 4.2 Eyebrow

- indicatore quadrato `9 × 9px`;
- raggio `2px`;
- margine destro `7px`;
- colore derivato dalla categoria;
- testo `11px`, peso `700`, uppercase;
- letter spacing `0.14em`;
- il testo usa lo stesso colore dell'indicatore.

L'eyebrow descrive lo stato o il tipo di azione, per esempio “Approvazione 1° livello”.

### 4.3 Titolo e introduzione

La cella della testata usa `padding:28px 24px 0`.

Titolo:

```css
margin:9px 0 0;
font-size:21px;
line-height:1.25;
color:#1A3E6E;
font-weight:700;
letter-spacing:-0.01em;
```

Introduzione:

```css
margin:12px 0 0;
font-size:15px;
line-height:1.6;
color:#444444;
```

Il copy deve spiegare in modo breve **che cosa è successo** e **quale azione è richiesta**.

### 4.4 Area contenuto

La cella esterna usa `padding:18px 24px 4px`. Il rendering dipende dal numero di elementi:

- **1 elemento:** card di dettaglio seguita da una CTA;
- **2 o più elementi:** lista digest, con un link per riga.

Questa biforcazione è parte integrante dell'esperienza e va mantenuta.

## 5. Variante singola: card di dettaglio

Usare una tabella larga `100%`, con:

```css
border:1px solid #E0E0E0;
border-radius:8px;
```

Ogni riga è composta da label a sinistra e valore a destra:

- padding celle `10px 14px`;
- font `13px`;
- label `#767676` e peso `400`;
- valore `#444444`, allineato a destra;
- separatore inferiore `1px solid #EEF1F4`, escluso sull'ultima riga.

Il primo campo identifica sempre l'entità:

- label, per esempio `Codice`;
- valore in blu `#1A3E6E`;
- peso `700`.

Gli altri campi dipendono dal dominio. Evidenziare in grassetto solo i valori davvero rilevanti, per esempio un importo.

### CTA

Dopo la card inserire una tabella con `margin-top:20px`. La cella del pulsante usa:

```css
background:#1A3E6E;
border-radius:8px;
```

Il link usa:

```css
display:inline-block;
padding:12px 22px;
font-size:14px;
font-weight:600;
color:#ffffff;
text-decoration:none;
```

Etichetta corrente: `Apri il PO →`. Nel nuovo contesto usare un verbo esplicito e l'oggetto dell'azione, mantenendo la freccia finale.

## 6. Variante multipla: digest

Usare una tabella larga `100%`. Ogni elemento occupa una riga con:

```css
padding:13px 4px;
border-bottom:1px solid #EEF1F4;
```

Dentro la riga usare una seconda tabella a due colonne:

- sinistra: identificativo e metadati;
- destra: link `Apri →`;
- entrambe le celle con `valign="top"`;
- link allineato a destra e `white-space:nowrap`.

Identificativo:

- `14px`;
- peso `700`;
- colore `#1A3E6E`.

Metadati opzionali:

```css
font-size:12.5px;
color:#767676;
font-weight:400;
margin-top:3px;
```

Link di riga:

```css
font-size:13px;
color:#1A3E6E;
text-decoration:none;
font-weight:600;
```

I metadati devono restare compatti e scansionabili, separando le informazioni con `·`. Se non esistono dati utili, mostrare solo identificativo e link.

## 7. Footer

La cella usa:

```css
padding:24px;
border-top:1px solid #EEF1F4;
```

Il testo usa:

```css
margin:16px 0 0;
font-size:12px;
line-height:1.5;
color:#8A8A8A;
```

Testo corrente:

> Email automatica dal portale MrSmith · RDA — non rispondere a questo messaggio.

Adattare prodotto e modulo, mantenendo l'indicazione che il messaggio è automatico e non richiede risposta.

## 8. Regole editoriali e dati

Per ogni tipo di notifica definire separatamente:

- categoria cromatica;
- eyebrow;
- subject singolare e plurale;
- titolo singolare e plurale;
- introduzione singolare e plurale;
- campi della card singola;
- metadati della riga digest.

Pattern del subject:

```text
[Prodotto][Modulo] <codice> · <azione/stato>
[Prodotto][Modulo] <N> elementi · <azione/stato>
```

Regole consigliate:

- usare frasi brevi e orientate all'azione;
- evitare di ripetere nel titolo tutte le informazioni già presenti nella card;
- gestire esplicitamente singolare e plurale, senza affidarsi a concatenazioni fragili;
- ordinare stabilmente gli elementi del digest per codice o altro identificativo prevedibile;
- raggruppare una email per destinatario e tipo di notifica;
- formattare date, numeri e valute secondo il locale del destinatario.

Nel sistema corrente gli importi seguono il formato italiano: `24.500,00 €`.

## 9. Contratto dati suggerito

Un'implementazione portabile può ricevere un modello simile:

```text
EmailNotification
  brand.product
  brand.module
  brand.area
  category
  eyebrow
  subject
  title
  intro
  items[]
    id
    url
    fields[]       # label/value, per la variante singola
    meta           # testo compatto, per il digest
  footer
```

Il renderer decide la variante in base a `items.length`. Dati e copy devono restare separati dal markup condiviso.

## 10. Invio

Requisiti minimi:

- MIME body `text/html`;
- display name mittente coerente, attualmente `MrSmith · RDA`;
- URL assoluti e normalizzati;
- subject coerente con singolo/digest.

L'implementazione corrente non genera una parte plain-text. In un nuovo sistema è raccomandabile aggiungere una variante `text/plain` multipart, senza modificare l'aspetto HTML.

## 11. Checklist di accettazione

- [ ] Contenitore massimo `600px`, centrato e leggibile su mobile.
- [ ] Tabelle presentation e CSS inline.
- [ ] Nessuna risorsa remota necessaria alla comprensione.
- [ ] Hairline rossa alta `3px` e fascia header blu.
- [ ] Eyebrow con accento coerente alla categoria.
- [ ] Tipografia, colori, padding e raggi rispettano i token.
- [ ] Un elemento produce card + CTA.
- [ ] Più elementi producono digest con link per riga.
- [ ] Singolare e plurale sono editorialmente corretti.
- [ ] Dati dinamici, attributi e URL sono escapati in sicurezza.
- [ ] Link assoluti e azionabili.
- [ ] Rendering verificato almeno su Gmail web/mobile, Apple Mail e Outlook desktop.
- [ ] Test snapshot o golden file coprono shell, card, digest e casi senza metadati.

## 12. Riferimenti nell'implementazione originale

- Tema e componenti HTML: `internal/controller/notification_theme.go`
- Copy, categorie e scelta singolo/digest: `internal/controller/notification_handlers.go`
- Raggruppamento e ordinamento: `internal/controller/process_po.go`
- Invio MIME HTML: `pkg/cdmailer/mailer.go`
- Costruzione URL: `internal/utils/helpers.go`

Questi file sono il riferimento normativo in caso di divergenza tra il documento e il comportamento corrente.