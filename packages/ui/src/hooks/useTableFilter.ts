import { useMemo } from 'react';

interface FilterEntry<T> {
  field: keyof T;
  value: string | number | null;
}

interface UseTableFilterConfig<T> {
  data: T[] | undefined;
  searchQuery: string;
  searchFields: (keyof T)[];
  filters?: Record<string, FilterEntry<T>>;
}

interface UseTableFilterResult<T> {
  filtered: T[];
  totalCount: number;
  filteredCount: number;
}

export function useTableFilter<T>(config: UseTableFilterConfig<T>): UseTableFilterResult<T> {
  const { data, searchQuery, searchFields, filters } = config;

  return useMemo(() => {
    if (!data) return { filtered: [], totalCount: 0, filteredCount: 0 };

    const totalCount = data.length;
    const q = searchQuery.toLowerCase().trim();

    const activeFilters = filters
      ? Object.values(filters).filter((f) => f.value !== null)
      : [];

    const filtered = data.filter((item) => {
      // Text search
      if (q) {
        const matches = searchFields.some((field) =>
          String(item[field]).toLowerCase().includes(q),
        );
        if (!matches) return false;
      }

      // Discrete filters
      for (const f of activeFilters) {
        if (item[f.field] !== f.value) return false;
      }

      return true;
    });

    return { filtered, totalCount, filteredCount: filtered.length };
  }, [data, searchQuery, searchFields, filters]);
}
