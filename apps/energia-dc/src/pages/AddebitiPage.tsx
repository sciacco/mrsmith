import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { SingleSelect } from '@mrsmith/ui';
import { useBillingCharges, useCustomers } from '../api/queries';
import type { BillingCharge } from '../api/types';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import { ViewState } from '../components/ViewState';
import { downloadCsv, formatAmpere, formatCount, formatDate, formatMoneyEUR, formatNumber } from '../utils/format';
import styles from './shared.module.css';

function isServiceUnavailable(error: unknown) {
  return error instanceof ApiError && error.status === 503;
}

function exportBillingCsv(customerName: string, rows: BillingCharge[]) {
  downloadCsv(
    `addebiti_${customerName.replaceAll(/\s+/g, '-').toLowerCase()}.csv`,
    [
      'Periodo iniziale',
      'Periodo finale',
      'Ampere',
      'Eccedenti',
      'Importo',
      'PUN',
      'Coefficiente',
      'Fisso CU',
      'Importo eccedenti',
    ],
    rows.map((row) => [
      formatDate(row.startPeriod),
      formatDate(row.endPeriod),
      formatAmpere(row.ampere),
      formatNumber(row.eccedenti),
      formatMoneyEUR(row.amount),
      formatNumber(row.pun),
      formatNumber(row.coefficiente),
      formatNumber(row.fissoCu),
      formatMoneyEUR(row.importoEccedenti),
    ]),
  );
}

export function AddebitiPage() {
  const [customerId, setCustomerId] = useState<number | null>(null);
  const customersQ = useCustomers();
  const billingQ = useBillingCharges(customerId);

  const customerOptions = customersQ.data ?? [];
  const customerName = customerOptions.find((item) => item.id === customerId)?.name ?? 'cliente';

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.title}>Addebiti</h1>
          <p className={styles.subtitle}>
            Seleziona un cliente per consultare il dettaglio degli addebiti variabili e, quando presenti, esportare il dataset visibile in CSV.
          </p>
        </div>
      </div>

      {customersQ.error && isServiceUnavailable(customersQ.error) ? <ServiceUnavailable /> : null}
      {customersQ.error && !isServiceUnavailable(customersQ.error) ? (
        <ViewState
          title="Clienti non disponibili"
          message="Non e stato possibile caricare il catalogo clienti per la sezione addebiti."
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
            </div>
          </section>

          {customerId === null ? (
            <ViewState
              title="Selezione richiesta"
              message="Scegli un cliente per visualizzare la tabella degli addebiti."
            />
          ) : null}

          {customerId !== null && billingQ.error && isServiceUnavailable(billingQ.error) ? <ServiceUnavailable /> : null}
          {customerId !== null && billingQ.error && !isServiceUnavailable(billingQ.error) ? (
            <ViewState
              title="Addebiti non disponibili"
              message="La tabella degli addebiti non e al momento disponibile per il cliente selezionato."
              tone="error"
            />
          ) : null}

          {customerId !== null && !billingQ.error ? (
            <section className={styles.card}>
              <div className={styles.sectionHeader}>
                <div>
                  <h2 className={styles.sectionTitle}>{customerName}</h2>
                  <div className={styles.sectionMeta}>
                    {billingQ.isLoading
                      ? 'Caricamento in corso'
                      : formatCount(billingQ.data?.length ?? 0, 'addebito', 'addebiti')}
                  </div>
                </div>
                {!billingQ.isLoading && (billingQ.data?.length ?? 0) > 0 ? (
                  <button
                    type="button"
                    className={styles.btnSecondary}
                    onClick={() => exportBillingCsv(customerName, billingQ.data ?? [])}
                  >
                    Esporta CSV
                  </button>
                ) : null}
              </div>

              {billingQ.isLoading ? (
                <ViewState title="Caricamento in corso" message="Gli addebiti del cliente selezionato si stanno aggiornando." />
              ) : null}

              {!billingQ.isLoading && (billingQ.data ?? []).length === 0 ? (
                <ViewState
                  title="Nessun addebito presente"
                  message="Il cliente selezionato non restituisce righe di addebito variabile."
                />
              ) : null}

              {!billingQ.isLoading && (billingQ.data ?? []).length > 0 ? (
                <div className={styles.tableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        <th>Periodo iniziale</th>
                        <th>Periodo finale</th>
                        <th className={styles.alignRight}>Ampere</th>
                        <th className={styles.alignRight}>Eccedenti</th>
                        <th className={styles.alignRight}>Importo</th>
                        <th className={styles.alignRight}>PUN</th>
                        <th className={styles.alignRight}>Coefficiente</th>
                        <th className={styles.alignRight}>Fisso CU</th>
                        <th className={styles.alignRight}>Importo eccedenti</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(billingQ.data ?? []).map((row) => (
                        <tr key={row.id}>
                          <td>{formatDate(row.startPeriod)}</td>
                          <td>{formatDate(row.endPeriod)}</td>
                          <td className={styles.alignRight}>{formatAmpere(row.ampere)}</td>
                          <td className={styles.alignRight}>{formatNumber(row.eccedenti)}</td>
                          <td className={styles.alignRight}>{formatMoneyEUR(row.amount)}</td>
                          <td className={styles.alignRight}>{formatNumber(row.pun)}</td>
                          <td className={styles.alignRight}>{formatNumber(row.coefficiente)}</td>
                          <td className={styles.alignRight}>{formatNumber(row.fissoCu)}</td>
                          <td className={styles.alignRight}>{formatMoneyEUR(row.importoEccedenti)}</td>
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
