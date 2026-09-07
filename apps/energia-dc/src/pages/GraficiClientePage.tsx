import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Button, SingleSelect, Skeleton, VisuallyHidden } from '@mrsmith/ui';
import { useCustomers, useKWReport } from '../api/queries';
import type { KWReportParams } from '../api/types';
import {
  KWReportChart,
  type KWReportChartSpec,
} from '../components/KWReportChart';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import { ViewState } from '../components/ViewState';
import {
  exportChartPNG,
  exportReportPDF,
  reportCharts,
  reportPeriod,
} from '../utils/kwReportExport';
import styles from './GraficiClientePage.module.css';

const months = Array.from({ length: 12 }, (_, index) => ({
  value: index + 1,
  label: new Intl.DateTimeFormat('it-IT', {
    month: 'long',
    timeZone: 'UTC',
  }).format(new Date(Date.UTC(2026, index, 1))),
}));

export function GraficiClientePage() {
  const [customerId, setCustomerId] = useState<number | null>(null);
  const [year, setYear] = useState(
    new Intl.DateTimeFormat('en', {
      year: 'numeric',
      timeZone: 'Europe/Rome',
    }).format(new Date()),
  );
  const [month, setMonth] = useState<number | null>(null);
  const [cosfi, setCosfi] = useState(95);
  const [unit, setUnit] = useState<'kW' | 'A'>('kW');
  const [submitted, setSubmitted] = useState<KWReportParams | null>(null);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState(false);
  const customers = useCustomers();
  const query = useKWReport(submitted);
  const report = query.data;
  const validYear =
    /^\d{4}$/.test(year) && Number(year) >= 1000 && Number(year) <= 9998;
  const disabledExport = exporting || query.isFetching;
  const specs = report ? reportCharts(report) : [];
  const error = customers.error ?? query.error;

  function update() {
    if (customerId === null || !validYear) return;
    const params = {
      customerId,
      year: Number(year),
      month: month ?? undefined,
      cosfi,
      unit,
    };
    setExportError(false);
    if (
      submitted &&
      submitted.customerId === params.customerId &&
      submitted.year === params.year &&
      submitted.month === params.month &&
      submitted.cosfi === params.cosfi &&
      submitted.unit === params.unit
    )
      void query.refetch();
    else setSubmitted(params);
  }

  async function generate(spec?: KWReportChartSpec) {
    if (!report || disabledExport) return;
    setExporting(true);
    setExportError(false);
    try {
      if (spec) await exportChartPNG(spec);
      else await exportReportPDF(report);
    } catch {
      setExportError(true);
    } finally {
      setExporting(false);
    }
  }

  function chart(spec: KWReportChartSpec) {
    return (
      <section
        className={styles.chartSection}
        key={spec.key}
        aria-label={`${spec.title} · ${spec.customer} · ${spec.period}`}
      >
        <div className={styles.sectionHeader}>
          <div>
            <h2 className={styles.sectionTitle}>{spec.title}</h2>
            <p className={styles.meta}>
              {spec.customer} · {spec.period} · {spec.unit} medi
            </p>
          </div>
          <Button
            variant="secondary"
            onClick={() => void generate(spec)}
            disabled={disabledExport}
            aria-label={`Scarica PNG: ${spec.title}`}
          >
            Scarica PNG
          </Button>
        </div>
        {spec.series.some((point) => point.kilowatt !== null) ? (
          <KWReportChart series={spec.series} unit={spec.unit} />
        ) : (
          <ViewState
            title="Nessuna lettura disponibile"
            message="Non sono presenti letture per il periodo selezionato."
          />
        )}
      </section>
    );
  }

  return (
    <div className={styles.page}>
      <header>
        <h1 className={styles.title}>Grafici cliente</h1>
        <p className={styles.subtitle}>
          Potenza media del cliente, per sala e per rack. Scarica i grafici o il
          PDF completo.
        </p>
      </header>
      <form
        className={styles.filters}
        onSubmit={(event) => {
          event.preventDefault();
          update();
        }}
      >
        <div className={styles.customerField}>
          <label>
            Cliente <span className={styles.required} aria-hidden="true" />{' '}
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </label>
          <SingleSelect
            ariaLabel="Cliente"
            options={(customers.data ?? []).map((item) => ({
              value: item.id,
              label: item.name,
            }))}
            selected={customerId}
            onChange={setCustomerId}
            disabled={customers.isLoading}
            placeholder="Seleziona cliente"
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="kw-report-year">
            Anno <span className={styles.required} aria-hidden="true" />{' '}
            <VisuallyHidden>obbligatorio</VisuallyHidden>
          </label>
          <input
            id="kw-report-year"
            type="number"
            min={1000}
            max={9998}
            step={1}
            required
            value={year}
            onChange={(event) => setYear(event.target.value)}
            aria-invalid={!validYear}
            aria-describedby={!validYear ? 'kw-year-error' : undefined}
          />
          {!validYear && (
            <span className={styles.validation} id="kw-year-error">
              Inserisci un anno tra 1000 e 9998.
            </span>
          )}
        </div>
        <div className={styles.field}>
          <label>Mese facoltativo</label>
          <SingleSelect
            ariaLabel="Mese facoltativo"
            options={months}
            selected={month}
            onChange={setMonth}
            allowClear
            clearLabel="Anno intero"
            placeholder="Anno intero"
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="kw-report-cosfi">
            Cos φ · {(cosfi / 100).toFixed(2).replace('.', ',')}
          </label>
          <input
            id="kw-report-cosfi"
            type="range"
            min={70}
            max={100}
            step={1}
            value={cosfi}
            onChange={(event) => setCosfi(Number(event.target.value))}
            aria-valuetext={(cosfi / 100).toFixed(2).replace('.', ',')}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="kw-report-unit">Unità</label>
          <SingleSelect
            ariaLabel="Unità"
            options={[
              { value: 'kW', label: 'kW' },
              { value: 'A', label: 'A' },
            ]}
            selected={unit}
            onChange={(value) => setUnit(value as 'kW' | 'A')}
            placeholder="kW"
          />
        </div>
        <Button
          type="submit"
          disabled={
            customerId === null || !validYear || exporting || query.isFetching
          }
        >
          Aggiorna
        </Button>
      </form>
      {customers.isLoading && <Skeleton rows={2} />}
      {error ? (
        error instanceof ApiError && error.status === 503 ? (
          <ServiceUnavailable />
        ) : (
          <div>
            <ViewState
              title="Grafici non disponibili"
              message="Non è stato possibile caricare i dati. Riprova ad aggiornare."
              tone="error"
            />
            <Button
              onClick={() =>
                void (customers.error ? customers.refetch() : query.refetch())
              }
            >
              Riprova
            </Button>
          </div>
        )
      ) : null}
      {!customers.isLoading && !error && !submitted && (
        <ViewState
          title="Seleziona un cliente"
          message="Scegli il periodo e premi Aggiorna per visualizzare i grafici."
        />
      )}
      {submitted && query.isLoading && <Skeleton rows={6} />}
      {exportError && (
        <div role="alert">
          <ViewState
            title="Esportazione non riuscita"
            message="Riprova a scaricare il grafico o il PDF completo."
            tone="error"
          />
        </div>
      )}
      {report && !query.error && (
        <>
          <div className={styles.reportHeader}>
            <p className={styles.meta}>
              {report.customer.name} · {reportPeriod(report)} ·{' '}
              {report.month ? 'Medie giornaliere' : 'Medie mensili'}
              <br />
              Gli intervalli senza letture non hanno valore.
            </p>
            <Button
              variant="secondary"
              disabled={disabledExport}
              onClick={() => void generate()}
            >
              {exporting ? 'Esportazione in corso…' : 'Scarica PDF completo'}
            </Button>
          </div>
          {query.isFetching && (
            <div role="status">Aggiornamento dei grafici…</div>
          )}
          {specs[0] && chart(specs[0])}
          {report.rooms.map((room) => (
            <div className={styles.room} key={room.id}>
              {chart(specs.find((spec) => spec.key === `sala-${room.id}`)!)}
              <details className={styles.details}>
                <summary>Dettaglio rack · {room.name}</summary>
                <div className={styles.racks}>
                  {room.racks.map((rack) =>
                    chart(
                      specs.find(
                        (spec) =>
                          spec.key === `sala-${room.id}-rack-${rack.id}`,
                      )!,
                    ),
                  )}
                </div>
              </details>
            </div>
          ))}
        </>
      )}
    </div>
  );
}
