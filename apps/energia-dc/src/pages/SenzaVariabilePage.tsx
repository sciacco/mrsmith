import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useNoVariableCustomers, useNoVariableRacks } from '../api/queries';
import type { LookupItem } from '../api/types';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import { ViewState } from '../components/ViewState';
import { formatCount, formatMaybeText } from '../utils/format';
import styles from './shared.module.css';

function isServiceUnavailable(error: unknown) {
  return error instanceof ApiError && error.status === 503;
}

export function SenzaVariabilePage() {
  const [selectedCustomer, setSelectedCustomer] = useState<LookupItem | null>(null);
  const customersQ = useNoVariableCustomers();
  const racksQ = useNoVariableRacks(selectedCustomer?.id ?? null);

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.title}>Senza variabile</h1>
          <p className={styles.subtitle}>
            Consulta i clienti che non hanno addebito variabile e apri il dettaglio solo quando serve, senza preselezioni automatiche.
          </p>
        </div>
      </div>

      {customersQ.error && isServiceUnavailable(customersQ.error) ? <ServiceUnavailable /> : null}
      {customersQ.error && !isServiceUnavailable(customersQ.error) ? (
        <ViewState
          title="Audit non disponibile"
          message="L&apos;elenco clienti senza addebito variabile non e al momento raggiungibile."
          tone="error"
        />
      ) : null}

      {!customersQ.error ? (
        <div className={styles.masterDetail}>
          <section className={styles.card}>
            <div className={styles.sectionHeader}>
              <h2 className={styles.sectionTitle}>Rack senza addebito variabile</h2>
              <div className={styles.sectionMeta}>
                {customersQ.isLoading
                  ? 'Caricamento in corso'
                  : formatCount(customersQ.data?.length ?? 0, 'cliente', 'clienti')}
              </div>
            </div>

            {customersQ.isLoading ? (
              <ViewState
                title="Caricamento in corso"
                message="L&apos;elenco clienti senza variabile si sta aggiornando."
              />
            ) : null}

            {!customersQ.isLoading && (customersQ.data ?? []).length === 0 ? (
              <ViewState
                title="Nessun cliente disponibile"
                message="Non risultano clienti con rack senza addebito variabile nel perimetro corrente."
              />
            ) : null}

            {!customersQ.isLoading && (customersQ.data ?? []).length > 0 ? (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Cliente</th>
                    </tr>
                  </thead>
                  <tbody>
                    {(customersQ.data ?? []).map((customer) => (
                      <tr
                        key={customer.id}
                        className={`${styles.clickableRow} ${selectedCustomer?.id === customer.id ? styles.selectedRow : ''}`}
                        onClick={() => setSelectedCustomer(customer)}
                      >
                        <td>{customer.name}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : null}
          </section>

          <section className={styles.card}>
            <div className={styles.sectionHeader}>
              <div>
                <h2 className={styles.sectionTitle}>Dettaglio rack</h2>
                <div className={styles.sectionMeta}>
                  {selectedCustomer ? selectedCustomer.name : 'Seleziona un cliente'}
                </div>
              </div>
              {selectedCustomer && !racksQ.isLoading && !racksQ.error ? (
                <div className={styles.sectionMeta}>
                  {formatCount(racksQ.data?.length ?? 0, 'rack', 'rack')}
                </div>
              ) : null}
            </div>

            {selectedCustomer === null ? (
              <ViewState
                title="Dettaglio in attesa"
                message="Seleziona una riga cliente per caricare il relativo elenco rack."
              />
            ) : null}

            {selectedCustomer !== null && racksQ.error && isServiceUnavailable(racksQ.error) ? <ServiceUnavailable /> : null}
            {selectedCustomer !== null && racksQ.error && !isServiceUnavailable(racksQ.error) ? (
              <ViewState
                title="Dettaglio non disponibile"
                message="Il dettaglio rack del cliente selezionato non e stato caricato correttamente."
                tone="error"
              />
            ) : null}

            {selectedCustomer !== null && !racksQ.error ? (
              <>
                {racksQ.isLoading ? (
                  <ViewState
                    title="Caricamento in corso"
                    message="Il dettaglio rack del cliente selezionato si sta aggiornando."
                  />
                ) : null}

                {!racksQ.isLoading && (racksQ.data ?? []).length === 0 ? (
                  <ViewState
                    title="Nessun rack presente"
                    message="Il cliente selezionato non restituisce rack senza addebito variabile."
                  />
                ) : null}

                {!racksQ.isLoading && (racksQ.data ?? []).length > 0 ? (
                  <div className={styles.tableWrap}>
                    <table className={styles.table}>
                      <thead>
                        <tr>
                          <th>Rack</th>
                          <th>Edificio</th>
                          <th>Sala</th>
                          <th>Posizione</th>
                          <th>Codice ordine</th>
                          <th>Seriale</th>
                        </tr>
                      </thead>
                      <tbody>
                        {(racksQ.data ?? []).map((rack) => (
                          <tr key={rack.id}>
                            <td>{rack.name}</td>
                            <td>{rack.buildingName}</td>
                            <td>{rack.roomName}</td>
                            <td>{formatMaybeText(rack.position)}</td>
                            <td>{formatMaybeText(rack.orderCode)}</td>
                            <td>{formatMaybeText(rack.serialNumber)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : null}
              </>
            ) : null}
          </section>
        </div>
      ) : null}
    </div>
  );
}
