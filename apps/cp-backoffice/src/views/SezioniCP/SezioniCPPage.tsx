import { useDeferredValue, useMemo, useState, useEffect } from 'react';
import { SearchInput, Skeleton, Drawer, ToggleSwitch, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCustomers, useUpdateCustomerVariables, type Customer } from '@mrsmith/features';
import styles from './SezioniCPPage.module.css';

/**
 * SezioniCPPage — gestisce la visibilità delle sezioni del Customer Portal per ciascun cliente.
 */
export function SezioniCPPage() {
  const { toast } = useToast();
  const customersQuery = useCustomers();
  const updateVariables = useUpdateCustomerVariables();

  const [selectedCustomerId, setSelectedCustomerId] = useState<number | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const deferredSearch = useDeferredValue(searchQuery);

  const [serviziGestitiEnabled, setServiziGestitiEnabled] = useState<boolean>(true);

  const customers = customersQuery.data;

  const selectedCustomer = useMemo(
    () => customers?.find((c) => c.id === selectedCustomerId) ?? null,
    [customers, selectedCustomerId],
  );

  // Inizializza lo stato del toggle quando cambia il cliente selezionato
  useEffect(() => {
    if (selectedCustomer) {
      setServiziGestitiEnabled(isSectionVisible(selectedCustomer, 'servizi_gestiti'));
    }
  }, [selectedCustomer]);

  const filteredCustomers = useMemo(
    () => filterCustomers(customers, deferredSearch),
    [customers, deferredSearch],
  );
  const hasSearch = deferredSearch.trim().length > 0;



  const upstreamUnavailable = isUpstreamUnavailable(customersQuery.error);

  function handleSave() {
    if (!selectedCustomer) return;

    // Preserva tutti gli altri oggetti presenti in variables non direttamente modificati
    const otherVariables = (selectedCustomer.variables ?? []).filter(
      (v) => !(v.action === 'hide_pages' && v.resource === 'servizi_gestiti'),
    );

    const nextVariables = [...otherVariables];

    // Se l'interruttore è disattivato (sezione nascosta), aggiungiamo l'oggetto hide_pages
    if (!serviziGestitiEnabled) {
      nextVariables.push({
        action: 'hide_pages',
        resource: 'servizi_gestiti',
      });
    }

    // Regola importante: se l'array è vuoto, passiamo null anziché un array vuoto
    const payloadVariables = nextVariables.length > 0 ? nextVariables : null;

    updateVariables.mutate(
      { customerId: selectedCustomer.id, variables: payloadVariables },
      {
        onSuccess: () => {
          toast('Configurazione salvata con successo.', 'success');
          setDrawerOpen(false);
        },
        onError: (error) => {
          let msg = 'Impossibile salvare la configurazione.';
          if (error instanceof ApiError) {
            const body = error.body;
            if (typeof body === 'object' && body !== null && 'message' in body) {
              const apiMsg = (body as { message: unknown }).message;
              if (typeof apiMsg === 'string' && apiMsg.trim().length > 0) {
                msg = `${error.status} — ${apiMsg}`;
              }
            }
          }
          toast(msg, 'error');
        },
      },
    );
  }

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.pageTitle}>Sezioni CP</h1>
          <p className={styles.pageSubtitle}>
            Abilita o disabilita la visibilità delle sezioni del Customer Portal per ogni cliente.
          </p>
        </div>
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
            <p className={styles.stateText}>Riprova tra qualche minuto.</p>
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
            <p className={styles.stateTitle}>Nessun cliente</p>
            <p className={styles.stateText}>Non ci sono clienti da mostrare.</p>
          </div>
        ) : (
          <>
            <div className={styles.tableTools}>
              <SearchInput
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder="Cerca per ID, ragione sociale o stato..."
                className={styles.search}
              />
            </div>
            {filteredCustomers.length === 0 ? (
              <div className={styles.stateBox}>
                <p className={styles.stateTitle}>Nessun risultato</p>
                <p className={styles.stateText}>
                  {hasSearch
                    ? 'Nessun cliente corrisponde alla ricerca.'
                    : 'Non ci sono clienti da mostrare.'}
                </p>
              </div>
            ) : (
              <div className={styles.tableScroll}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th className={styles.idCol}>ID</th>
                      <th>Ragione sociale</th>
                      <th className={styles.statusCol}>Servizi Gestiti</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredCustomers.map((c) => {
                      const isSelected = c.id === selectedCustomerId;
                      const active = isSectionVisible(c, 'servizi_gestiti');
                      return (
                        <tr
                          key={c.id}
                          className={isSelected ? styles.rowSelected : undefined}
                          onClick={() => {
                            setSelectedCustomerId(c.id);
                            setDrawerOpen(true);
                          }}
                          aria-selected={isSelected}
                        >
                          <td className={styles.idCol}>{c.id}</td>
                          <td>{c.name}</td>
                          <td className={styles.statusCol}>
                            <span className={active ? styles.badgeActive : styles.badgeInactive}>
                              {active ? 'Visibile' : 'Nascosto'}
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </div>

      {selectedCustomer && (
        <Drawer
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          title="Gestisci Sezioni Customer Portal"
          subtitle={`Cliente: ${selectedCustomer.name} (ID: ${selectedCustomer.id})`}
          footer={
            <div className={styles.drawerFooter}>
              <button
                type="button"
                className={styles.btnSecondary}
                onClick={() => setDrawerOpen(false)}
                disabled={updateVariables.isPending}
              >
                Annulla
              </button>
              <button
                type="button"
                className={styles.primaryBtn}
                onClick={handleSave}
                disabled={updateVariables.isPending}
              >
                {updateVariables.isPending ? 'Salvataggio...' : 'Salva'}
              </button>
            </div>
          }
        >
          <div className={styles.drawerBody}>
            <div className={styles.sectionItem}>
              <div className={styles.sectionInfo}>
                <span className={styles.sectionLabel}>Servizi Gestiti</span>
                <span className={styles.sectionDesc}>
                  Abilita o disabilita la visualizzazione della sezione Servizi Gestiti nel portale del cliente.
                </span>
              </div>
              <ToggleSwitch
                id="toggle-servizi-gestiti"
                checked={serviziGestitiEnabled}
                onChange={setServiziGestitiEnabled}
                disabled={updateVariables.isPending}
              />
            </div>
          </div>
        </Drawer>
      )}
    </section>
  );
}

function isSectionVisible(customer: Customer, resourceId: string): boolean {
  const variables = customer.variables;
  if (!variables) return true;
  const hasHide = variables.some((v) => v.action === 'hide_pages' && v.resource === resourceId);
  return !hasHide;
}

function filterCustomers(
  customers: ReadonlyArray<Customer> | undefined,
  query: string,
): ReadonlyArray<Customer> {
  if (!customers) return [];
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) return customers;
  return customers.filter((c) => {
    const activeLabel = isSectionVisible(c, 'servizi_gestiti') ? 'visibile' : 'nascosto';
    return [
      String(c.id),
      c.name,
      c.group?.name ?? '',
      c.language ?? '',
      activeLabel,
    ].some((value) => value.toLocaleLowerCase().includes(needle));
  });
}

function isUpstreamUnavailable(error: unknown): boolean {
  if (!(error instanceof ApiError)) return false;
  if (error.status === 503) return true;
  if (error.status >= 500) return true;
  const body = error.body;
  if (typeof body === 'object' && body !== null && 'error' in body) {
    return (body as { error: unknown }).error === 'upstream_error';
  }
  return false;
}
