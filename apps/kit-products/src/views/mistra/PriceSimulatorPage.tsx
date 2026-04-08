import { useEffect, useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Skeleton } from '@mrsmith/ui';
import {
  useMistraCustomers,
  useMistraDiscountedKitDetail,
  useMistraDiscountedKits,
} from './mistraQueries';
import type { RelatedProduct } from './mistraTypes';
import styles from './PriceSimulatorPage.module.css';

interface FlattenedRelatedProduct extends RelatedProduct {
  group_name: string;
  group_required: boolean;
}

export function PriceSimulatorPage() {
  const { data: customerResponse, isLoading: isCustomersLoading, error: customersError } = useMistraCustomers();
  const customers = customerResponse?.items ?? [];
  const [selectedCustomerId, setSelectedCustomerId] = useState<number | null>(null);
  const [selectedKitId, setSelectedKitId] = useState<number | null>(null);

  useEffect(() => {
    const firstCustomer = customers[0];
    if (selectedCustomerId == null && firstCustomer) {
      setSelectedCustomerId(firstCustomer.id);
    }
  }, [customers, selectedCustomerId]);

  const {
    data: discountedKitResponse,
    isLoading: isKitsLoading,
    error: kitsError,
  } = useMistraDiscountedKits(selectedCustomerId);
  const discountedKits = discountedKitResponse?.items ?? [];

  useEffect(() => {
    if (discountedKits.length === 0) {
      setSelectedKitId(null);
      return;
    }
    const firstKit = discountedKits[0];
    if (firstKit && !discountedKits.some((item) => item.id === selectedKitId)) {
      setSelectedKitId(firstKit.id);
    }
  }, [discountedKits, selectedKitId]);

  const {
    data: detail,
    isLoading: isDetailLoading,
    error: detailError,
  } = useMistraDiscountedKitDetail(selectedCustomerId, selectedKitId);

  const selectedKit = discountedKits.find((kit) => kit.id === selectedKitId) ?? null;
  const relatedProducts = useMemo<FlattenedRelatedProduct[]>(
    () =>
      (detail?.related_products ?? []).flatMap((group) =>
        group.products.map((product) => ({
          ...product,
          group_name: group.group_name,
          group_required: group.required,
        })),
      ),
    [detail],
  );

  return (
    <section className={styles.page}>
      <header className={styles.hero}>
        <div>
          <p className={styles.eyebrow}>Phase 6</p>
          <h1>Simulatore prezzi</h1>
          <p className={styles.lead}>
            Vista read-only sui kit scontati per cliente, con flatten dei prodotti correlati dal
            payload Mistra.
          </p>
        </div>
        <label className={styles.customerField}>
          <span>Cliente</span>
          <select
            value={selectedCustomerId ?? 0}
            disabled={isCustomersLoading}
            onChange={(event) => setSelectedCustomerId(Number(event.target.value) || null)}
          >
            <option value={0}>Seleziona cliente</option>
            {customers.map((customer) => (
              <option key={customer.id} value={customer.id}>
                {customer.name}
              </option>
            ))}
          </select>
        </label>
      </header>

      {customersError ? <EmptyState title="Impossibile caricare i clienti" text={getErrorMessage(customersError, 'Riprova tra poco.')} /> : null}
      {isCustomersLoading ? <Skeleton rows={4} /> : null}

      <section className={styles.layout}>
        <article className={styles.panel}>
          <div className={styles.panelHeader}>
            <h2>Kit scontati</h2>
            <span>{discountedKits.length} elementi</span>
          </div>

          {isKitsLoading ? <Skeleton rows={8} /> : null}
          {kitsError ? <EmptyState title="Impossibile caricare i kit" text={getErrorMessage(kitsError, 'Riprova tra poco.')} /> : null}
          {!isKitsLoading && !kitsError ? (
            discountedKits.length === 0 ? (
              <EmptyState title="Nessun kit disponibile" text="Seleziona un cliente con almeno un kit scontato." />
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Kit</th>
                      <th>NRC</th>
                      <th>MRC</th>
                      <th>Categoria</th>
                    </tr>
                  </thead>
                  <tbody>
                    {discountedKits.map((kit) => (
                      <tr
                        key={kit.id}
                        className={selectedKitId === kit.id ? styles.rowSelected : ''}
                        onClick={() => setSelectedKitId(kit.id)}
                      >
                        <td>
                          <strong>{kit.internal_name}</strong>
                          <small>#{kit.id}</small>
                        </td>
                        <td>{kit.base_price?.nrc ?? 'n/d'}</td>
                        <td>{kit.base_price?.mrc ?? 'n/d'}</td>
                        <td>{kit.category}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          ) : null}
        </article>

        <article className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2>Prodotti correlati</h2>
              <span>{selectedKit ? `Kit selezionato: ${selectedKit.internal_name}` : 'Seleziona un kit'}</span>
            </div>
          </div>

          {isDetailLoading ? <Skeleton rows={8} /> : null}
          {detailError ? <EmptyState title="Impossibile caricare il dettaglio kit" text={getErrorMessage(detailError, 'Riprova tra poco.')} /> : null}
          {!isDetailLoading && !detailError ? (
            relatedProducts.length === 0 ? (
              <EmptyState title="Nessun prodotto correlato" text="Il kit selezionato non espone prodotti correlati nel payload Mistra." />
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Gruppo</th>
                      <th>ID</th>
                      <th>Titolo</th>
                      <th>NRC</th>
                      <th>MRC</th>
                      <th>Min</th>
                      <th>Max</th>
                    </tr>
                  </thead>
                  <tbody>
                    {relatedProducts.map((product) => (
                      <tr key={`${product.group_name}-${product.id}`}>
                        <td>
                          <strong>{product.group_name}</strong>
                          <small>{product.group_required ? 'Obbligatorio' : 'Opzionale'}</small>
                        </td>
                        <td>{product.id}</td>
                        <td>{product.title}</td>
                        <td>{product.price?.nrc ?? 'n/d'}</td>
                        <td>{product.price?.mrc ?? 'n/d'}</td>
                        <td>{product.min_qty}</td>
                        <td>{product.max_qty}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          ) : null}
        </article>
      </section>
    </section>
  );
}

function EmptyState({ title, text }: { title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <p className={styles.emptyTitle}>{title}</p>
      <p className={styles.emptyText}>{text}</p>
    </div>
  );
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body && 'error' in error.body) {
    const message = error.body.error;
    if (typeof message === 'string') {
      return message;
    }
  }
  if (error instanceof Error) {
    return error.message;
  }
  return fallback;
}
