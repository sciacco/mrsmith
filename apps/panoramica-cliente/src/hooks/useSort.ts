import { useState, useMemo } from 'react';

export type SortDir = 'asc' | 'desc';

export interface SortState<K> {
  key: K;
  dir: SortDir;
}

export function useSort<T>(defaultKey: keyof T, defaultDir: SortDir = 'asc') {
  const [sort, setSort] = useState<SortState<keyof T>>({ key: defaultKey, dir: defaultDir });

  function toggle(key: keyof T) {
    setSort(prev =>
      prev.key === key
        ? { key, dir: prev.dir === 'asc' ? 'desc' : 'asc' }
        : { key, dir: 'asc' },
    );
  }

  function sorted(data: T[]): T[] {
    return [...data].sort((a, b) => {
      const av = a[sort.key];
      const bv = b[sort.key];
      if (av == null && bv == null) return 0;
      if (av == null) return 1;
      if (bv == null) return -1;
      const cmp = typeof av === 'number' && typeof bv === 'number'
        ? av - bv
        : String(av).localeCompare(String(bv), 'it');
      return sort.dir === 'asc' ? cmp : -cmp;
    });
  }

  return { sort, toggle, sorted };
}

export function useSortedData<T>(
  data: T[] | undefined,
  defaultKey: keyof T,
  defaultDir: SortDir = 'asc',
) {
  const { sort, toggle, sorted } = useSort<T>(defaultKey, defaultDir);
  const result = useMemo(() => sorted(data ?? []), [data, sort.key, sort.dir]);
  return { sortedData: result, sort, toggle };
}
