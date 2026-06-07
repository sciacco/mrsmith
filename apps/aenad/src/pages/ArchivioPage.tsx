import { Button, Icon, SingleSelect, Skeleton } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { useArchiveDocuments, useDocumentTypes } from '../api/queries';
import type { AenadDocument } from '../api/types';
import styles from './PreventiviPage.module.css';

const DEFAULT_TIPO_DOC = 'Q';
const DEFAULT_DAYS = 100;
const MAX_DAYS = 380;
const PAGE_SIZE = 50;

const moneyFormatter = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});

function formatISODate(date: Date) {
  return date.toISOString().slice(0, 10);
}

function defaultDateRange() {
  const to = new Date();
  const from = new Date(to);
  from.setDate(from.getDate() - (DEFAULT_DAYS - 1));
  return {
    from: formatISODate(from),
    to: formatISODate(to),
  };
}

function inclusiveDays(dateFrom: string, dateTo: string) {
  const from = new Date(`${dateFrom}T00:00:00`);
  const to = new Date(`${dateTo}T00:00:00`);
  if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime())) return null;
  return Math.floor((to.getTime() - from.getTime()) / 86_400_000) + 1;
}

function dateError(dateFrom: string, dateTo: string) {
  if (!dateFrom || !dateTo) return 'Seleziona entrambe le date.';
  const days = inclusiveDays(dateFrom, dateTo);
  if (days === null) return 'Controlla le date selezionate.';
  if (days <= 0) return 'La data iniziale deve precedere la data finale.';
  if (days > MAX_DAYS) return `Il periodo massimo consultabile e di ${MAX_DAYS} giorni.`;
  return null;
}

function formatDate(value: string | null) {
  if (!value) return '-';
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (match) {
    const [, year, month, day] = match;
    return `${day}/${month}/${year}`;
  }
  const date = new Date(value);
  if (!Number.isNaN(date.getTime())) {
    return date.toLocaleDateString('it-IT', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
    });
  }
  return value;
}


function formatMoney(value: number | null) {
  if (value == null) return '-';
  return moneyFormatter.format(value);
}

