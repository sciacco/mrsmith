#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
pa_card.py — Esempio illustrativo: la "scheda" di un ordine di acquisto.

ATTENZIONE: questo file è un **esempio da leggere**, non uno strumento pronto
all'uso. Mostra, in modo concreto, quali interrogazioni servono per ricostruire
la scheda completa di un ordine dello schema `pa` (database `arak`) e come
se ne può formattare l'output.

È pensato come riferimento per il progetto consumer:

  - le SELECT nelle funzioni `fetch_*` sono **SQL standard**, riusabile con
    qualsiasi linguaggio/driver (sono il vero contenuto dell'esempio);
  - la funzione `run_query` è il **punto di connessione** al database e va
    adattata al proprio stack (psycopg2, psycopg, JDBC, SQLAlchemy, …);
  - le funzioni `render_*` sono solo un esempio di formattazione testuale.

Nello schema `pa` tutto ruota attorno all'ordine (chiave `issue_key`, es.
`PA-1821`); le tabelle satelliti si collegano tramite `issue_id`.
Riferimenti: docs/INDEX.md, docs/schema.md, docs/reporting.md, docs/esempio.md.

Esecuzione (solo se si vuole provare l'esempio)
-----------------------------------------------
Python 3 + `psycopg2` (o `psycopg`). La connessione usa le variabili d'ambiente
standard di libpq: `PGHOST`, `PGPORT`, `PGDATABASE` (default `arak`),
`PGUSER`, `PGPASSWORD`. Senza driver, `run_query` interrompe con un messaggio
chiaro: il file resta comunque leggibile come riferimento SQL.

    python3 pa_card.py PA-1821
    python3 pa_card.py PA-1821 --schema pa --history 20

Sezioni della scheda
--------------------
  1. INTESTAZIONE     ← pa.issue            (identità, stato, date, persone)
  2. ACQUISTO         ← pa.issue            (importi, fornitore, budget)
  3. DESCRIZIONE      ← pa.issue.description
  4. RIGHE D'ORDINE   ← pa.line_item        (Articoli/Merci/Servizi/Leasing)
  5. COMMENTI         ← pa.comment
  6. ALLEGATI         ← pa.attachment       (solo metadati)
  7. COLLEGAMENTI     ← pa.issue_link
  8. ULTIME MODIFICHE ← pa.change_group + pa.change_item
"""
import argparse
import os
import sys

SCH = "pa"
DB_DEFAULT = "arak"


# --------------------------------------------------------------------------- #
#  Connessione — l'unico punto da adattare al proprio stack
# --------------------------------------------------------------------------- #
def run_query(sql):
    """Punto di connessione al database (da adattare al proprio stack).

    Contratto: riceve una SELECT (stringa), ritorna una lista di dict
    (chiavi = nomi colonna). L'implementazione seguente usa `psycopg2` e le
    variabili d'ambiente standard di libpq; sostituirla con il driver di
    proprio uso mantenendo la firma.
    """
    try:
        import psycopg2  # type: ignore
    except ImportError:
        sys.exit(
            "run_query: driver non disponibile. Questo file è un esempio da "
            "leggere; per eseguirlo serve psycopg2 (o psycopg). Adattare "
            "run_query al proprio stack."
        )
    conn = psycopg2.connect(
        host=os.environ.get("PGHOST", "localhost"),
        port=os.environ.get("PGPORT", "5432"),
        dbname=os.environ.get("PGDATABASE", DB_DEFAULT),
        user=os.environ.get("PGUSER", "postgres"),
        password=os.environ.get("PGPASSWORD", ""),
    )
    try:
        with conn.cursor() as cur:
            cur.execute(sql)
            cols = [d[0] for d in cur.description]
            return [dict(zip(cols, row)) for row in cur.fetchall()]
    finally:
        conn.close()


# --------------------------------------------------------------------------- #
#  Sezioni "data" — una SELECT per blocco della scheda (riusabili così come sono)
# --------------------------------------------------------------------------- #
def fetch_issue(issue_key):
    """Dati principali dell'ordine (pa.issue). 1 riga = l'ordine."""
    return run_query(f"SELECT * FROM {SCH}.issue WHERE issue_key='{issue_key}'")


def fetch_line_items(issue_key):
    """Righe d'ordine (pa.line_item), ordinate per tipo e posizione."""
    return run_query(f"""
        SELECT grid, row_no, articolo_name, vendor, part_number, descrizione,
               quantita, importo, prezzo_totale, vendita,
               pagamento_name, durata_name, data_pagamento
        FROM {SCH}.line_item
        WHERE issue_key='{issue_key}'
        ORDER BY grid, row_no""")


def fetch_comments(issue_key):
    """Commenti dell'ordine (pa.comment), in ordine cronologico."""
    return run_query(f"""
        SELECT to_char(created,'YYYY-MM-DD HH24:MI') AS quando,
               author_name, actionbody
        FROM {SCH}.comment
        WHERE issue_key='{issue_key}'
        ORDER BY created""")


def fetch_attachments(issue_key):
    """Allegati (pa.attachment): SOLO metadati, i file non sono nel DB."""
    return run_query(f"""
        SELECT filename, pg_size_pretty(filesize) AS dim,
               to_char(created,'YYYY-MM-DD') AS quando, author_name
        FROM {SCH}.attachment
        WHERE issue_key='{issue_key}'
        ORDER BY created""")


def fetch_links(issue_key):
    """Collegamenti ad altri ordini (pa.issue_link)."""
    return run_query(f"""
        SELECT source_key, destination_key, link_name, inward, outward
        FROM {SCH}.issue_link
        WHERE source_key='{issue_key}' OR destination_key='{issue_key}'""")


def fetch_history(issue_key, limit=12):
    """Ultime modifiche (pa.change_group ⨝ pa.change_item), le più recenti."""
    return run_query(f"""
        SELECT to_char(g.created,'YYYY-MM-DD HH24:MI') AS quando,
               g.author_name, i.field, i.old_string, i.new_string
        FROM {SCH}.change_group g
        JOIN {SCH}.change_item i ON i.group_id = g.id
        WHERE g.issue_key='{issue_key}'
        ORDER BY g.created DESC, i.id
        LIMIT {limit}""")


# --------------------------------------------------------------------------- #
#  Helper di formattazione
# --------------------------------------------------------------------------- #
def nz(v):
    """Null/blank → trattino, per output leggibile."""
    return v if v not in (None, "", "None") else "—"


def eur(v):
    """Formatta un importo in stile italiano (1.234,56 €)."""
    try:
        return f"{float(v):,.2f} €".replace(",", "X").replace(".", ",").replace("X", ".")
    except (TypeError, ValueError):
        return nz(v)


def dt(v):
    """Tronca un timestamp a 'YYYY-MM-DD HH:MM' leggibile."""
    return (v or "")[:16].replace("T", " ") if v and v != "—" else "—"


def line(s="─", n=78):
    print(s * n)


# --------------------------------------------------------------------------- #
#  Sezioni "render" — trasformano i dati in testo (esempio di formattazione)
# --------------------------------------------------------------------------- #
def render_header(i):
    """1. INTESTAZIONE — identità, stato, date, persone (pa.issue)."""
    line("═")
    print(f"  {i['issue_key']}   ·   {nz(i['summary'])}")
    line("═")
    print(f"  Tipo: {nz(i['issue_type'])}    Stato: {nz(i['status'])}    "
          f"Priorità: {nz(i['priority'])}    Risoluzione: {nz(i['resolution'])}")
    print(f"  Numero ordine: {nz(i['numero_ordine'])}    Valuta: {nz(i['valuta'])}    "
          f"Stato (custom): {nz(i['stato'])}")
    print(f"  Creato: {dt(i['created'])}    Aggiornato: {dt(i['updated'])}    "
          f"Risolto: {dt(i['resolution_date'])}    Scadenza: {dt(i['due_date'])}")
    print(f"  Richiedente: {nz(i['reporter_name'])} ({nz(i['reporter_email'])})")
    print(f"  Assegnatario: {nz(i['assignee_name'])}    Creatore: {nz(i['creator_name'])}")


def render_purchase(i):
    """2. ACQUISTO — importi, fornitore, budget (pa.issue)."""
    forn = i.get('fornitore_selezionato') or i.get('fornitore') or ''
    line()
    print("  ▪ ACQUISTO / ECONOMICO")
    print(f"    Importo totale: {eur(i.get('importo_totale'))}    Fornitore: {nz(forn)}")
    print(f"    Tipo ordine: {nz(i.get('tipo_di_ordine'))}    "
          f"Tipo documento: {nz(i.get('tipo_documento'))}")
    print(f"    Budget riferimento: {nz(i.get('budget_di_riferimento'))}    "
          f"Corrente: {nz(i.get('budget_corrente'))}    Totale: {nz(i.get('budget_totale'))}")
    print(f"    Limite approv.: {nz(i.get('limite_approvazione'))}    "
          f"% approvazione: {nz(i.get('percentuale_approvazione'))}    "
          f"Ufficio acquisti: {nz(i.get('ufficio_acquisti'))}")
    print(f"    Inviato appr.: {dt(i.get('inviato_in_approvazione'))}    "
          f"Approvato: {dt(i.get('approvato'))}    "
          f"Ricorrente: {nz(i.get('ricorrente'))}")


def render_description(i):
    """3. DESCRIZIONE — pa.issue.description."""
    if i.get("description") and i["description"] != "—":
        line()
        print("  ▪ DESCRIZIONE")
        print("    " + nz(i["description"]).replace("\n", "\n    "))


def render_line_items(li):
    """4. RIGHE D'ORDINE — pa.line_item, raggruppate per tipo."""
    line()
    print(f"  ▪ RIGHE D'ORDINE ({len(li)})")
    if not li:
        print("    — (nessuna riga d'ordine per questo ticket) —")
        return
    cur = None
    for r in li:
        if r['grid'] != cur:                       # nuovo tipo → titolo
            cur = r['grid']
            print(f"    [{cur}]")
        extra = []
        if r.get('pagamento_name') and r['pagamento_name'] not in (None, '', '—'):
            extra.append(f"pag. {r['pagamento_name']}")
        if r.get('durata_name') and r['durata_name'] not in (None, '', '—'):
            extra.append(r['durata_name'])
        if r.get('data_pagamento') and r['data_pagamento'] not in (None, '', '—'):
            extra.append(f"scad. {r['data_pagamento']}")
        if r.get('vendita') and r['vendita'] not in (None, '', '—'):
            extra.append(f"vendita:{r['vendita']}")
        ex = ("  (" + " · ".join(extra) + ")") if extra else ""
        print(f"      {r['row_no']}. {nz(r['articolo_name'])} — {nz(r['vendor'])} "
              f"(PN {nz(r['part_number'])})  qtà {nz(r['quantita'])}  "
              f"importo {eur(r['importo'])}  tot {eur(r['prezzo_totale'])}{ex}")


def render_comments(cm):
    """5. COMMENTI — pa.comment."""
    line()
    print(f"  ▪ COMMENTI ({len(cm)})")
    for c in cm:
        print(f"    [{c['quando']}] {nz(c['author_name'])}:")
        print("      " + nz(c["actionbody"]).replace("\n", "\n      "))


def render_attachments(at):
    """6. ALLEGATI — pa.attachment (solo metadati; i file non sono nel DB)."""
    line()
    print(f"  ▪ ALLEGATI ({len(at)}) — solo metadati (i file non sono nel DB)")
    for a in at:
        print(f"    - {nz(a['filename'])}  ({nz(a['dim'])})  "
              f"{a['quando']}  {nz(a['author_name'])}")


def render_links(lk, issue_key):
    """7. COLLEGAMENTI — pa.issue_link."""
    line()
    print(f"  ▪ COLLEGAMENTI ({len(lk)})")
    for l in lk:
        other = l["destination_key"] if l["source_key"] == issue_key else l["source_key"]
        print(f"    - {nz(l['link_name'])}: {other}")


def render_history(ch, limit=12):
    """8. ULTIME MODIFICHE — pa.change_group ⨝ pa.change_item."""
    line()
    print(f"  ▪ ULTIME MODIFICHE (max {limit})")
    for c in ch:
        print(f"    [{c['quando']}] {nz(c['author_name'])} — {nz(c['field'])}: "
              f"{nz(c['old_string'])[:30]} → {nz(c['new_string'])[:30]}")
    line("═")


# --------------------------------------------------------------------------- #
#  Demo CLI
# --------------------------------------------------------------------------- #
def main():
    ap = argparse.ArgumentParser(description="Esempio illustrativo: scheda di un ordine PA.")
    ap.add_argument("issue_key", help="chiave dell'ordine, es. PA-1821")
    ap.add_argument("--schema", default=SCH, help=f"schema (default: {SCH})")
    ap.add_argument("--history", type=int, default=12, help="numero di modifiche mostrate")
    args = ap.parse_args()

    global SCH
    SCH = args.schema
    key = args.issue_key

    iss = fetch_issue(key)
    if not iss:
        print(f"Ordine {key} non trovato nello schema {SCH}.")
        return 1
    i = iss[0]

    render_header(i)
    render_purchase(i)
    render_description(i)
    render_line_items(fetch_line_items(key))
    render_comments(fetch_comments(key))
    render_attachments(fetch_attachments(key))
    render_links(fetch_links(key), key)
    render_history(fetch_history(key, limit=args.history), args.history)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
