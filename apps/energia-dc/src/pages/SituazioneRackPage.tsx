import { useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { SingleSelect } from '@mrsmith/ui';
import {
  CartesianGrid,
  ComposedChart,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import {
  useCustomers,
  usePowerReadings,
  useRackDetail,
  useRackSocketStatus,
  useRacks,
  useRackStatsLastDays,
  useRooms,
  useSites,
} from '../api/queries';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import { ViewState } from '../components/ViewState';
import {
  formatAmpere,
  formatCount,
  formatDate,
  formatDateTime,
  formatKw,
  formatMaybeText,
  toDateTimeLocalInput,
} from '../utils/format';
import type { LookupItem } from '../api/types';
import styles from './shared.module.css';

interface SubmittedRackFilters {
  customerId: number;
  siteId: number;
  roomId: number;
  rackId: number;
  from: string;
  to: string;
  labels: {
    customer: string;
    site: string;
    room: string;
    rack: string;
  };
}

function resolveLabel(options: LookupItem[], selectedId: number | null): string {
  if (selectedId === null) return '';
  return options.find((option) => option.id === selectedId)?.name ?? '';
}

function rackTypeChip(
  rackType: string | undefined,
  position: string | undefined,
): { text: string; variant: 'full' | 'half' } | null {
  const type = (rackType ?? '').trim().toUpperCase();
  if (!type) return null;
  if (type === 'HALF') {
    const pos = (position ?? '').trim();
    return { text: pos ? `HALF posizione ${pos}` : 'HALF', variant: 'half' };
  }
  if (type === 'FULL') return { text: 'FULL', variant: 'full' };
  const pos = (position ?? '').trim();
  return { text: pos ? `${type} posizione ${pos}` : type, variant: 'full' };
}

function defaultDateRange() {
  const now = new Date();
  const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000);
  return {
    from: toDateTimeLocalInput(yesterday),
    to: toDateTimeLocalInput(now),
  };
}

function isServiceUnavailable(error: unknown) {
  return error instanceof ApiError && error.status === 503;
}

export function SituazioneRackPage() {
  const initialRange = useMemo(() => defaultDateRange(), []);
  const [customerId, setCustomerId] = useState<number | null>(null);
  const [siteId, setSiteId] = useState<number | null>(null);
  const [roomId, setRoomId] = useState<number | null>(null);
  const [rackId, setRackId] = useState<number | null>(null);
  const [from, setFrom] = useState(initialRange.from);
  const [to, setTo] = useState(initialRange.to);
  const [page, setPage] = useState(1);
  const [submitted, setSubmitted] = useState<SubmittedRackFilters | null>(null);

  const customersQ = useCustomers();
  const sitesQ = useSites(customerId);
  const roomsQ = useRooms(siteId, customerId);
  const racksQ = useRacks(roomId, customerId);

  const rackDetailQ = useRackDetail(submitted?.rackId ?? null);
  const socketStatusQ = useRackSocketStatus(submitted?.rackId ?? null);
  const rackStatsQ = useRackStatsLastDays(submitted?.rackId ?? null);
  const powerReadingsQ = usePowerReadings(
    submitted
      ? {
          rackId: submitted.rackId,
          from: submitted.from,
          to: submitted.to,
          page,
          size: 20,
        }
      : null,
  );

  const customerOptions = customersQ.data ?? [];
  const siteOptions = sitesQ.data ?? [];
  const roomOptions = roomsQ.data ?? [];
  const rackOptions = racksQ.data ?? [];

  const canSubmit = rackId !== null && from !== '' && to !== '';
  const workspaceError = rackDetailQ.error ?? socketStatusQ.error ?? rackStatsQ.error ?? powerReadingsQ.error;
  const workspaceLoading =
    submitted !== null &&
    (rackDetailQ.isLoading || socketStatusQ.isLoading || rackStatsQ.isLoading || powerReadingsQ.isLoading);

  const totalPages = powerReadingsQ.data ? Math.max(1, Math.ceil(powerReadingsQ.data.total / powerReadingsQ.data.size)) : 1;

  function handleCustomerChange(nextValue: number | null) {
    setCustomerId(nextValue);
    setSiteId(null);
    setRoomId(null);
    setRackId(null);
  }

  function handleSiteChange(nextValue: number | null) {
    setSiteId(nextValue);
    setRoomId(null);
    setRackId(null);
  }

  function handleRoomChange(nextValue: number | null) {
    setRoomId(nextValue);
    setRackId(null);
  }

  function handleReset() {
    const nextRange = defaultDateRange();
    setCustomerId(null);
    setSiteId(null);
    setRoomId(null);
    setRackId(null);
    setFrom(nextRange.from);
    setTo(nextRange.to);
    setPage(1);
    setSubmitted(null);
  }

  function handleSubmit() {
    if (!canSubmit || customerId === null || siteId === null || roomId === null || rackId === null) {
      return;
    }
    setPage(1);
    setSubmitted({
      customerId,
      siteId,
      roomId,
      rackId,
      from,
      to,
      labels: {
        customer: resolveLabel(customerOptions, customerId),
        site: resolveLabel(siteOptions, siteId),
        room: resolveLabel(roomOptions, roomId),
        rack: resolveLabel(rackOptions, rackId),
      },
    });
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.title}>Situazione rack</h1>
          <p className={styles.subtitle}>
            Seleziona il percorso Cliente, Edificio, Sala e Rack per consultare Socket, storico assorbimenti e andamento degli ultimi due giorni.
          </p>
        </div>
      </div>

      {customersQ.error && isServiceUnavailable(customersQ.error) ? (
        <ServiceUnavailable />
      ) : null}

      {customersQ.error && !isServiceUnavailable(customersQ.error) ? (
        <ViewState
          title="Clienti non disponibili"
          message="Non e stato possibile caricare il catalogo clienti per questo workspace."
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
                  onChange={handleCustomerChange}
                  placeholder="Seleziona cliente"
                />
              </div>
              <div className={styles.field}>
                <label>Edificio</label>
                <SingleSelect
                  options={siteOptions.map((item) => ({ value: item.id, label: item.name }))}
                  selected={siteId}
                  onChange={handleSiteChange}
                  placeholder="Seleziona edificio"
                />
              </div>
              <div className={styles.field}>
                <label>Sala</label>
                <SingleSelect
                  options={roomOptions.map((item) => ({ value: item.id, label: item.name }))}
                  selected={roomId}
                  onChange={handleRoomChange}
                  placeholder="Seleziona sala"
                />
              </div>
              <div className={styles.field}>
                <label>Rack</label>
                <SingleSelect
                  options={rackOptions.map((item) => ({ value: item.id, label: item.name }))}
                  selected={rackId}
                  onChange={setRackId}
                  placeholder="Seleziona rack"
                />
              </div>
              <div className={`${styles.field} ${styles.fieldCompact}`}>
                <label>Letture dal</label>
                <input type="datetime-local" value={from} onChange={(event) => setFrom(event.target.value)} />
              </div>
              <div className={`${styles.field} ${styles.fieldCompact}`}>
                <label>Letture al</label>
                <input type="datetime-local" value={to} onChange={(event) => setTo(event.target.value)} />
              </div>
              <div className={styles.actions}>
                <button type="button" className={styles.btnPrimary} onClick={handleSubmit} disabled={!canSubmit}>
                  Aggiorna
                </button>
                <button type="button" className={styles.btnSecondary} onClick={handleReset}>
                  Reimposta
                </button>
              </div>
            </div>
          </section>

          {submitted === null ? (
            <ViewState
              title="Workspace pronto"
              message="Completa il percorso di selezione e conferma con Aggiorna per caricare Socket rack, storico assorbimenti e andamento degli ultimi due giorni."
            />
          ) : null}

          {submitted !== null && workspaceError && isServiceUnavailable(workspaceError) ? <ServiceUnavailable /> : null}

          {submitted !== null && workspaceError && !isServiceUnavailable(workspaceError) ? (
            <ViewState
              title="Dati rack non disponibili"
              message="Il caricamento del rack selezionato non e andato a buon fine. Riprova con gli stessi filtri o aggiorna il percorso."
              tone="error"
            />
          ) : null}

          {submitted !== null && !workspaceError ? (
            <>
              {workspaceLoading ? (
                <ViewState
                  title="Caricamento in corso"
                  message="Il workspace del rack si sta aggiornando con la selezione confermata."
                />
              ) : (
                <>
                  <div className={styles.gridTwo}>
                    <section className={styles.card}>
                      <div className={styles.sectionHeader}>
                        <h2 className={styles.sectionTitle}>Dettaglio rack</h2>
                        <div className={styles.sectionMeta}>{rackDetailQ.data?.name ?? submitted.labels.rack}</div>
                      </div>
                      <div className={styles.metaGrid}>
                        <div className={styles.metaItem}>
                          <span className={styles.metaLabel}>Floor</span>
                          <span className={styles.metaValue}>{formatMaybeText(rackDetailQ.data?.floor)}</span>
                        </div>
                        <div className={styles.metaItem}>
                          <span className={styles.metaLabel}>Island</span>
                          <span className={styles.metaValue}>{formatMaybeText(rackDetailQ.data?.island)}</span>
                        </div>
                        <div className={styles.metaItem}>
                          <span className={styles.metaLabel}>Tipo rack</span>
                          <span className={styles.metaValue}>
                            {(() => {
                              const chip = rackTypeChip(rackDetailQ.data?.rackType, rackDetailQ.data?.position);
                              if (!chip) return formatMaybeText(undefined);
                              return (
                                <span
                                  className={`${styles.chip} ${chip.variant === 'full' ? styles.chipFull : styles.chipHalf}`}
                                >
                                  {chip.text}
                                </span>
                              );
                            })()}
                          </span>
                        </div>
                        <div className={styles.metaItem}>
                          <span className={styles.metaLabel}>Codice ordine</span>
                          <span className={styles.metaValue}>{formatMaybeText(rackDetailQ.data?.orderCode)}</span>
                        </div>
                        <div className={styles.metaItem}>
                          <span className={styles.metaLabel}>Seriale</span>
                          <span className={styles.metaValue}>{formatMaybeText(rackDetailQ.data?.serialNumber)}</span>
                        </div>
                        <div className={styles.metaItem}>
                          <span className={styles.metaLabel}>Committed Ampere</span>
                          <span className={styles.metaValue}>{formatAmpere(rackDetailQ.data?.committedPower)}</span>
                        </div>
                        <div className={styles.metaItem}>
                          <span className={styles.metaLabel}>Inizio Fatturazione</span>
                          <span className={styles.metaValue}>{formatDate(rackDetailQ.data?.billingStartDate)}</span>
                        </div>
                        <div className={styles.metaItem}>
                          <span className={styles.metaLabel}>Tipo fatturazione</span>
                          <span className={styles.metaValue}>
                            {rackDetailQ.data ? (rackDetailQ.data.variableBilling ? 'Variabile' : 'Fissa') : formatMaybeText(undefined)}
                          </span>
                        </div>
                      </div>
                    </section>

                    <section className={styles.card}>
                      <div className={styles.sectionHeader}>
                        <h2 className={styles.sectionTitle}>Socket rack</h2>
                        <div className={styles.sectionMeta}>
                          {formatCount(socketStatusQ.data?.length ?? 0, 'Socket trovato', 'Socket trovati')}
                        </div>
                      </div>
                      {(socketStatusQ.data ?? []).length === 0 ? (
                        <ViewState
                          title="Nessun Socket rilevato"
                          message="Il rack selezionato non restituisce Socket monitorati nel periodo corrente."
                        />
                      ) : (
                        <div className={styles.gaugeGrid}>
                          {(socketStatusQ.data ?? []).map((socket) => {
                            const dialFill = `${Math.max(0, Math.min(socket.usagePercent, 100))}%`;
                            const dialColor =
                              socket.usagePercent > 90
                                ? 'var(--color-danger)'
                                : socket.usagePercent > 80
                                ? 'var(--color-warning)'
                                : 'var(--color-success)';
                            const positionsLabel =
                              socket.positions.length > 0 ? socket.positions.join(' ') : '-';
                            const positionsTitle =
                              socket.positions.length === 1 ? 'Posizione' : 'Posizioni';
                            const deviceMeta = [socket.powerMeter, socket.detectorIp]
                              .filter((value) => value && value.trim().length > 0)
                              .join(' · ');
                            return (
                              <article key={socket.socketId} className={styles.gaugeCard}>
                                <div className={styles.gaugeInfo}>
                                  <div className={styles.gaugeTitle}>Socket #{socket.socketId}</div>
                                  <div className={styles.gaugeMeta}>{socket.breaker || 'Breaker non disponibile'}</div>
                                  {deviceMeta && <div className={styles.gaugeMeta}>{deviceMeta}</div>}
                                  <div className={styles.gaugeMeta}>
                                    {positionsTitle}: {positionsLabel}
                                  </div>
                                </div>
                                <div
                                  className={styles.gaugeDial}
                                  style={{
                                    ['--dial-fill' as string]: dialFill,
                                    ['--dial-color' as string]: dialColor,
                                  }}
                                >
                                  <span className={styles.gaugeValue}>{formatAmpere(socket.ampere)}</span>
                                </div>
                              </article>
                            );
                          })}
                        </div>
                      )}
                    </section>
                  </div>

                  <div className={styles.gridWide}>
                    <section className={styles.card}>
                      <div className={styles.sectionHeader}>
                        <h2 className={styles.sectionTitle}>Assorbimenti ultimi due giorni</h2>
                        <div className={styles.sectionMeta}>
                          {formatCount(rackStatsQ.data?.length ?? 0, 'punto', 'punti orari')}
                        </div>
                      </div>
                      {(rackStatsQ.data ?? []).length === 0 ? (
                        <ViewState
                          title="Nessun andamento disponibile"
                          message="Il rack selezionato non ha serie storiche utili negli ultimi due giorni."
                        />
                      ) : (
                        <div className={styles.chartWrapTall}>
                          <ResponsiveContainer>
                            <ComposedChart data={rackStatsQ.data}>
                              <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                              <XAxis dataKey="bucket" tick={{ fontSize: 11 }} tickFormatter={(value) => value.slice(5)} />
                              <YAxis yAxisId="ampere" tick={{ fontSize: 11 }} />
                              <YAxis yAxisId="kw" orientation="right" tick={{ fontSize: 11 }} />
                              <Tooltip
                                formatter={(value: number, name: string) => (
                                  name === 'Ampere' ? formatAmpere(value) : formatKw(value)
                                )}
                                labelFormatter={(value) => formatDateTime(value)}
                                contentStyle={{
                                  background: 'var(--color-bg-elevated)',
                                  border: '1px solid var(--color-border)',
                                  borderRadius: 12,
                                }}
                              />
                              <Line
                                yAxisId="ampere"
                                type="monotone"
                                dataKey="ampere"
                                name="Ampere"
                                stroke="#0ea5e9"
                                strokeWidth={2}
                                dot={false}
                              />
                              <Line
                                yAxisId="kw"
                                type="monotone"
                                dataKey="kilowatt"
                                name="kW"
                                stroke="#f59e0b"
                                strokeWidth={2}
                                dot={false}
                              />
                            </ComposedChart>
                          </ResponsiveContainer>
                        </div>
                      )}
                    </section>

                    <section className={styles.card}>
                      <div className={styles.sectionHeader}>
                        <h2 className={styles.sectionTitle}>Storico assorbimenti</h2>
                        <div className={styles.sectionMeta}>
                          {powerReadingsQ.data ? formatCount(powerReadingsQ.data.total, 'lettura', 'letture') : '0 letture'}
                        </div>
                      </div>
                      {(powerReadingsQ.data?.items ?? []).length === 0 ? (
                        <ViewState
                          title="Nessuna lettura disponibile"
                          message="Il periodo selezionato non restituisce letture per il rack confermato."
                        />
                      ) : (
                        <>
                          <div className={styles.tableWrap}>
                            <table className={styles.table}>
                              <thead>
                                <tr>
                                  <th>Socket rack</th>
                                  <th>Data</th>
                                  <th>OID</th>
                                  <th className={styles.alignRight}>Ampere</th>
                                </tr>
                              </thead>
                              <tbody>
                                {(powerReadingsQ.data?.items ?? []).map((row) => (
                                  <tr key={row.id}>
                                    <td>{row.socketLabel}</td>
                                    <td>{formatDateTime(row.date)}</td>
                                    <td className={styles.mono}>{row.oid}</td>
                                    <td className={styles.alignRight}>{formatAmpere(row.ampere)}</td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          </div>
                          <div className={styles.pagination}>
                            <div className={styles.paginationInfo}>
                              Pagina {powerReadingsQ.data?.page ?? 1} di {totalPages}
                            </div>
                            <div className={styles.actions}>
                              <button
                                type="button"
                                className={styles.btnSecondary}
                                onClick={() => setPage((current) => Math.max(1, current - 1))}
                                disabled={page <= 1}
                              >
                                Precedente
                              </button>
                              <button
                                type="button"
                                className={styles.btnSecondary}
                                onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
                                disabled={page >= totalPages}
                              >
                                Successiva
                              </button>
                            </div>
                          </div>
                        </>
                      )}
                    </section>
                  </div>
                </>
              )}
            </>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
