import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { SingleSelect } from '@mrsmith/ui';
import { useCustomers, useLowConsumption } from '../api/queries';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import { ViewState } from '../components/ViewState';
import { formatAmpere, formatCount, formatMaybeText } from '../utils/format';
import styles from './shared.module.css';

function isServiceUnavailable(error: unknown) {
  return error instanceof ApiError && error.status === 503;
}

export function ConsumiBassiPage() {
  const [threshold, setThreshold] = useState('1');
  const [customerId, setCustomerId] = useState<number | null>(null);
  const [submitted, setSubmitted] = useState<{ min: number; customerId: number | null } | null>(null);

  const customersQ = useCustomers();
  const lowConsumptionQ = useLowConsumption(submitted);
  const customerOptions = customersQ.data ?? [];

  function handleSearch() {
    const parsedThreshold = Number(threshold);
    if (!Number.isFinite(parsedThreshold)) {
      return;
    }
    setSubmitted({
      min: parsedThreshold,
      customerId,
    });
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.title}>Consumi &lt; 1 A</h1>
          <p className={styles.subtitle}>
            Imposta una soglia minima e, se necessario, restringi la ricerca a un cliente per individuare prese sotto assorbimento.
          </p>
        </div>
      </div>

      {customersQ.error && isServiceUnavailable(customersQ.error) ? <ServiceUnavailable /> : null}
      {customersQ.error && !isServiceUnavailable(customersQ.error) ? (
        <ViewState
          title="Clienti non disponibili"
          message="Non e stato possibile caricare il catalogo clienti per la ricerca dei consumi bassi."
          tone="error"
        />
      ) : null}

      {!customersQ.error ? (
        <>
          <section className={styles.card}>
            <div className={styles.toolbar}>
              <div className={`${styles.field} ${styles.fieldCompact}`}>
                <label>Soglia minima</label>
                <input type="number" min="0" step="0.1" value={threshold} onChange={(event) => setThreshold(event.target.value)} />
              </div>
              <div className={styles.field}>
                <label>Cliente</label>
                <SingleSelect
                  options={customerOptions.map((item) => ({ value: item.id, label: item.name }))}
                  selected={customerId}
                  onChange={setCustomerId}
                  allowClear
                  placeholder="Tutti i clienti"
                />
              </div>
              <div className={styles.actions}>
                <button
                  type="button"
                  className={styles.btnPrimary}
                  onClick={handleSearch}
                  disabled={threshold.trim() === '' || Number.isNaN(Number(threshold))}
                >
                  Cerca
                </button>
              </div>
            </div>
          </section>

          {submitted === null ? (
            <ViewState
              title="Ricerca non avviata"
              message="Conferma la soglia minima per cercare prese sotto assorbimento nel perimetro selezionato."
            />
          ) : null}

          {submitted !== null && lowConsumptionQ.error && isServiceUnavailable(lowConsumptionQ.error) ? <ServiceUnavailable /> : null}
          {submitted !== null && lowConsumptionQ.error && !isServiceUnavailable(lowConsumptionQ.error) ? (
            <ViewState
              title="Ricerca non disponibile"
              message="La ricerca delle prese sotto soglia non e al momento disponibile."
              tone="error"
            />
          ) : null}

          {submitted !== null && !lowConsumptionQ.error ? (
            <section className={styles.card}>
              <div className={styles.sectionHeader}>
                <div>
                  <h2 className={styles.sectionTitle}>Prese sotto soglia</h2>
                  <div className={styles.sectionMeta}>
                    Soglia {submitted.min} A {submitted.customerId ? '· filtro cliente attivo' : '· tutti i clienti'}
                  </div>
                </div>
                {!lowConsumptionQ.isLoading ? (
                  <div className={styles.sectionMeta}>
                    {formatCount(lowConsumptionQ.data?.length ?? 0, 'presa', 'prese trovate')}
                  </div>
                ) : null}
              </div>

              {lowConsumptionQ.isLoading ? (
                <ViewState
                  title="Caricamento in corso"
                  message="La tabella delle prese sotto soglia si sta aggiornando."
                />
              ) : null}

              {!lowConsumptionQ.isLoading && (lowConsumptionQ.data ?? []).length === 0 ? (
                <ViewState
                  title="Nessuna presa sotto soglia"
                  message="La ricerca confermata non restituisce prese con assorbimento entro la soglia impostata."
                />
              ) : null}

              {!lowConsumptionQ.isLoading && (lowConsumptionQ.data ?? []).length > 0 ? (
                <div className={styles.tableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        <th>Cliente</th>
                        <th>Edificio</th>
                        <th>Sala</th>
                        <th>Rack</th>
                        <th>Presa rack</th>
                        <th className={styles.alignRight}>Ampere</th>
                        <th>Power meter</th>
                        <th>Magnetotermico</th>
                        <th>Posizioni</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(lowConsumptionQ.data ?? []).map((row) => (
                        <tr key={`${row.customerId}-${row.socketId}`}>
                          <td>{row.customerName}</td>
                          <td>{row.buildingName}</td>
                          <td>{row.roomName}</td>
                          <td>{row.rackName}</td>
                          <td>{row.socketLabel}</td>
                          <td className={styles.alignRight}>{formatAmpere(row.ampere)}</td>
                          <td>{formatMaybeText(row.powerMeter)}</td>
                          <td>{formatMaybeText(row.breaker)}</td>
                          <td>{row.positions.join(' / ') || '-'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : null}
            </section>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
