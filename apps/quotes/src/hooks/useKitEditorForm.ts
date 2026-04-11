import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ProductGroup, ProductVariant } from '../api/types';

export interface ProductFormValues {
  quantity: number;
  nrc: number;
  mrc: number;
  extended_description: string | null;
  included: boolean;
}

export interface ProductFormEntry extends ProductFormValues {
  id: number;
  product_code: string;
  product_name: string;
  group_name: string;
  minimum: number;
  maximum: number;
  required: boolean;
  main_product: boolean;
  position: number;
}

type FormMap = Map<number, ProductFormEntry>;

function buildEntries(groups: ProductGroup[] | undefined): FormMap {
  const map: FormMap = new Map();
  if (!groups) return map;
  for (const g of groups) {
    for (const p of g.products) {
      map.set(p.id, {
        id: p.id,
        product_code: p.product_code,
        product_name: p.product_name,
        group_name: g.group_name,
        minimum: p.minimum,
        maximum: p.maximum,
        required: g.required,
        main_product: p.main_product,
        position: p.position,
        quantity: p.quantity,
        nrc: p.nrc,
        mrc: p.mrc,
        extended_description: p.extended_description,
        included: p.included,
      });
    }
  }
  return map;
}

function cloneMap(map: FormMap): FormMap {
  const next: FormMap = new Map();
  for (const [id, entry] of map) next.set(id, { ...entry });
  return next;
}

function entriesDiffer(a: ProductFormEntry, b: ProductFormEntry): boolean {
  return (
    a.quantity !== b.quantity ||
    a.nrc !== b.nrc ||
    a.mrc !== b.mrc ||
    a.extended_description !== b.extended_description ||
    a.included !== b.included
  );
}

export interface KitEditorForm {
  state: FormMap;
  snapshot: FormMap;
  groupsByName: Map<string, ProductVariant[]>;
  groupOrder: ProductGroup[];
  isDirty: boolean;
  dirtyProductIds: number[];
  liveTotals: { nrc: number; mrc: number };
  validation: { missingRequiredGroups: string[] };
  setProductField: <K extends keyof ProductFormValues>(
    id: number,
    field: K,
    value: ProductFormValues[K],
  ) => void;
  setIncludedForGroup: (groupName: string, productId: number | null) => void;
  reset: () => void;
  commitAll: () => void;
  commitProducts: (ids: number[]) => void;
  savingRef: React.MutableRefObject<boolean>;
}

export function useKitEditorForm(
  groups: ProductGroup[] | undefined,
  rowId: number | null,
): KitEditorForm {
  const [state, setState] = useState<FormMap>(() => buildEntries(groups));
  const [snapshot, setSnapshot] = useState<FormMap>(() => buildEntries(groups));
  const savingRef = useRef(false);

  // Rebuild when a different kit is loaded or server data arrives.
  // Skip rebuilds while saving to avoid racing with our own mutations.
  useEffect(() => {
    if (savingRef.current) return;
    const next = buildEntries(groups);
    setState(next);
    setSnapshot(cloneMap(next));
  }, [groups, rowId]);

  const setProductField = useCallback(
    <K extends keyof ProductFormValues>(id: number, field: K, value: ProductFormValues[K]) => {
      setState(prev => {
        const entry = prev.get(id);
        if (!entry) return prev;
        const nextEntry = { ...entry, [field]: value };
        const next = new Map(prev);
        next.set(id, nextEntry);
        return next;
      });
    },
    [],
  );

  const setIncludedForGroup = useCallback((groupName: string, productId: number | null) => {
    setState(prev => {
      const next = new Map(prev);
      for (const [id, entry] of prev) {
        if (entry.group_name !== groupName) continue;
        const shouldInclude = id === productId;
        if (entry.included === shouldInclude) continue;
        const nextQty =
          shouldInclude && entry.quantity <= 0 ? Math.max(1, entry.minimum) : entry.quantity;
        next.set(id, { ...entry, included: shouldInclude, quantity: nextQty });
      }
      return next;
    });
  }, []);

  const reset = useCallback(() => {
    setState(cloneMap(snapshot));
  }, [snapshot]);

  const commitAll = useCallback(() => {
    setSnapshot(cloneMap(state));
  }, [state]);

  const commitProducts = useCallback((ids: number[]) => {
    setSnapshot(prev => {
      const next = new Map(prev);
      for (const id of ids) {
        const entry = state.get(id);
        if (entry) next.set(id, { ...entry });
      }
      return next;
    });
  }, [state]);

  const dirtyProductIds = useMemo(() => {
    const ids: number[] = [];
    for (const [id, entry] of state) {
      const original = snapshot.get(id);
      if (!original || entriesDiffer(entry, original)) ids.push(id);
    }
    return ids;
  }, [state, snapshot]);

  const isDirty = dirtyProductIds.length > 0;

  const liveTotals = useMemo(() => {
    let nrc = 0;
    let mrc = 0;
    for (const entry of state.values()) {
      if (!entry.included) continue;
      nrc += entry.nrc * entry.quantity;
      mrc += entry.mrc * entry.quantity;
    }
    return { nrc, mrc };
  }, [state]);

  const validation = useMemo(() => {
    const missing: string[] = [];
    if (!groups) return { missingRequiredGroups: missing };
    for (const g of groups) {
      if (!g.required) continue;
      const anyIncluded = g.products.some(p => state.get(p.id)?.included);
      if (!anyIncluded) missing.push(g.group_name);
    }
    return { missingRequiredGroups: missing };
  }, [groups, state]);

  const groupsByName = useMemo(() => {
    const map = new Map<string, ProductVariant[]>();
    if (!groups) return map;
    for (const g of groups) map.set(g.group_name, g.products);
    return map;
  }, [groups]);

  const groupOrder = useMemo(() => groups ?? [], [groups]);

  return {
    state,
    snapshot,
    groupsByName,
    groupOrder,
    isDirty,
    dirtyProductIds,
    liveTotals,
    validation,
    setProductField,
    setIncludedForGroup,
    reset,
    commitAll,
    commitProducts,
    savingRef,
  };
}
