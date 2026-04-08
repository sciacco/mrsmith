import { useEffect, useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { SearchInput, SingleSelect, Skeleton } from '@mrsmith/ui';
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
  const [kitSearch, setKitSearch] = useState('');

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

  const filteredKits = useMemo(() => {
    const q = kitSearch.trim().toLowerCase();
    if (!q) return discountedKits;
    return discountedKits.filter((kit) =>
      kit.internal_name.toLowerCase().includes(q) ||
      kit.category.toLowerCase().includes(q) ||
      String(kit.id).includes(q),
    );
  }, [discountedKits, kitSearch]);

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
      <header className={styles.pageHeader}>
        <div>
          <h1>Simulatore prezzi</h1>
          <p className={styles.subtitle}>{discountedKits.length} kit scontati</p>
        </div>
        <div className={styles.customerSelect}>
          <span>Cliente</span>
          <SingleSelect<number>
            options={customers.map((c) => ({ value: c.id, label: c.name }))}
            selected={selectedCustomerId}
            onChange={setSelectedCustomerId}
            placeholder="Seleziona cliente..."
          />
        </div>
      </header>

      {customersError ? <EmptyState title="Impossibile caricare i clienti" text={getErrorMessage(customersError, 'Riprova tra poco.')} /> : null}
      {isCustomersLoading ? <Skeleton rows={4} /> : null}

      <section className={styles.layout}>
        {/* Left — kit list */}
        <article className={styles.panel}>
          <div className={styles.panelToolbar}>
            <h2>Kit scontati</h2>
            <SearchInput value={kitSearch} onChange={setKitSearch} placeholder="Cerca..." className={styles.searchWrap} />
          </div>

          {isKitsLoading ? <Skeleton rows={8} /> : null}
          {kitsError ? <EmptyState title="Impossibile caricare i kit" text={getErrorMessage(kitsError, 'Riprova tra poco.')} /> : null}
          {!isKitsLoading && !kitsError ? (
            filteredKits.length === 0 ? (
              <EmptyState title={kitSearch ? 'Nessun risultato' : 'Nessun kit disponibile'} text={kitSearch ? `Nessun kit corrisponde a "${kitSearch}".` : 'Seleziona un cliente con almeno un kit scontato.'} />
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th className={styles.accentCell} />
                      <th>Kit</th>
                      <th>NRC</th>
                      <th>MRC</th>
                      <th>Categoria</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredKits.map((kit, index) => (
                      <tr
                        key={kit.id}
                        className={selectedKitId === kit.id ? styles.rowSelected : ''}
                        style={{ animationDelay: `${index * 0.03}s` }}
                        onClick={() => setSelectedKitId(kit.id)}
                      >
                        <td className={styles.accentCell}><div className={styles.accentBar} /></td>
                        <td>
                          <strong>{kit.internal_name}</strong>
                          <small>#{kit.id}</small>
                        </td>
                        <td className={styles.mono}>{kit.base_price?.nrc ?? 'n/d'}</td>
                        <td className={styles.mono}>{kit.base_price?.mrc ?? 'n/d'}</td>
                        <td>{kit.category}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          ) : null}
        </article>

        {/* Right — related products (sticky) */}
        <article className={styles.panelSticky}>
          <div className={styles.panelToolbar}>
            <div>
              <h2>Prodotti correlati</h2>
              {selectedKit ? <small>{selectedKit.internal_name}</small> : null}
            </div>
          </div>

          {isDetailLoading ? <Skeleton rows={8} /> : null}
          {detailError ? <EmptyState title="Impossibile caricare il dettaglio" text={getErrorMessage(detailError, 'Riprova tra poco.')} /> : null}
          {!isDetailLoading && !detailError ? (
            relatedProducts.length === 0 ? (
              <EmptyState title="Nessun prodotto correlato" text="Seleziona un kit dalla lista." />
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
                    {relatedProducts.map((product, index) => (
                      <tr key={`${product.group_name}-${product.id}`} style={{ animationDelay: `${index * 0.03}s` }}>
                        <td>
                          <strong>{product.group_name}</strong>
                          <small>{product.group_required ? 'Obbligatorio' : 'Opzionale'}</small>
                        </td>
                        <td className={styles.mono}>{product.id}</td>
                        <td>{product.title}</td>
                        <td className={styles.mono}>{product.price?.nrc ?? 'n/d'}</td>
                        <td className={styles.mono}>{product.price?.mrc ?? 'n/d'}</td>
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
      <div className={styles.emptyIcon}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
          <path d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0-3-3m3 3 3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" />
        </svg>
      </div>
      <p className={styles.emptyTitle}>{title}</p>
      <p className={styles.emptyText}>{text}</p>
    </div>
  );
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof ApiError && typeof error.body === 'object' && error.body && 'error' in error.body) {
    const message = error.body.error;
    if (typeof message === 'string') return message;
  }
  if (error instanceof Error) return error.message;
  return fallback;
}
