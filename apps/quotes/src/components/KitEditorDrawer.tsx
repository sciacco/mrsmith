import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Drawer, useToast } from '@mrsmith/ui';
import type { DocumentType, ProductGroup, QuoteRow } from '../api/types';
import { useRowProducts, useUpdateProduct } from '../api/queries';
import { useKitEditorForm, type ProductFormEntry } from '../hooks/useKitEditorForm';
import { buildProductUpdatePayload } from '../utils/quoteRules';
import { ProductGroupEditor } from './ProductGroupEditor';
import { ConfirmDialog } from './ConfirmDialog';
import styles from './KitEditorDrawer.module.css';

interface KitEditorDrawerProps {
  open: boolean;
  quoteId: number;
  documentType: DocumentType;
  rows: QuoteRow[];
  initialRowId: number | null;
  onClose: () => void;
  onDirtyChange?: (isDirty: boolean) => void;
}

type PendingAction =
  | { type: 'navigate'; targetRowId: number }
  | { type: 'close' }
  | null;

function sortGroups(groups: ProductGroup[], missing: Set<string>): ProductGroup[] {
  return [...groups].sort((a, b) => {
    const aMissing = missing.has(a.group_name) ? 0 : 1;
    const bMissing = missing.has(b.group_name) ? 0 : 1;
    if (aMissing !== bMissing) return aMissing - bMissing;
    const aReq = a.required ? 0 : 1;
    const bReq = b.required ? 0 : 1;
    if (aReq !== bReq) return aReq - bReq;
    return a.position - b.position;
  });
}

function formatCurrency(value: number): string {
  return value.toFixed(2);
}

