import { useDeferredValue, useState } from 'react';
import { Button, Icon, SearchInput, Tooltip } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import type { User } from '../../api/users';
import { useCustomers } from '../../hooks/useCustomers';
import { useUsersByCustomer } from '../../hooks/useUsersByCustomer';
import { CustomerSelector } from './CustomerSelector';
import { NuovoAdminModal } from './NuovoAdminModal';
import styles from './GestioneUtenti.module.css';

const HELP_CONTENT = (
  <div className={styles.helpContent}>
    <p>
      In questa applicazione vengono visualizzati tutti gli utenti inseriti
      sul Customer Portal per l&apos;azienda selezionata — da indicare
      tramite la select che trovi qui sotto.
    </p>
    <ul>
      <li>
        Se non trovi utenti in lista, non sono stati aggiunti al portale
        oppure sono stati aggiunti e poi eliminati.
      </li>
      <li>
        La colonna &laquo;Ultimo login&raquo; permette di vedere se è stato
        fatto almeno un login sul portale — funzione attiva da settembre
        &apos;25, quindi non sempre veritiera (lo sarà a tendere).
      </li>
      <li>
        Accesso CP abilitato: se non è presente il flag, l&apos;utente è
        stato disabilitato con un soft delete dal proprio admin.
      </li>
    </ul>
  </div>
);

export function GestioneUtentiPage() {
  const [selectedCustomerId, setSelectedCustomerId] = useState<number | null>(
    null,
  );
  const [modalOpen, setModalOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const customersQuery = useCustomers();
  const usersQuery = useUsersByCustomer(selectedCustomerId);
  const deferredSearch = useDeferredValue(searchQuery);

  const selectionMade = selectedCustomerId != null;
  const allUsers = usersQuery.data ?? [];
  const filteredUsers = filterUsers(allUsers, deferredSearch);
  const hasUsers = allUsers.length > 0;
  const hasSearch = deferredSearch.trim().length > 0;

  function handleCustomerChange(customerId: number | null) {
    setSelectedCustomerId(customerId);
    setSearchQuery('');
  }

  return (
    <section className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.pageTitle}>Gestione utenti per azienda</h1>
        <Tooltip content={HELP_CONTENT} placement="bottom" maxWidth={480}>
          <button
            type="button"
            className={styles.helpButton}
            aria-label="Aiuto"
          >
            <Icon name="info" size={18} />
          </button>
        </Tooltip>
      </header>

      <div className={styles.toolbar}>
        <div className={styles.selector}>
          <span className={styles.selectorLabel}>Azienda</span>
          <CustomerSelector
            customers={customersQuery.data}
            selectedId={selectedCustomerId}
            onChange={handleCustomerChange}
            loading={customersQuery.isLoading}
            error={customersQuery.isError}
          />
        </div>
      </div>

      {!selectionMade ? (
        <div className={styles.emptyState}>
          Seleziona un&apos;azienda per visualizzare gli utenti.
        </div>
      ) : (
        <div className={styles.tableWrapper}>
          <div className={styles.tableTools}>
            {hasUsers ? (
              <SearchInput
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder="Cerca utenti..."
                className={styles.search}
              />
            ) : null}
            <Button
              variant="primary"
              className={styles.newAdminBtn}
              onClick={() => setModalOpen(true)}
            >
              Nuovo Admin
            </Button>
          </div>

          {usersQuery.isLoading ? (
            <div className={styles.loadingState}>Caricamento utenti...</div>
          ) : usersQuery.isError ? (
            <div className={styles.errorState}>
              {formatUsersError(usersQuery.error)}
            </div>
          ) : !hasUsers ? (
            <div className={styles.emptyState}>
              Nessun utente associato a questa azienda.
            </div>
          ) : filteredUsers.length === 0 ? (
            <div className={styles.emptyState}>
              {hasSearch
                ? 'Nessun risultato per la ricerca inserita.'
                : 'Nessun utente associato a questa azienda.'}
            </div>
          ) : (
            <div className={styles.tableScroll}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>email</th>
                    <th>Nome</th>
                    <th>Cognome</th>
                    <th>nome ruolo</th>
                    <th className={styles.checkboxHeader}>
                      Accesso CP abilitato
                    </th>
                    <th>Creato il</th>
                    <th>last_login</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredUsers.map((u) => (
                    <tr key={u.id}>
                      <td>{u.email}</td>
                      <td>{u.first_name}</td>
                      <td>{u.last_name}</td>
                      <td>{u.role.name}</td>
                      <td className={styles.checkboxCell}>
                        <input
                          className={styles.readOnlyCheckbox}
                          type="checkbox"
                          checked={u.enabled}
                          readOnly
                          tabIndex={-1}
                          aria-label={
                            u.enabled
                              ? 'Accesso CP abilitato'
                              : 'Accesso CP non abilitato'
                          }
                        />
                      </td>
                      <td>{u.created}</td>
                      <td>{u.last_login ?? ''}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {selectionMade && (
        <NuovoAdminModal
          open={modalOpen}
          onClose={() => setModalOpen(false)}
          customerId={selectedCustomerId}
        />
      )}
    </section>
  );
}

function formatUsersError(error: unknown): string {
  const fallback = "Qualcosa e' andato storto";
  if (error instanceof ApiError) {
    const message = extractMessage(error.body) ?? fallback;
    return `${error.status} — ${message}`;
  }
  return fallback;
}

function extractMessage(body: unknown): string | undefined {
  if (typeof body === 'object' && body !== null && 'message' in body) {
    const raw = (body as { message: unknown }).message;
    if (typeof raw === 'string' && raw.trim().length > 0) return raw.trim();
  }
  return undefined;
}

function filterUsers(
  users: ReadonlyArray<User>,
  query: string,
) {
  const normalizedQuery = normalize(query);
  if (!normalizedQuery) return users;

  return users.filter((user) =>
    [
      user.email,
      user.first_name,
      user.last_name,
      user.role.name,
      user.created,
      user.last_login ?? '',
    ].some((value) => normalize(value).includes(normalizedQuery)),
  );
}

function normalize(value: string) {
  return value.trim().toLocaleLowerCase();
}
