import { useCallback, useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Icon, SingleSelect, Skeleton } from '@mrsmith/ui';
import { downloadRiepilogoPaExcel, useRiepilogoPa } from '../api/queries';
import type { PeriodPreset, RiepilogoBudget } from '../api/types';
import { useApiClient } from '../api/client';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import { formatEUR, nz } from '../lib/format';
import s from './RiepilogoPaPage.module.css';

const DEFAULT_PERIOD: PeriodPreset = 'previous_month';

const PERIOD_OPTIONS: Array<{ value: PeriodPreset; label: string }> = [
  { value: 'this_month', label: 'Mese corrente' },
  { value: 'previous_month', label: 'Mese precedente' },
  { value: 'this_quarter', label: 'Trimestre corrente' },
  { value: 'previous_quarter', label: 'Trimestre precedente' },
  { value: 'current_year', label: 'Anno corrente' },
  { value: 'previous_year', label: 'Anno precedente' },
];

const periodValues = new Set<PeriodPreset>(PERIOD_OPTIONS.map((option) => option.value));

function isPeriodPreset(value: string | null): value is PeriodPreset {
  return Boolean(value && periodValues.has(value as PeriodPreset));
}

function formatPeriodDate(value: string | null | undefined): string {
  if (!value) return '—';
  const [year, month, day] = value.slice(0, 10).split('-');
  if (!year || !month || !day) return value;
  return `${day}/${month}/${year}`;
}

function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value.slice(0, 16).replace('T', ' ');
  return new Intl.DateTimeFormat('it-IT', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}

function formatPercent(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  }).format(value);
}

function filenameFor(period: PeriodPreset, from?: string, to?: string): string {
  if (from && to) return `riepilogo-pa-jira_${period}_${from}_${to}.xlsx`;
  return `riepilogo-pa-jira_${period}.xlsx`;
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiError && error.status === 401) return 'Accesso richiesto.';
  if (error instanceof ApiError && error.status === 403) return 'Accesso non consentito.';
  return 'Non è stato possibile caricare il riepilogo PA.';
}

