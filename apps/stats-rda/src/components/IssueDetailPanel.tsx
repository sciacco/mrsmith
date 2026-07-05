import { useMemo } from 'react';
import type { IssueDetail, LineItem } from '../api/types';
import {
  formatEUR,
  formatFileSize,
  formatNumber,
  lineItemTotal,
  nz,
  shortDate,
  shortDateTime,
  statusBadgeClass,
} from '../lib/format';
import s from './IssueDetailPanel.module.css';

const GRID_ORDER = ['Articoli', 'Merci', 'Servizi', 'Leasing'];

interface Props {
  data: IssueDetail;
}

export function IssueDetailPanel({ data }: Props) {
  const { issue, purchase, description, line_items_by_grid, comments, attachments, links, history, history_total } = data;

  const grids = useMemo(() => {
    return GRID_ORDER.filter((g) => (line_items_by_grid[g]?.length ?? 0) > 0);
  }, [line_items_by_grid]);

  const totalLineItems = useMemo(
    () => Object.values(line_items_by_grid).reduce((sum, items) => sum + items.length, 0),
    [line_items_by_grid],
  );

  return (
    <div className={s.panel}>
      {/* 1. Header */}
      <header className={s.header}>
        <div className={s.headerTop}>
          <span className={s.issueKey}>{issue.issue_key}</span>
          <span className={s.summary}>{issue.summary}</span>
        </div>
        <div className={s.metaRow}>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Tipo:</span> {nz(issue.issue_type)}
          </span>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Stato:</span>{' '}
            <span className={statusBadgeClass(issue.status)}>{nz(issue.status)}</span>
          </span>
          {issue.stato && issue.stato !== issue.status && (
            <span className={s.metaItem}>
              <span className={s.metaLabel}>Stato richiesta:</span> {issue.stato}
            </span>
          )}
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Priorità:</span> {nz(issue.priority)}
          </span>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Risoluzione:</span> {nz(issue.resolution)}
          </span>
        </div>
        <div className={s.metaRow}>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Numero ordine:</span>{' '}
            <span className={s.fieldValue + ' ' + s.mono}>{nz(issue.numero_ordine)}</span>
          </span>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Valuta:</span> {nz(issue.valuta)}
          </span>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Creato:</span> {shortDateTime(issue.created)}
          </span>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Aggiornato:</span> {shortDateTime(issue.updated)}
          </span>
          {issue.resolution_date && (
            <span className={s.metaItem}>
              <span className={s.metaLabel}>Risolto:</span> {shortDateTime(issue.resolution_date)}
            </span>
          )}
          {issue.due_date && (
            <span className={s.metaItem}>
              <span className={s.metaLabel}>Scadenza:</span> {shortDate(issue.due_date)}
            </span>
          )}
        </div>
        <div className={s.metaRow}>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Richiedente:</span> {nz(issue.reporter_name)}
            {issue.reporter_email ? ` (${issue.reporter_email})` : ''}
          </span>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Assegnatario:</span> {nz(issue.assignee_name)}
          </span>
          <span className={s.metaItem}>
            <span className={s.metaLabel}>Creatore:</span> {nz(issue.creator_name)}
          </span>
        </div>
      </header>

      {/* 2. Acquisto / economico */}
      <section className={s.section}>
        <h2 className={s.sectionTitle}>Acquisto</h2>
        <div className={s.grid}>
          <div className={s.field}>
            <span className={s.fieldLabel}>Importo totale</span>
            <span className={`${s.fieldValue} ${s.num}`}>{formatEUR(purchase.importo_totale)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Fornitore</span>
            <span className={s.fieldValue}>{nz(purchase.fornitore_selezionato)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Tipo ordine</span>
            <span className={s.fieldValue}>{nz(purchase.tipo_di_ordine)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Tipo documento</span>
            <span className={s.fieldValue}>{nz(purchase.tipo_documento)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Budget di riferimento</span>
            <span className={s.fieldValue}>{nz(purchase.budget_di_riferimento)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Budget corrente</span>
            <span className={`${s.fieldValue} ${s.num}`}>{formatEUR(purchase.budget_corrente)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Budget totale</span>
            <span className={`${s.fieldValue} ${s.num}`}>{formatEUR(purchase.budget_totale)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Limite approvazione</span>
            <span className={`${s.fieldValue} ${s.num}`}>{formatEUR(purchase.limite_approvazione)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>% approvazione</span>
            <span className={`${s.fieldValue} ${s.num}`}>
              {purchase.percentuale_approvazione != null
                ? formatNumber(purchase.percentuale_approvazione) + '%'
                : '—'}
            </span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Inviato in approvazione</span>
            <span className={s.fieldValue}>{shortDateTime(purchase.inviato_in_approvazione)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Approvato</span>
            <span className={s.fieldValue}>{shortDateTime(purchase.approvato)}</span>
          </div>
          <div className={s.field}>
            <span className={s.fieldLabel}>Ricorrente</span>
            <span className={s.fieldValue}>{nz(purchase.ricorrente)}</span>
          </div>
        </div>
      </section>

      {/* 3. Descrizione */}
      {description && (
        <section className={s.section}>
          <h2 className={s.sectionTitle}>Descrizione</h2>
          <p className={s.description}>{description}</p>
        </section>
      )}

      {/* 4. Righe d'ordine */}
      <section className={s.section}>
        <h2 className={s.sectionTitle}>
          Righe d'ordine <span className={s.sectionCount}>({totalLineItems})</span>
        </h2>
        {totalLineItems === 0 ? (
          <p className={s.description}>Nessuna riga d'ordine per questo ordine.</p>
        ) : (
          grids.map((grid) => (
            <div key={grid} className={s.gridGroup}>
              <h3 className={s.gridGroupTitle}>{grid}</h3>
              {(line_items_by_grid[grid] ?? []).map((item: LineItem) => {
                const total = lineItemTotal(item);
                return (
                  <div key={`${item.grid}-${item.row_no}`} className={s.lineItem}>
                    <span className={s.lineNo}>{item.row_no}.</span>
                    <span className={s.lineMain}>
                      <span className={s.lineTitle}>{nz(item.articolo_name) === '—' ? nz(item.descrizione) : nz(item.articolo_name)}</span>
                      {(item.vendor || item.part_number) && (
                        <span className={s.lineVendor}>
                          {' '}
                          — {nz(item.vendor)} {item.part_number ? `(PN ${item.part_number})` : ''}
                        </span>
                      )}
                      {item.descrizione && item.articolo_name && (
                        <span className={s.lineVendor}> · {item.descrizione}</span>
                      )}
                      {(item.pagamento_name || item.durata_name || item.data_pagamento) && (
                        <span className={s.lineVendor}>
                          {' '}
                          · {[item.pagamento_name, item.durata_name, item.data_pagamento ? `scad. ${shortDate(item.data_pagamento)}` : null]
                            .filter(Boolean)
                            .join(' · ')}
                        </span>
                      )}
                    </span>
                    <span className={s.lineQty}>
                      {item.quantita != null ? `qtà ${formatNumber(item.quantita)}` : ''}
                    </span>
                    <span className={s.lineAmount}>
                      {item.importo != null ? formatEUR(item.importo) : '—'}
                      {total != null && item.prezzo_totale != null && (
                        <span className={s.lineVendor}> · tot {formatEUR(total)}</span>
                      )}
                    </span>
                  </div>
                );
              })}
            </div>
          ))
        )}
      </section>

      {/* 5. Commenti */}
      <section className={s.section}>
        <h2 className={s.sectionTitle}>
          Commenti <span className={s.sectionCount}>({comments.length})</span>
        </h2>
        {comments.length === 0 ? (
          <p className={s.description}>Nessun commento.</p>
        ) : (
          comments.map((c, idx) => (
            <div key={idx} className={s.comment}>
              <div className={s.commentMeta}>
                <span className={s.commentAuthor}>{nz(c.author_name)}</span> · {shortDateTime(c.created)}
              </div>
              {c.body && <div className={s.commentBody}>{c.body}</div>}
            </div>
          ))
        )}
      </section>

      {/* 6. Allegati */}
      <section className={s.section}>
        <h2 className={s.sectionTitle}>
          Allegati <span className={s.sectionCount}>({attachments.length})</span>
        </h2>
        <p className={s.disclosure}>Solo metadati: i file non sono inclusi nell'archivio.</p>
        {attachments.length === 0 ? (
          <p className={s.description}>Nessun allegato.</p>
        ) : (
          attachments.map((a, idx) => (
            <div key={idx} className={s.attachment}>
              <span className={s.attachmentName}>{a.filename}</span>
              <span className={s.attachmentMeta}>
                {formatFileSize(a.filesize)} · {shortDate(a.created)} · {nz(a.author_name)}
              </span>
            </div>
          ))
        )}
      </section>

      {/* 7. Collegamenti */}
      <section className={s.section}>
        <h2 className={s.sectionTitle}>
          Collegamenti <span className={s.sectionCount}>({links.length})</span>
        </h2>
        {links.length === 0 ? (
          <p className={s.description}>Nessun collegamento.</p>
        ) : (
          links.map((l, idx) => {
            const other = l.destination_key;
            return (
              <div key={idx} className={s.link}>
                <span className={s.linkType}>{nz(l.link_name)}:</span>
                <span className={s.linkKey}>{other}</span>
              </div>
            );
          })
        )}
      </section>

      {/* 8. Storico modifiche */}
      <section className={s.section}>
        <h2 className={s.sectionTitle}>
          Storico modifiche <span className={s.sectionCount}>({history.length})</span>
        </h2>
        {history.length === 0 ? (
          <p className={s.description}>Nessuna modifica registrata.</p>
        ) : (
          <>
            {history.map((h, idx) => (
              <div key={idx} className={s.historyEntry}>
                <span className={s.historyWhen}>{shortDateTime(h.created)}</span>
                <span className={s.historyBody}>
                  <span className={s.historyField}>{nz(h.field)}</span> · {nz(h.author_name)}
                  <div className={s.historyChange}>
                    {nz(h.old_string)} → {nz(h.new_string)}
                  </div>
                </span>
              </div>
            ))}
            {history_total > history.length && (
              <p className={s.historyTruncation}>
                Mostrate le ultime {history.length} modifiche di {history_total} totali.
              </p>
            )}
          </>
        )}
      </section>
    </div>
  );
}
