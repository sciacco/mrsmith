import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { SingleSelect } from '@mrsmith/ui';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { useCustomerKWSeries, useCustomers } from '../api/queries';
import type { KWPeriod, LookupItem } from '../api/types';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import { ViewState } from '../components/ViewState';
import { formatCount, formatKw } from '../utils/format';
import styles from './shared.module.css';

function isServiceUnavailable(error: unknown) {
  return error instanceof ApiError && error.status === 503;
}

function resolveCustomerName(options: LookupItem[], customerId: number | null) {
  if (customerId === null) return '';
  return options.find((option) => option.id === customerId)?.name ?? '';
}

export function ConsumiKwPage() {
  const [customerId, setCustomerId] = useState<number | null>(null);
  const [period, setPeriod] = useState<KWPeriod>('day');
  const [cosfi, setCosfi] = useState(95);
  const [submitted, setSubmitted] = useState<{ customerId: number; customerName: string; period: KWPeriod; cosfi: number } | null>(null);

  const customersQ = useCustomers();
  const seriesQ = useCustomerKWSeries(
    submitted
      ? {
          customerId: submitted.customerId,
          period: submitted.period,
          cosfi: submitted.cosfi,
        }
      : null,
  );

  const customerOptions = customersQ.data ?? [];

  function handleSubmit() {
    if (customerId === null) return;
    setSubmitted({
      customerId,
      customerName: resolveCustomerName(customerOptions, customerId),
      period,
      cosfi,
    });
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.title}>Consumi kW</h1>
          <p className={styles.subtitle}>
            Analizza l&apos;andamento giornaliero o mensile del cliente selezionato e aggiorna il grafico con il valore di Cos fi desiderato.
          </p>
        </div>
      </div>

      {customersQ.error && isServiceUnavailable(customersQ.error) ? <ServiceUnavailable /> : null}
      {customersQ.error && !isServiceUnavailable(customersQ.error) ? (
        <ViewState
          title="Clienti non disponibili"
          message="Non e stato possibile caricare il catalogo clienti per il grafico dei consumi."
          tone="error"
        />
      ) : null}

      {!customersQ.error ? (
        <>
          <section className={styles.card}>
            <div className={styles.toolbar}>
              <div className={styles.field}>
                <label>Cliente</label>
                <SingleSelect
                  options={customerOptions.map((item) => ({ value: item.id, label: item.name }))}
                  selected={customerId}
                  onChange={setCustomerId}
                  placeholder="Seleziona cliente"
                />
              </div>
              <div className={`${styles.field} ${styles.fieldCompact}`}>
                <label>Periodo</label>
                <SingleSelect
                  options={[
                    { value: 'day', label: 'Giornaliero' },
                    { value: 'month', label: 'Mensile' },
                  ]}
                  selected={period}
                  onChange={(value) => setPeriod((value as KWPeriod | null) ?? 'day')}
                  placeholder="Seleziona periodo"
                />
              </div>
              <div className={`${styles.field} ${styles.fieldWide}`}>
                <label>Cos fi</label>
                <div className={styles.rangeWrap}>
                  <input
                    className={styles.rangeInput}
                    type="range"
                    min={70}
                    max={100}
                    step={1}
                    value={cosfi}
                    onChange={(event) => setCosfi(Number(event.target.value))}
                  />
                  <span className={styles.rangeValue}>{cosfi}%</span>
                </div>
              </div>
              <div className={styles.actions}>
                <button type="button" className={styles.btnPrimary} onClick={handleSubmit} disabled={customerId === null}>
                  Aggiorna
                </button>
              </div>
            </div>
          </section>

          {submitted === null ? (
            <ViewState
              title="Grafico in attesa"
              message="Seleziona cliente, periodo e Cos fi per caricare l&apos;andamento dei consumi."
            />
          ) : null}

          {submitted !== null && seriesQ.error && isServiceUnavailable(seriesQ.error) ? <ServiceUnavailable /> : null}
          {submitted !== null && seriesQ.error && !isServiceUnavailable(seriesQ.error) ? (
            <ViewState
              title="Consumi non disponibili"
              message="Non e stato possibile aggiornare il grafico dei consumi per i parametri selezionati."
              tone="error"
            />
          ) : null}

          {submitted !== null && !seriesQ.error ? (
            <section className={styles.card}>
              <div className={styles.sectionHeader}>
                <div>
                  <h2 className={styles.sectionTitle}>
                    {submitted.customerName} · Cos fi {submitted.cosfi}%
                  </h2>
                  <div className={styles.sectionMeta}>
                    {submitted.period === 'day' ? 'Serie giornaliera' : 'Serie mensile'} · {formatCount(seriesQ.data?.length ?? 0, 'rilevazione', 'rilevazioni')}
                  </div>
                </div>
              </div>

              {seriesQ.isLoading ? (
                <ViewState
                  title="Caricamento in corso"
                  message="Il grafico dei consumi si sta aggiornando con i parametri confermati."
                />
              ) : null}

              {!seriesQ.isLoading && (seriesQ.data ?? []).length === 0 ? (
                <ViewState
                  title="Nessun consumo disponibile"
                  message="Il cliente selezionato non restituisce valori kW per il periodo richiesto."
                />
              ) : null}

              {!seriesQ.isLoading && (seriesQ.data ?? []).length > 0 ? (
                <div className={styles.chartWrapTall}>
                  <ResponsiveContainer>
                    <BarChart data={seriesQ.data}>
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                      <XAxis dataKey="label" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} />
                      <Tooltip
                        formatter={(value: number) => formatKw(value)}
                        contentStyle={{
                          background: 'var(--color-bg-elevated)',
                          border: '1px solid var(--color-border)',
                          borderRadius: 12,
                        }}
                      />
                      <Bar dataKey="kilowatt" name="kW" fill="#0ea5e9" radius={[8, 8, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              ) : null}
            </section>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
