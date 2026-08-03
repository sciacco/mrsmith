import { useEffect, useMemo, useRef, useState } from 'react';
import { formatInstant, formatLocalDate } from '@mrsmith/format';
import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, SearchInput, Skeleton, TableToolbar, useToast } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import { downloadRdaDdtZip, useRdaDdtAttachments } from '../api/queries';
import type { RDADdtAttachmentRow } from '../types';
import shared from './shared.module.css';
import styles from './DdtPurchaseOrderPage.module.css';

interface PoGroup {
  poId: number;
  poCode: string | null;
  attachments: RDADdtAttachmentRow[];
  requesterFirstName: string | null;
  requesterLastName: string | null;
  requesterEmail: string | null;
  project: string;
  subject: string | null;
  costCenter: string | null;
  latestCreated: string;
}

function toISODate(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// First and last civil day of the previous month, computed in local calendar
// time. Never derived via toISOString() so timezone shifts cannot move the day.
function previousMonthRange(): { from: string; to: string } {
  const now = new Date();
  const firstDay = new Date(now.getFullYear(), now.getMonth() - 1, 1);
  const lastDay = new Date(now.getFullYear(), now.getMonth(), 0); // day 0 = last day of previous month
  return { from: toISODate(firstDay), to: toISODate(lastDay) };
}

function getDownloadErrorMessage(error: unknown): string {
  if (error instanceof ApiError && error.body && typeof error.body === 'object') {
    const code = (error.body as { error?: unknown }).error;
    if (typeof code === 'string') {
      switch (code) {
        case 'no_matching_ddt_documents':
          return 'Nessun documento DDT corrisponde ai PO selezionati. Cerca di nuovo e riprova.';
        case 'po_ids_required':
          return 'Seleziona almeno un PO prima di scaricare.';
        case 'invalid_po_ids':
          return 'La selezione dei PO non è valida. Riprova.';
        case 'arak_client_not_configured':
          return 'Servizio non configurato. Riprova più tardi.';
        default:
          return 'Errore durante il download dello ZIP.';
      }
    }
  }
  if (error instanceof ApiError && error.status === 503) {
    return 'Servizio temporaneamente non disponibile. Riprova più tardi.';
  }
  return 'Errore durante il download dello ZIP. Riprova.';
}

export default function DdtPurchaseOrderPage() {
  const initial = useMemo(previousMonthRange, []);
  const [from, setFrom] = useState(initial.from);
  const [to, setTo] = useState(initial.to);
  const [search, setSearch] = useState('');
  const [run, setRun] = useState<{ from: string; to: string; runId: number } | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState<string | null>(null);

  const api = useApiClient();
  const { toast } = useToast();

  const q = useRdaDdtAttachments(run?.from ?? '', run?.to ?? '', run?.runId ?? null);

  const rangeInvalid = from !== '' && to !== '' && from > to;
  const canSearch = from !== '' && to !== '' && !rangeInvalid;

  const groups = useMemo<PoGroup[]>(() => {
    const rows = q.data ?? [];
    const byPo = new Map<number, PoGroup>();
    for (const row of rows) {
      let group = byPo.get(row.po_id);
      if (!group) {
        group = {
          poId: row.po_id,
          poCode: row.po_code,
          attachments: [],
          requesterFirstName: row.requester_first_name,
          requesterLastName: row.requester_last_name,
          requesterEmail: row.requester_email,
          project: row.project,
          subject: row.subject,
          costCenter: row.cost_center,
          latestCreated: row.created,
        };
        byPo.set(row.po_id, group);
      }
      group.attachments.push(row);
      if (row.created > group.latestCreated) group.latestCreated = row.created;
    }
    return Array.from(byPo.values()).sort((a, b) => b.latestCreated.localeCompare(a.latestCreated));
  }, [q.data]);

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return groups;
    return groups.filter((g) => {
      const haystack = [
        g.poCode ?? '',
        g.project,
        g.subject ?? '',
        g.costCenter ?? '',
        g.requesterFirstName ?? '',
        g.requesterLastName ?? '',
        g.requesterEmail ?? '',
        ...g.attachments.map((a) => a.file_name),
      ]
        .join(' ')
        .toLowerCase();
      return haystack.includes(needle);
    });
  }, [groups, search]);

  const allVisibleSelected = filtered.length > 0 && filtered.every((g) => selected.has(g.poId));
  const someVisibleSelected = filtered.some((g) => selected.has(g.poId));
  const selectedCount = selected.size;
  // Scarica selezionati acts only on POs visible after the local filter, so
  // selections hidden by the current filter are excluded from the payload.
  const visibleSelected = useMemo(() => filtered.filter((g) => selected.has(g.poId)), [filtered, selected]);
  const visibleSelectedCount = visibleSelected.length;
  const hiddenSelectedCount = selectedCount - visibleSelectedCount;

  const headerCheckboxRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (headerCheckboxRef.current) {
      headerCheckboxRef.current.indeterminate = someVisibleSelected && !allVisibleSelected;
    }
  }, [allVisibleSelected, someVisibleSelected]);

  function handleSearch() {
    if (!canSearch) return;
    setSelected(new Set());
    setDownloadError(null);
    setRun({ from, to, runId: Date.now() });
  }

  function togglePo(poId: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(poId)) {
        next.delete(poId);
      } else {
        next.add(poId);
      }
      return next;
    });
  }

  function toggleAllVisible() {
    setSelected((prev) => {
      const next = new Set(prev);
      for (const g of filtered) {
        if (allVisibleSelected) {
          next.delete(g.poId);
        } else {
          next.add(g.poId);
        }
      }
      return next;
    });
  }

  function triggerDownload(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  async function handleDownload() {
    if (!run || visibleSelectedCount === 0 || downloading) return;
    setDownloading(true);
    setDownloadError(null);
    try {
      const blob = await downloadRdaDdtZip(api, run.from, run.to, visibleSelected.map((g) => g.poId));
      triggerDownload(blob, `ddt-purchase-order_${run.from}_${run.to}.zip`);
      toast('Download dello ZIP avviato', 'success');
    } catch (error) {
      setDownloadError(getDownloadErrorMessage(error));
    } finally {
      setDownloading(false);
    }
  }

  const hasData = q.data !== undefined;

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>DDT Purchase Order</h1>
      <p className={shared.info}>
        Elenca i DDT dei Purchase Order (RDA) nel periodo selezionato e scarica un unico ZIP con i
        documenti dei PO scelti.
      </p>

      <TableToolbar
        className={styles.toolbar}
        activeFilterCount={search.trim() ? 1 : 0}
        filters={
          <div className={styles.search}>
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder="Filtra per codice PO, file, richiedente, progetto…"
              ariaLabel="Filtra i PO ricevuti"
            />
          </div>
        }
      >
        <div className={styles.dateFields}>
          <div className={styles.dateField}>
            <label className={styles.dateLabel} htmlFor="ddt-from">
              Dal
            </label>
            <input
              id="ddt-from"
              className={`${styles.dateInput} ${rangeInvalid ? styles.dateInputInvalid : ''}`}
              type="date"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              aria-invalid={rangeInvalid || undefined}
              aria-describedby={rangeInvalid ? 'ddt-range-error' : undefined}
            />
          </div>
          <div className={styles.dateField}>
            <label className={styles.dateLabel} htmlFor="ddt-to">
              Al
            </label>
            <input
              id="ddt-to"
              className={`${styles.dateInput} ${rangeInvalid ? styles.dateInputInvalid : ''}`}
              type="date"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              aria-invalid={rangeInvalid || undefined}
              aria-describedby={rangeInvalid ? 'ddt-range-error' : undefined}
            />
          </div>
        </div>
        <Button onClick={handleSearch} disabled={!canSearch || downloading}>
          Cerca
        </Button>
        {hasData && (
          <div className={styles.actions}>
            <Button
              variant="secondary"
              onClick={toggleAllVisible}
              disabled={filtered.length === 0 || downloading}
            >
              Seleziona tutti
            </Button>
            <Button
              variant="secondary"
              onClick={handleDownload}
              disabled={visibleSelectedCount === 0 || downloading}
              loading={downloading}
              leftIcon={<Icon name="download" size={16} />}
            >
              {downloading ? 'Scaricamento…' : 'Scarica selezionati'}
            </Button>
          </div>
        )}
      </TableToolbar>

      {rangeInvalid && (
        <p id="ddt-range-error" className={styles.dateError} role="alert">
          La data di inizio deve precedere la data di fine.
        </p>
      )}

      {downloadError && (
        <div className={styles.errorBanner} role="alert">
          <span>{downloadError}</span>
        </div>
      )}

      {!run && !q.isLoading && (
        <div className={styles.emptyState}>
          <div className={styles.emptyIcon}>
            <Icon name="package" size={32} />
          </div>
          <div className={styles.emptyTitle}>Nessuna ricerca effettuata</div>
          <div className={styles.emptyDesc}>
            Imposta l'intervallo di date (predefinito: mese precedente) e premi Cerca per elencare i
            DDT dei Purchase Order.
          </div>
        </div>
      )}

      {q.isLoading && <Skeleton rows={8} />}

      {q.isError && (
        <div className={styles.errorBanner} role="alert">
          <span>Errore nel caricamento dei DDT.</span>
          <Button variant="secondary" size="sm" onClick={() => q.refetch()}>
            Riprova
          </Button>
        </div>
      )}

      {q.data && q.data.length === 0 && (
        <div className={styles.emptyState}>
          <div className={styles.emptyIcon}>
            <Icon name="file-warning" size={32} />
          </div>
          <div className={styles.emptyTitle}>Nessun DDT nel periodo selezionato</div>
          <div className={styles.emptyDesc}>
            Nessun documento DDT tra {formatLocalDate(run?.from) ?? run?.from} e{' '}
            {formatLocalDate(run?.to) ?? run?.to}. Prova ad allargare l'intervallo di date.
          </div>
        </div>
      )}

      {q.data && q.data.length > 0 && filtered.length === 0 && (
        <div className={styles.emptyState}>
          <div className={styles.emptyIcon}>
            <Icon name="search" size={32} />
          </div>
          <div className={styles.emptyTitle}>Nessun PO corrisponde al filtro</div>
          <div className={styles.emptyDesc}>
            Modifica il testo di ricerca o svuotalo per vedere tutti i PO ricevuti.
          </div>
        </div>
      )}

      {hasData && q.data && q.data.length > 0 && filtered.length > 0 && (
        <>
          <div className={styles.selectionInfo}>
            {visibleSelectedCount === 0
              ? hiddenSelectedCount > 0
                ? 'I PO selezionati sono nascosti dal filtro. Svuota il filtro o seleziona PO visibili.'
                : 'Nessun PO selezionato. Seleziona almeno un PO per scaricare lo ZIP.'
              : `${visibleSelectedCount} PO selezionat${visibleSelectedCount === 1 ? 'o' : 'i'} su ${filtered.length} visibili${hiddenSelectedCount > 0 ? ` (${hiddenSelectedCount} nascost${hiddenSelectedCount === 1 ? 'o' : 'i'} dal filtro)` : ''}.`}
          </div>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={styles.checkboxCell}>
                    <input
                      ref={headerCheckboxRef}
                      className={styles.checkbox}
                      type="checkbox"
                      checked={allVisibleSelected}
                      onChange={toggleAllVisible}
                      aria-label="Seleziona tutti i PO visibili"
                    />
                  </th>
                  <th>PO</th>
                  <th>Documenti</th>
                  <th>Creato</th>
                  <th>Richiedente</th>
                  <th>Progetto</th>
                  <th>Oggetto</th>
                  <th>Centro di costo</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((g) => (
                  <tr key={g.poId}>
                    <td className={styles.checkboxCell}>
                      <input
                        className={styles.checkbox}
                        type="checkbox"
                        checked={selected.has(g.poId)}
                        onChange={() => togglePo(g.poId)}
                        aria-label={`Seleziona PO ${g.poCode ?? g.poId}`}
                      />
                    </td>
                    <td>
                      <span className={styles.monoCode}>{g.poCode ?? g.poId}</span>
                    </td>
                    <td className={styles.documentsCell}>
                      <div className={styles.fileList}>
                        {g.attachments.map((a) => (
                          <div key={a.attachment_id} className={styles.fileName}>
                            {a.file_name}
                          </div>
                        ))}
                      </div>
                      <div className={styles.fileCount}>
                        {g.attachments.length} {g.attachments.length === 1 ? 'documento' : 'documenti'}
                      </div>
                    </td>
                    <td className={styles.created}>{formatInstant(g.latestCreated) ?? '—'}</td>
                    <td>
                      <div className={styles.requesterName}>
                        {[g.requesterFirstName, g.requesterLastName].filter(Boolean).join(' ') || '—'}
                      </div>
                      {g.requesterEmail && <div className={styles.requesterEmail}>{g.requesterEmail}</div>}
                    </td>
                    <td>{g.project || '—'}</td>
                    <td>{g.subject ?? '—'}</td>
                    <td>{g.costCenter ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