export function ArchivioPage() {
  const range = useMemo(defaultDateRange, []);
  const [tipoDoc, setTipoDoc] = useState(DEFAULT_TIPO_DOC);
  const [dateFrom, setDateFrom] = useState(range.from);
  const [dateTo, setDateTo] = useState(range.to);
  const [page, setPage] = useState(1);

  const documentTypes = useDocumentTypes();
  const typeOptions = useMemo(
    () => (documentTypes.data ?? []).map((item) => ({ value: item.tipoDoc, label: `${item.tipoDoc} - ${item.label}` })),
    [documentTypes.data],
  );
  const selectedTypeExists = typeOptions.length === 0 || typeOptions.some((item) => item.value === tipoDoc);
  const currentDateError = dateError(dateFrom, dateTo);

  useEffect(() => {
    if (documentTypes.data && documentTypes.data.length > 0 && !documentTypes.data.some((item) => item.tipoDoc === tipoDoc)) {
      const firstType = documentTypes.data[0];
      if (!firstType) return;
      setTipoDoc(firstType.tipoDoc);
      setPage(1);
    }
  }, [documentTypes.data, tipoDoc]);

  const documents = useArchiveDocuments(
    {
      tipoDoc,
      dateFrom,
      dateTo,
      page,
      pageSize: PAGE_SIZE,
    },
    Boolean(tipoDoc && selectedTypeExists && !currentDateError && !documentTypes.isLoading && !documentTypes.error),
  );

  const rows = documents.data?.items ?? [];
  const total = documents.data?.total ?? 0;
  const pageSize = documents.data?.pageSize ?? PAGE_SIZE;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const fromRow = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const toRow = Math.min(total, page * pageSize);
  const periodDays = inclusiveDays(dateFrom, dateTo);

  function updateTipoDoc(value: string | null) {
    setTipoDoc(value ?? DEFAULT_TIPO_DOC);
    setPage(1);
  }

  function updateDateFrom(value: string) {
    setDateFrom(value);
    setPage(1);
  }

  function updateDateTo(value: string) {
    setDateTo(value);
    setPage(1);
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Vendite</p>
          <h1>Archivio</h1>
        </div>
      </header>

      <section className={styles.filterPanel} aria-label="Filtri archivio">
        <div className={styles.filterGrid}>
          <div className={styles.field}>
            <label>Tipo documento</label>
            <SingleSelect
              options={typeOptions}
              selected={tipoDoc}
              onChange={updateTipoDoc}
              placeholder={documentTypes.isLoading ? 'Caricamento...' : 'Seleziona tipo'}
              disabled={documentTypes.isLoading || Boolean(documentTypes.error)}
            />
          </div>
          <div className={styles.field}>
            <label>Data da</label>
            <input type="date" value={dateFrom} onChange={(event) => updateDateFrom(event.target.value)} />
          </div>
          <div className={styles.field}>
            <label>Data a</label>
            <input type="date" value={dateTo} onChange={(event) => updateDateTo(event.target.value)} />
          </div>
        </div>
        <div className={styles.filterFooter}>
          <div className={styles.chipRow}>
            <span className={styles.defaultChip}>Tipo {tipoDoc}</span>
            <span className={currentDateError ? styles.warningChip : styles.defaultChip}>
              {periodDays && periodDays > 0 ? `${periodDays} giorni` : 'Periodo da completare'}
            </span>
          </div>
          {currentDateError ? <p className={styles.validationText}>{currentDateError}</p> : null}
        </div>
      </section>

      <section className={styles.resultsPanel} aria-label="Documenti archivio">
        <div className={styles.resultsHeader}>
          <div>
            <h2>Documenti</h2>
            <p>
              {documents.isLoading || documents.isFetching
                ? 'Caricamento documenti...'
                : `${fromRow}-${toRow} di ${total}`}
            </p>
          </div>
        </div>

        {documentTypes.error ? (
          <ViewState
            icon="file-warning"
            title="Tipi documento non disponibili"
            message="Non e stato possibile caricare i tipi documento."
          />
        ) : currentDateError ? (
          <ViewState icon="calendar" title="Periodo non valido" message={currentDateError} />
        ) : documents.isLoading ? (
          <div className={styles.loadingPanel}>
            <Skeleton rows={8} />
          </div>
        ) : documents.error ? (
          <ViewState
            icon="file-warning"
            title="Archivio non disponibile"
            message="Non e stato possibile caricare i documenti."
          />
        ) : rows.length === 0 ? (
          <ViewState icon="search" title="Nessun documento trovato" message="Modifica tipo documento o periodo." />
        ) : (
          <>
            <DocumentTable rows={rows} />
            <div className={styles.pagination}>
              <span>
                {fromRow}-{toRow} di {total}
              </span>
              <div className={styles.pageActions}>
                <Button size="sm" variant="secondary" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>
                  Precedente
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={page >= totalPages}
                  onClick={() => setPage((current) => current + 1)}
                >
                  Successiva
                </Button>
              </div>
            </div>
          </>
        )}
      </section>
    </main>
  );
}

function DocumentTable({ rows }: { rows: AenadDocument[] }) {
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Num doc.</th>
            <th>Data doc.</th>
            <th>Cliente</th>
            <th>Descrizione</th>
            <th className={styles.numCol}>Netto</th>
            <th className={styles.numCol}>Totale</th>
            <th className={styles.numCol}>Acquisto</th>
            <th className={styles.numCol}>Guadagno</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={row.IDDoc} style={{ animationDelay: `${Math.min(index, 8) * 25}ms` }}>
              <td className={styles.monoCell}>{row.NumDoc ?? '-'}</td>
              <td>{formatDate(row.DataDoc)}</td>
              <td>{row.Anagr_Nome ?? '-'}</td>
              <td className={styles.descriptionCell}>{row.DescDoc ?? '-'}</td>
              <td className={styles.numCol}>{formatMoney(row.TotNetto)}</td>
              <td className={styles.numCol}>{formatMoney(row.TotDoc)}</td>
              <td className={styles.numCol}>{formatMoney(row.TotPrezzoAcquisto)}</td>
              <td className={styles.numCol}>{formatMoney(row.TotGuadagno)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ViewState({
  icon,
  title,
  message,
}: {
  icon: 'calendar' | 'file-warning' | 'search';
  title: string;
  message: string;
}) {
  return (
    <div className={styles.emptyPanel}>
      <div className={styles.emptyIcon} aria-hidden="true">
        <Icon name={icon} size={24} />
      </div>
      <div className={styles.emptyText}>
        <h2>{title}</h2>
        <p>{message}</p>
      </div>
    </div>
  );
}
