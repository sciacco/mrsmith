import { useState } from 'react';
import { Button } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import { useCustomers } from '../../hooks/useCustomers';
import { useUsersByCustomer } from '../../hooks/useUsersByCustomer';
import { useOptionalAuth } from '../../hooks/useOptionalAuth';
import { CustomerSelector } from './CustomerSelector';
import { NuovoAdminModal } from './NuovoAdminModal';
import styles from './GestioneUtenti.module.css';

export function GestioneUtentiPage() {
  const { user } = useOptionalAuth();
  const [selectedCustomerId, setSelectedCustomerId] = useState<number | null>(
    null,
  );
  const [modalOpen, setModalOpen] = useState(false);

  const customersQuery = useCustomers();
  const usersQuery = useUsersByCustomer(selectedCustomerId);

  const operatorLabel = user?.name ?? user?.email ?? '';
  const greeting = `Ciao ${operatorLabel}, in questa applicazione vengono visualizzati tutti gli utenti inseriti per l'azienda selezionata - da indicare tramite la select`;

  const selectionMade = selectedCustomerId != null;

  return (
    <section className={styles.page}>
      <p className={styles.greeting}>{greeting}</p>

      <div className={styles.toolbar}>
        <div className={styles.selector}>
          <span className={styles.selectorLabel}>Azienda</span>
          <CustomerSelector
            customers={customersQuery.data}
            selectedId={selectedCustomerId}
            onChange={setSelectedCustomerId}
            loading={customersQuery.isLoading}
            error={customersQuery.isError}
          />
        </div>
        <div className={styles.ctaSlot}>
          <Button
            variant="primary"
            disabled={!selectionMade}
            onClick={() => setModalOpen(true)}
          >
            Nuovo Admin
          </Button>
        </div>
      </div>

      {!selectionMade ? (
        <div className={styles.emptyState}>
          Seleziona un'azienda per visualizzare gli utenti.
        </div>
      ) : usersQuery.isLoading ? (
        <div className={styles.loadingState}>Caricamento utenti...</div>
      ) : usersQuery.isError ? (
        <div className={styles.errorState}>
          {formatUsersError(usersQuery.error)}
        </div>
      ) : !usersQuery.data || usersQuery.data.length === 0 ? (
        <div className={styles.emptyState}>
          Nessun utente associato a questa azienda.
        </div>
      ) : (
        <div className={styles.tableWrapper}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Cognome</th>
                <th>Em@il</th>
                <th>Admin</th>
              </tr>
            </thead>
            <tbody>
              {usersQuery.data.map((u) => (
                <tr key={u.id}>
                  <td>{u.nome}</td>
                  <td>{u.cognome}</td>
                  <td>{u.email}</td>
                  <td>{u.is_admin ? 'Sì' : 'No'}</td>
                </tr>
              ))}
            </tbody>
          </table>
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