export function KitEditorDrawer({
  open,
  quoteId,
  documentType,
  rows,
  initialRowId,
  onClose,
  onDirtyChange,
}: KitEditorDrawerProps) {
  const [currentRowId, setCurrentRowId] = useState<number | null>(null);
  const [pendingAction, setPendingAction] = useState<PendingAction>(null);
  const [saving, setSaving] = useState(false);
  const { toast } = useToast();

  const updateProduct = useUpdateProduct();
  const { data: groups, isLoading: loadingGroups } = useRowProducts(quoteId, currentRowId ?? 0);
  const form = useKitEditorForm(groups, currentRowId);

  const dismissDeferredRef = useRef<((value: boolean) => void) | null>(null);

  // When the drawer opens, seed current kit from initialRowId
  useEffect(() => {
    if (open) {
      setCurrentRowId(initialRowId);
      setPendingAction(null);
    }
  }, [open, initialRowId]);

  // Notify parent about dirty state
  useEffect(() => {
    onDirtyChange?.(form.isDirty);
  }, [form.isDirty, onDirtyChange]);

  const currentRow = useMemo(
    () => rows.find(r => r.id === currentRowId) ?? null,
    [rows, currentRowId],
  );

  const missingSet = useMemo(
    () => new Set(form.validation.missingRequiredGroups),
    [form.validation.missingRequiredGroups],
  );

  const orderedGroups = useMemo(
    () => (groups ? sortGroups(groups, missingSet) : []),
    [groups, missingSet],
  );

  const handleSave = useCallback(async (): Promise<boolean> => {
    if (!currentRowId || form.dirtyProductIds.length === 0) return true;

    form.savingRef.current = true;
    setSaving(true);

    // Save products toggled off first, then the rest. Avoids transient state
    // where two products in the same group are both `included=true`.
    const orderedIds = [...form.dirtyProductIds].sort((a, b) => {
      const aOff = form.state.get(a)?.included ? 1 : 0;
      const bOff = form.state.get(b)?.included ? 1 : 0;
      return aOff - bOff;
    });

    const succeeded: number[] = [];
    const failed: ProductFormEntry[] = [];
    const isSpot = documentType === 'TSC-ORDINE';

    for (const id of orderedIds) {
      const entry = form.state.get(id);
      if (!entry) continue;
      try {
        await updateProduct.mutateAsync({
          quoteId,
          rowId: currentRowId,
          productId: id,
          data: buildProductUpdatePayload(
            {
              id: entry.id,
              product_name: entry.product_name,
              nrc: entry.nrc,
              mrc: entry.mrc,
              quantity: entry.quantity,
              extended_description: entry.extended_description,
              included: entry.included,
            },
            isSpot,
          ),
        });
        succeeded.push(id);
      } catch {
        failed.push(entry);
      }
    }

    if (succeeded.length > 0) {
      form.commitProducts(succeeded);
    }
    form.savingRef.current = false;
    setSaving(false);

    if (failed.length === 0) {
      toast('Modifiche salvate', 'success');
      return true;
    }

    if (succeeded.length === 0) {
      toast('Salvataggio fallito. Riprova.', 'error');
      return false;
    }

    toast(
      `Salvataggio parziale: ${failed.length} prodott${failed.length === 1 ? 'o' : 'i'} non salvat${failed.length === 1 ? 'o' : 'i'}`,
      'warning',
    );
    return false;
  }, [currentRowId, documentType, form, quoteId, toast, updateProduct]);

  const applyPendingAction = useCallback(
    (action: PendingAction) => {
      if (!action) return;
      if (action.type === 'navigate') {
        setCurrentRowId(action.targetRowId);
      } else {
        onClose();
      }
    },
    [onClose],
  );

  const handleNavigate = useCallback(
    (targetRowId: number) => {
      if (targetRowId === currentRowId) return;
      if (form.isDirty) {
        setPendingAction({ type: 'navigate', targetRowId });
        return;
      }
      setCurrentRowId(targetRowId);
    },
    [currentRowId, form.isDirty],
  );

  const handleDismissAttempt = useCallback(async (): Promise<boolean> => {
    if (!form.isDirty) return true;
    setPendingAction({ type: 'close' });
    return new Promise<boolean>(resolve => {
      dismissDeferredRef.current = resolve;
    });
  }, [form.isDirty]);

  const resolveDismissDeferred = useCallback((value: boolean) => {
    const resolve = dismissDeferredRef.current;
    dismissDeferredRef.current = null;
    resolve?.(value);
  }, []);

  const confirmSaveAndContinue = useCallback(async () => {
    const ok = await handleSave();
    if (!ok) return;
    const action = pendingAction;
    setPendingAction(null);
    if (action?.type === 'close') {
      resolveDismissDeferred(true);
    } else {
      applyPendingAction(action);
    }
  }, [applyPendingAction, handleSave, pendingAction, resolveDismissDeferred]);

  const confirmDiscardAndContinue = useCallback(() => {
    form.reset();
    const action = pendingAction;
    setPendingAction(null);
    if (action?.type === 'close') {
      resolveDismissDeferred(true);
    } else {
      applyPendingAction(action);
    }
  }, [applyPendingAction, form, pendingAction, resolveDismissDeferred]);

  const cancelPending = useCallback(() => {
    const action = pendingAction;
    setPendingAction(null);
    if (action?.type === 'close') {
      resolveDismissDeferred(false);
    }
  }, [pendingAction, resolveDismissDeferred]);

  const totals = form.liveTotals;
  const missingCount = form.validation.missingRequiredGroups.length;

  const drawerFooter = (
    <>
      <Button variant="ghost" onClick={() => void handleDismissAttempt().then(ok => ok && onClose())}>
        Annulla
      </Button>
      <Button
        variant="primary"
        disabled={!form.isDirty || saving}
        loading={saving}
        onClick={() => void handleSave()}
      >
        Salva modifiche
      </Button>
    </>
  );

  const drawerHeaderExtra = currentRow && (
    <div className={styles.headerTotals}>
      <div className={styles.headerTotal}>
        <span className={styles.headerTotalLabel}>NRC</span>
        <span className={styles.headerTotalValue}>{formatCurrency(totals.nrc)}</span>
      </div>
      <div className={styles.headerTotalDivider} aria-hidden />
      <div className={styles.headerTotal}>
        <span className={styles.headerTotalLabel}>MRC</span>
        <span className={styles.headerTotalValue}>{formatCurrency(totals.mrc)}</span>
      </div>
    </div>
  );

  return (
    <>
      <Drawer
        open={open}
        onClose={onClose}
        onDismissAttempt={handleDismissAttempt}
        size="xl"
        side="right"
        title={currentRow?.internal_name ?? 'Modifica kit'}
        subtitle={
          missingCount > 0
            ? `${missingCount} grupp${missingCount === 1 ? 'o' : 'i'} obbligator${missingCount === 1 ? 'io' : 'i'} da configurare`
            : 'Configura varianti, quantità, prezzi e descrizioni aggiuntive'
        }
        headerExtra={drawerHeaderExtra}
        footer={drawerFooter}
      >
        <div className={styles.layout}>
          <aside className={styles.sidebar}>
            <div className={styles.sidebarLabel}>Kit del preventivo</div>
            <ul className={styles.sidebarList}>
              {rows.map(row => {
                const active = row.id === currentRowId;
                return (
                  <li key={row.id}>
                    <button
                      type="button"
                      className={`${styles.sidebarItem} ${active ? styles.sidebarItemActive : ''}`}
                      onClick={() => handleNavigate(row.id)}
                    >
                      <span className={styles.sidebarName}>{row.internal_name}</span>
                      <span className={styles.sidebarTotals}>
                        NRC {formatCurrency(row.nrc_row)} · MRC {formatCurrency(row.mrc_row)}
                      </span>
                      {active && form.isDirty && (
                        <span className={styles.dirtyDot} aria-label="Modifiche non salvate" />
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          </aside>

          <div className={styles.main}>
            {!currentRowId ? (
              <div className={styles.emptyState}>Seleziona un kit dalla sidebar.</div>
            ) : loadingGroups && !groups ? (
              <div className={styles.emptyState}>Caricamento prodotti…</div>
            ) : orderedGroups.length === 0 ? (
              <div className={styles.emptyState}>Nessun gruppo prodotto nel kit.</div>
            ) : (
              <div className={styles.groups}>
                {orderedGroups.map(g => (
                  <ProductGroupEditor
                    key={g.group_name}
                    group={g}
                    documentType={documentType}
                    form={form}
                    isMissing={missingSet.has(g.group_name)}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      </Drawer>

      <ConfirmDialog
        open={pendingAction !== null}
        title="Modifiche non salvate"
        message={
          pendingAction?.type === 'navigate'
            ? 'Hai modifiche non salvate su questo kit. Vuoi salvarle prima di passare a un altro kit?'
            : 'Hai modifiche non salvate su questo kit. Vuoi salvarle prima di chiudere?'
        }
        confirmLabel="Salva e continua"
        variant="primary"
        discardLabel="Scarta modifiche"
        onConfirm={() => void confirmSaveAndContinue()}
        onDiscard={confirmDiscardAndContinue}
        onCancel={cancelPending}
        confirmLoading={saving}
      />
    </>
  );
}
