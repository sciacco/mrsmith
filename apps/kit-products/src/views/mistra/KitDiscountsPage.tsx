import { useEffect, useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCustomerGroups } from '../../api/queries';
import {
  useMistraKitDiscounts,
  useMistraKits,
  useUpsertMistraKitDiscount,
} from './mistraQueries';
import type { DiscountValue, KitDiscountCreateRequest, KitDiscountEntry } from './mistraTypes';
import styles from './KitDiscountsPage.module.css';

interface DiscountModalState {
  customer_group_id: number;
  sellable: boolean;
  use_int_rounding: boolean;
  mrc: DiscountValue;
  nrc: DiscountValue;
}

export function KitDiscountsPage() {
  const { toast } = useToast();
  const { data: kitResponse, isLoading: isKitsLoading, error: kitsError } = useMistraKits();
  const { data: customerGroups } = useCustomerGroups();
  const upsertDiscount = useUpsertMistraKitDiscount();

  const kits = kitResponse?.items ?? [];
  const [selectedKitId, setSelectedKitId] = useState<number | null>(null);
  const [editingDiscount, setEditingDiscount] = useState<KitDiscountEntry | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [modalState, setModalState] = useState<DiscountModalState>(emptyDiscountModalState());

  useEffect(() => {
    const firstKit = kits[0];
    if (selectedKitId == null && firstKit) {
      setSelectedKitId(firstKit.id);
    }
  }, [kits, selectedKitId]);

  const {
    data: discountResponse,
    isLoading: isDiscountsLoading,
    error: discountsError,
    refetch: refetchDiscounts,
  } = useMistraKitDiscounts(selectedKitId);

  const discounts = discountResponse?.items ?? [];
  const selectedKit = kits.find((kit) => kit.id === selectedKitId) ?? null;

  const unassignedGroups = useMemo(() => {
    const assigned = new Set(discounts.map((item) => item.customer_group.id));
    return (customerGroups ?? []).filter((group) => !assigned.has(group.id));
  }, [customerGroups, discounts]);

  async function handleSubmit() {
    if (selectedKitId == null || modalState.customer_group_id <= 0) {
      toast('Seleziona kit e gruppo cliente prima di salvare', 'error');
      return;
    }

    const payload: KitDiscountCreateRequest = {
      kit_id: selectedKitId,
      customer_group_id: modalState.customer_group_id,
      sellable: modalState.sellable,
      use_int_rounding: modalState.use_int_rounding,
      mrc: normalizeDiscount(modalState.mrc),
      nrc: normalizeDiscount(modalState.nrc),
    };

    try {
      await upsertDiscount.mutateAsync(payload);
      await refetchDiscounts();
      setModalOpen(false);
      setEditingDiscount(null);
      setModalState(emptyDiscountModalState());
      toast(editingDiscount ? 'Sconto aggiornato' : 'Sconto creato', 'success');
    } catch (error) {
      toast(getErrorMessage(error, 'Impossibile salvare lo sconto kit'), 'error');
    }
  }

  function openCreateModal() {
    setEditingDiscount(null);
    setModalState(emptyDiscountModalState());
    setModalOpen(true);
  }

  function openEditModal(entry: KitDiscountEntry) {
    setEditingDiscount(entry);
    setModalState({
      customer_group_id: entry.customer_group.id,
      sellable: entry.sellable,
      use_int_rounding: entry.use_int_rounding,
      mrc: { ...entry.mrc },
      nrc: { ...entry.nrc },
    });
    setModalOpen(true);
  }

  function handleGroupSelect(groupId: number) {
    const group = customerGroups?.find((item) => item.id === groupId);
    const defaultPercentage =
      group?.base_discount == null ? '0' : formatPercentage(group.base_discount);
    setModalState((current) => ({
      ...current,
      customer_group_id: groupId,
      mrc: { percentage: defaultPercentage, sign: '-' },
      nrc: { percentage: defaultPercentage, sign: '-' },
    }));
  }

  function handleMrcChange(patch: Partial<DiscountValue>) {
    setModalState((current) => {
      const nextMrc = normalizeDiscount({ ...current.mrc, ...patch });
      const shouldMirrorNrc =
        !editingDiscount &&
        current.nrc.percentage === current.mrc.percentage &&
        current.nrc.sign === current.mrc.sign;
      return {
        ...current,
        mrc: nextMrc,
        nrc: shouldMirrorNrc ? { ...nextMrc } : current.nrc,
      };
    });
  }

  return (
    <section className={styles.page}>
      <header className={styles.hero}>
        <div>
          <p className={styles.eyebrow}>Phase 6</p>
          <h1>Sconti kit</h1>
          <p className={styles.lead}>
            Master-detail sui kit Mistra con upsert delle regole per customer group e auto-fill del
            base discount.
          </p>
        </div>
        <div className={styles.heroActions}>
          <button type="button" className={styles.primaryButton} onClick={openCreateModal} disabled={selectedKitId == null}>
            Nuovo sconto
          </button>
          <button type="button" className={styles.secondaryButton} onClick={() => void refetchDiscounts()} disabled={selectedKitId == null}>
            Refresh
          </button>
        </div>
      </header>

      <section className={styles.layout}>
        <article className={styles.panel}>
          <div className={styles.panelHeader}>
            <h2>Kit</h2>
            <span>{kits.length} elementi</span>
          </div>
          {isKitsLoading ? <Skeleton rows={8} /> : null}
          {kitsError ? <EmptyState title="Impossibile caricare i kit" text={getErrorMessage(kitsError, 'Riprova tra poco.')} /> : null}
          {!isKitsLoading && !kitsError ? (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Kit</th>
                    <th>Categoria</th>
                  </tr>
                </thead>
                <tbody>
                  {kits.map((kit) => (
                    <tr
                      key={kit.id}
                      className={selectedKitId === kit.id ? styles.rowSelected : ''}
                      onClick={() => setSelectedKitId(kit.id)}
                    >
                      <td>
                        <strong>{kit.internal_name}</strong>
                        <small>#{kit.id}</small>
                      </td>
                      <td>{kit.category}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}
        </article>

        <article className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2>Gruppi cliente</h2>
              <span>{selectedKit ? `Kit selezionato: ${selectedKit.internal_name}` : 'Seleziona un kit'}</span>
            </div>
          </div>
          {isDiscountsLoading ? <Skeleton rows={8} /> : null}
          {discountsError ? (
            <EmptyState
              title="Impossibile caricare gli sconti"
              text={getErrorMessage(discountsError, 'Riprova tra poco.')}
            />
          ) : null}
          {!isDiscountsLoading && !discountsError ? (
            discounts.length === 0 ? (
              <EmptyState
                title="Nessuna regola configurata"
                text="Aggiungi il primo sconto per associare un customer group al kit."
              />
            ) : (
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Gruppo</th>
                      <th>Sellable</th>
                      <th>Rounding</th>
                      <th>MRC</th>
                      <th>NRC</th>
                    </tr>
                  </thead>
                  <tbody>
                    {discounts.map((entry) => (
                      <tr key={entry.customer_group.id} onClick={() => openEditModal(entry)}>
                        <td>{entry.customer_group.name}</td>
                        <td>{entry.sellable ? 'Si' : 'No'}</td>
                        <td>{entry.use_int_rounding ? 'Si' : 'No'}</td>
                        <td>{entry.mrc.sign}{entry.mrc.percentage}%</td>
                        <td>{entry.nrc.sign}{entry.nrc.percentage}%</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )
          ) : null}
        </article>
      </section>

      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title={editingDiscount ? 'Modifica sconto kit' : 'Nuovo sconto kit'}
      >
        <div className={styles.modalBody}>
          <div className={styles.formGrid}>
            <label className={styles.field}>
              <span>Gruppo cliente</span>
              <select
                value={modalState.customer_group_id}
                disabled={editingDiscount != null}
                onChange={(event) => handleGroupSelect(Number(event.target.value))}
              >
                <option value={0}>Seleziona</option>
                {(editingDiscount ? customerGroups ?? [] : unassignedGroups).map((group) => (
                  <option key={group.id} value={group.id}>
                    {group.name}
                  </option>
                ))}
              </select>
            </label>
            <label className={styles.checkboxField}>
              <input
                type="checkbox"
                checked={modalState.sellable}
                onChange={(event) => setModalState((current) => ({ ...current, sellable: event.target.checked }))}
              />
              <span>Sellable</span>
            </label>
            <label className={styles.checkboxField}>
              <input
                type="checkbox"
                checked={modalState.use_int_rounding}
                onChange={(event) =>
                  setModalState((current) => ({ ...current, use_int_rounding: event.target.checked }))
                }
              />
              <span>Arrotondamento</span>
            </label>

            <section className={styles.discountBlock}>
              <header>
                <h3>MRC</h3>
                <p>Massimo 100% solo per segno negativo.</p>
              </header>
              <div className={styles.discountRow}>
                <select
                  value={modalState.mrc.sign}
                  onChange={(event) => handleMrcChange({ sign: event.target.value as '+' | '-' })}
                >
                  <option value="-">-</option>
                  <option value="+">+</option>
                </select>
                <input
                  type="number"
                  min={0}
                  max={modalState.mrc.sign === '-' ? 100 : undefined}
                  step="0.01"
                  value={modalState.mrc.percentage}
                  onChange={(event) => handleMrcChange({ percentage: event.target.value })}
                />
              </div>
            </section>

            <section className={styles.discountBlock}>
              <header>
                <h3>NRC</h3>
                <p>Di default segue l&apos;MRC in creazione.</p>
              </header>
              <div className={styles.discountRow}>
                <select
                  value={modalState.nrc.sign}
                  onChange={(event) =>
                    setModalState((current) => ({
                      ...current,
                      nrc: normalizeDiscount({ ...current.nrc, sign: event.target.value as '+' | '-' }),
                    }))
                  }
                >
                  <option value="-">-</option>
                  <option value="+">+</option>
                </select>
                <input
                  type="number"
                  min={0}
                  max={modalState.nrc.sign === '-' ? 100 : undefined}
                  step="0.01"
                  value={modalState.nrc.percentage}
                  onChange={(event) =>
                    setModalState((current) => ({
                      ...current,
                      nrc: normalizeDiscount({ ...current.nrc, percentage: event.target.value }),
                    }))
                  }
                />
              </div>
            </section>
          </div>

          <div className={styles.modalActions}>
            <button type="button" className={styles.secondaryButton} onClick={() => setModalOpen(false)}>
              Annulla
            </button>
            <button type="button" className={styles.primaryButton} onClick={() => void handleSubmit()}>
              Salva
            </button>
          </div>
        </div>
      </Modal>
    </section>
  );
}

function emptyDiscountModalState(): DiscountModalState {
  return {
    customer_group_id: 0,
    sellable: true,
    use_int_rounding: false,
    mrc: { percentage: '0', sign: '-' },
    nrc: { percentage: '0', sign: '-' },
  };
}

function normalizeDiscount(value: DiscountValue): DiscountValue {
  const percentage = Number(value.percentage);
  if (!Number.isFinite(percentage) || percentage < 0) {
    return { ...value, percentage: '0' };
  }
  const bounded = value.sign === '-' ? Math.min(percentage, 100) : percentage;
  return { sign: value.sign, percentage: trimFloat(bounded) };
}

function trimFloat(value: number) {
  return value % 1 === 0 ? String(value) : value.toFixed(2).replace(/\.?0+$/, '');
}

function formatPercentage(value: number) {
  return trimFloat(Math.abs(value));
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
