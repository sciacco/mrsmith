import { useMemo, useState } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCustomers } from '../../hooks/useCustomers';
import { useCustomerStates } from '../../hooks/useCustomerStates';
import { UpdateStateModal } from './UpdateStateModal';
import styles from './StatoAziendePage.module.css';

/**
 * StatoAziendePage — table-first route.
 *
 * Locks (FINAL.md §Slice S5a):
 *  - Single primary table, no KPI row, no summary tiles, no hero.
 *  - Row selection enables the CTA; the CTA label becomes
 *    `Aggiorna {selectedCustomer.name}` (EXACT).
 *  - Modal is the only mutation surface (no detail page, no sticky save bar).
 *  - Explicit empty state and explicit upstream-unavailable state.
 *  - Neutral copy — never leak `Arak`/`Mistra`/`upstream` into user strings.
 */
export function StatoAziendePage() {
  const customersQuery = useCustomers();
  // Prefetched on mount so the modal select is instant when the CTA is clicked.
  const customerStatesQuery = useCustomerStates();

  const [selectedCustomerId, setSelectedCustomerId] = useState<number | null>(null);
  const [modalOpen, setModalOpen] = useState(false);

  const customers = customersQuery.data;

  const selectedCustomer = useMemo(
    () => customers?.find((c) => c.id === selectedCustomerId) ?? null,
    [customers, selectedCustomerId],
  );

  const ctaDisabled = selectedCustomer == null;
  // CTA label is EXACTLY `Aggiorna {selectedCustomer.name}` when a row is
  // selected; otherwise it falls back to a neutral placeholder (and is
  // disabled). This is the locked copy from FINAL.md §Slice S5a.
  const ctaLabel = selectedCustomer
    ? `Aggiorna ${selectedCustomer.name}`
    : 'Aggiorna azienda';

  // Upstream-unavailable is mapped from any 5xx/business-error response.
  // The backend returns 503 when Arak is not configured; it forwards the
  // upstream status code for other failures. User copy stays neutral.
  const upstreamUnavailable = isUpstreamUnavailable(customersQuery.error);

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.pageTitle}>Stato Aziende</h1>
          <p className={styles.pageSubtitle}>
            Seleziona un&apos;azienda per aggiornarne lo stato.
          </p>
        </div>
        <button
          type="button"
          className={styles.primaryBtn}
          disabled={ctaDisabled}
          onClick={() => setModalOpen(true)}
          title={selectedCustomer ? ctaLabel : undefined}
        >
          {ctaLabel}
        </button>
      </div>

      <div className={styles.tableCard}>
        {customersQuery.isLoading ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={6} />
          </div>
        ) : upstreamUnavailable ? (
          <div className={styles.stateBox}>
            <div className={styles.stateIcon}>
              <svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M12 9v4M12 17h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </div>
            <p className={styles.stateTitle}>Servizio non disponibile</p>
            <p className={styles.stateText}>
              Riprova tra qualche minuto.
            </p>
          </div>
        ) : customers == null || customers.length === 0 ? (
          <div className={styles.stateBox}>
            <div className={styles.stateIcon}>
              <svg width="40" height="40" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path
                  d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
                <circle cx="12" cy="7" r="4" stroke="currentColor" strokeWidth="1.5" />
              </svg>
            </div>
            <p className={styles.stateTitle}>Nessuna azienda</p>
            <p className={styles.stateText}>Non ci sono aziende da mostrare.</p>
          </div>
        ) : (
          <div className={styles.tableScroll}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={styles.idCol}>ID</th>
                  <th>Azienda</th>
                </tr>
              </thead>
              <tbody>
                {customers.map((c) => {
                  const isSelected = c.id === selectedCustomerId;
                  return (
                    <tr
                      key={c.id}
                      className={isSelected ? styles.rowSelected : undefined}
                      onClick={() => setSelectedCustomerId(c.id)}
                      aria-selected={isSelected}
                    >
                      <td className={styles.idCol}>{c.id}</td>
                      <td>{c.name}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {selectedCustomer && (
        <UpdateStateModal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          customer={selectedCustomer}
          states={customerStatesQuery.data}
          statesLoading={customerStatesQuery.isLoading}
          statesError={customerStatesQuery.error}
        />
      )}
    </section>
  );
}

// isUpstreamUnavailable treats both 503 (dependency missing) and any other
// structured API failure as a condition the operator should see as
// "service not available". We intentionally do NOT surface the upstream
// hostname or the word "upstream" to the operator (forbidden copy lock).
function isUpstreamUnavailable(error: unknown): boolean {
  if (!(error instanceof ApiError)) return false;
  if (error.status === 503) return true;
  if (error.status >= 500) return true;
  // Backend-forwarded business errors on list endpoints also imply the data
  // cannot be trusted; show the unavailable state rather than a stale table.
  const body = error.body;
  if (typeof body === 'object' && body !== null && 'error' in body) {
    return (body as { error: unknown }).error === 'upstream_error';
  }
  return false;
}
