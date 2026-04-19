import { useEffect, useMemo, useState } from 'react';

interface UsePagedDataResult<T> {
  page: number;
  setPage: (p: number) => void;
  totalPages: number;
  pageData: T[];
  rangeLabel: string;
}

export function usePagedData<T>(data: T[], pageSize: number): UsePagedDataResult<T> {
  const [page, setPage] = useState(1);
  const size = Math.max(1, pageSize);
  const total = data.length;
  const totalPages = Math.max(1, Math.ceil(total / size));

  useEffect(() => {
    if (page > totalPages) setPage(totalPages);
  }, [page, totalPages]);

  const pageData = useMemo(() => {
    const start = (Math.min(page, totalPages) - 1) * size;
    return data.slice(start, start + size);
  }, [data, page, size, totalPages]);

  const rangeLabel = useMemo(() => {
    if (total === 0) return '0 di 0';
    const start = (Math.min(page, totalPages) - 1) * size + 1;
    const end = Math.min(start + size - 1, total);
    return `${start}–${end} di ${total}`;
  }, [page, size, total, totalPages]);

  return { page: Math.min(page, totalPages), setPage, totalPages, pageData, rangeLabel };
}