export function RiepilogoPaPage() {
  const [params, setParams] = useSearchParams();
  const api = useApiClient();
  const periodParam = params.get('period');
  const period = isPeriodPreset(periodParam) ? periodParam : DEFAULT_PERIOD;
  const [selectedBudget, setSelectedBudget] = useState<string | null>(null);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  const riepilogoQ = useRiepilogoPa(period);
  const data = riepilogoQ.data;

  useEffect(() => {
    if (!isPeriodPreset(periodParam)) {
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('period', DEFAULT_PERIOD);
        return next;
      }, { replace: true });
    }
  }, [periodParam, setParams]);

  useEffect(() => {
    setSelectedBudget(null);
  }, [period]);

  const handlePeriodChange = useCallback(
    (value: PeriodPreset | null) => {
      const nextPeriod = value ?? DEFAULT_PERIOD;
      setParams((prev) => {
        const next = new URLSearchParams(prev);
        next.set('period', nextPeriod);
        return next;
      });
    },
    [setParams],
  );

  const handleExport = useCallback(async () => {
    setIsExporting(true);
    setExportError(null);
    try {
      await downloadRiepilogoPaExcel(
        api,
        period,
        filenameFor(period, data?.period.from, data?.period.to),
      );
    } catch {
      setExportError('Non è stato possibile generare il file Excel.');
    } finally {
      setIsExporting(false);
    }
  }, [api, data?.period.from, data?.period.to, period]);

  const maxAmount = useMemo(
    () => Math.max(0, ...(data?.budgets.map((budget) => budget.amount) ?? [])),
    [data?.budgets],
  );

  const visibleDetails = useMemo(() => {
    const details = data?.details ?? [];
    if (!selectedBudget) return details;
    return details.filter((detail) => detail.budget_di_riferimento === selectedBudget);
  }, [data?.details, selectedBudget]);

  const dateRange = data?.period
    ? `${formatPeriodDate(data.period.from)} – ${formatPeriodDate(data.period.to)} escluso`
    : 'Periodo in caricamento…';

  const isEmpty = Boolean(data && data.details.length === 0);
  const isServiceUnavailable = riepilogoQ.error instanceof ApiError && riepilogoQ.error.status === 503;

  return (
    <main className={`statsRdaPage ${s.page}`}>
      <header className={`pageHeader ${s.header}`}>
        <div className="pageHeaderCopy">
          <span className="pageHeaderEyebrow">
            <Icon name="bar-chart-2" size={14} /> Riepilogo PA
          </span>
          <h1>Riepilogo PA Jira</h1>
          <p>Totali degli ordini di acquisto PA per budget nel periodo selezionato.</p>
        </div>
      </header>

      <section className={`surface ${s.toolbar}`} aria-label="Filtri riepilogo PA">
        <div className={s.toolbarField}>
          <label>Periodo</label>
          <SingleSelect
            options={PERIOD_OPTIONS}
            selected={period}
            onChange={handlePeriodChange}
            placeholder="Seleziona periodo"
          />
        </div>
        <div className={s.rangeText} aria-live="polite">
          <Icon name="calendar" size={16} />
          <span>{dateRange}</span>
        </div>
        <button type="button" className={s.exportButton} onClick={handleExport} disabled={isExporting}>
          <Icon name="download" size={16} />
          {isExporting ? 'Preparazione file Excel…' : 'Scarica Excel'}
        </button>
        {exportError && <p className={s.exportError}>{exportError}</p>}
      </section>

      {riepilogoQ.isLoading && (
        <section className="surface stateCard" aria-live="polite">
          <p className="stateTitle">Caricamento riepilogo PA…</p>
          <Skeleton rows={5} />
        </section>
      )}

      {!riepilogoQ.isLoading && isServiceUnavailable && <ServiceUnavailable service="Stats RDA" />}

      {!riepilogoQ.isLoading && riepilogoQ.error && !isServiceUnavailable && (
        <section className={s.errorState} role="alert">
          <div className={s.stateIcon}><Icon name="triangle-alert" size={30} /></div>
          <p className="stateTitle">{errorMessage(riepilogoQ.error)}</p>
          <p className="muted">Riprova più tardi.</p>
        </section>
      )}

      {!riepilogoQ.isLoading && !riepilogoQ.error && isEmpty && (
        <section className={s.emptyState}>
          <div className={s.stateIcon}><Icon name="file-text" size={30} /></div>
          <p className="emptyStateTitle">Nessun ordine di acquisto trovato per il periodo selezionato.</p>
          <p className="emptyStateDesc">Seleziona un periodo diverso per consultare altri ordini PA.</p>
        </section>
      )}

      {data && !isEmpty && (
        <>
          <section className={s.summaryGrid} aria-label="Indicatori riepilogo PA">
            <article className={s.metricCard}>
              <span>Totale periodo</span>
              <strong>{formatEUR(data.totals.amount)}</strong>
            </article>
            <article className={s.metricCard}>
              <span>Ordini inclusi</span>
              <strong>{data.totals.order_count.toLocaleString('it-IT')}</strong>
            </article>
            <article className={s.metricCard}>
              <span>Budget trovati</span>
              <strong>{data.totals.budget_count.toLocaleString('it-IT')}</strong>
            </article>
          </section>

          <section className={`surface ${s.chartCard}`}>
            <div className={s.cardHeader}>
              <div>
                <h2>Totali per budget</h2>
                <p>Seleziona una barra per filtrare i dettagli degli ordini.</p>
              </div>
              {selectedBudget && (
                <button type="button" className={s.clearButton} onClick={() => setSelectedBudget(null)}>
                  Mostra tutti
                </button>
              )}
            </div>
            <div className={s.barList} role="list" aria-label="Budget ordinati per importo">
              {data.budgets.map((budget) => (
                <BudgetBar
                  key={budget.budget}
                  budget={budget}
                  maxAmount={maxAmount}
                  selected={selectedBudget === budget.budget}
                  onSelect={() => setSelectedBudget(budget.budget)}
                />
              ))}
            </div>
          </section>

          <section className={`surface ${s.detailsCard}`}>
            <div className={s.cardHeader}>
              <div>
                <h2>Dettaglio ordini</h2>
                <p>
                  {selectedBudget
                    ? `Filtro budget: ${selectedBudget}`
                    : 'Tutti gli ordini inclusi nel periodo selezionato.'}
                </p>
              </div>
            </div>
            {visibleDetails.length === 0 ? (
              <div className={s.emptyState}>
                <div className={s.stateIcon}><Icon name="file-text" size={30} /></div>
                <p className="emptyStateTitle">Nessun ordine di acquisto trovato per il periodo selezionato.</p>
              </div>
            ) : (
              <div className={s.tableWrap}>
                <table className={s.table}>
                  <thead>
                    <tr>
                      <th>Ordine</th>
                      <th>Oggetto</th>
                      <th>Budget</th>
                      <th className={s.numeric}>Importo totale</th>
                      <th>Valuta</th>
                      <th>Richiedente</th>
                      <th>Fornitore</th>
                      <th>Stato</th>
                      <th>Risoluzione</th>
                      <th>Creato</th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleDetails.map((detail, idx) => (
                      <tr key={detail.issue_key} style={{ animationDelay: `${Math.min(idx * 20, 300)}ms` }}>
                        <td>
                          <div className={s.orderKey}>{detail.issue_key}</div>
                          {detail.numero_ordine && <div className={s.secondaryText}>{detail.numero_ordine}</div>}
                        </td>
                        <td>{nz(detail.summary)}</td>
                        <td>{nz(detail.budget_di_riferimento)}</td>
                        <td className={s.numeric}>{formatEUR(detail.importo_totale)}</td>
                        <td>{nz(detail.valuta)}</td>
                        <td>{nz(detail.reporter_name)}</td>
                        <td>{nz(detail.fornitore_selezionato)}</td>
                        <td>{nz(detail.status)}</td>
                        <td>{nz(detail.resolution)}</td>
                        <td>{formatDateTime(detail.created)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </>
      )}
    </main>
  );
}

function BudgetBar({
  budget,
  maxAmount,
  selected,
  onSelect,
}: {
  budget: RiepilogoBudget;
  maxAmount: number;
  selected: boolean;
  onSelect: () => void;
}) {
  const width = maxAmount > 0 ? Math.max(4, (budget.amount / maxAmount) * 100) : 0;
  const tooltip = `${budget.budget} — ${formatEUR(budget.amount)} — ${formatPercent(budget.percentage)}% — ${budget.order_count} ordini`;

  return (
    <button
      type="button"
      className={`${s.barButton} ${selected ? s.barButtonSelected : ''}`}
      onClick={onSelect}
      title={tooltip}
      aria-pressed={selected}
      aria-label={tooltip}
    >
      <span className={s.barMeta}>
        <span className={s.barBudget}>{budget.budget}</span>
        <span className={s.barValues}>
          {formatEUR(budget.amount)} · {formatPercent(budget.percentage)}% · {budget.order_count} ordini
        </span>
      </span>
      <span className={s.barTrack} aria-hidden="true">
        <span className={s.barFill} style={{ width: `${width}%` }} />
      </span>
    </button>
  );
}
