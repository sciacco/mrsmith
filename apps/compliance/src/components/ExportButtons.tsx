import { useState } from 'react';
import { useToast } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import styles from './Compliance.module.css';

interface ExportButtonsProps {
  basePath: string;
  params: Record<string, string>;
}

export function ExportButtons({ basePath, params }: ExportButtonsProps) {
  const [loading, setLoading] = useState<'csv' | 'xlsx' | null>(null);
  const { toast } = useToast();
  const api = useApiClient();

  async function handleExport(format: 'csv' | 'xlsx') {
    setLoading(format);
    try {
      const qs = new URLSearchParams({ ...params, format }).toString();
      const blob = await api.getBlob(`${basePath}?${qs}`);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      const date = new Date().toISOString().split('T')[0];
      a.download = `export-${date}.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast('Errore durante l\'esportazione', 'error');
    } finally {
      setLoading(null);
    }
  }

  return (
    <div className={styles.exportRow}>
      <button
        className={styles.btnSecondary}
        onClick={() => handleExport('csv')}
        disabled={loading !== null}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
          <path d="M7 1v9M4 7l3 3 3-3M2 12h10" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {loading === 'csv' ? 'Esportazione...' : 'CSV'}
      </button>
      <button
        className={styles.btnSecondary}
        onClick={() => handleExport('xlsx')}
        disabled={loading !== null}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
          <path d="M7 1v9M4 7l3 3 3-3M2 12h10" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {loading === 'xlsx' ? 'Esportazione...' : 'XLSX'}
      </button>
    </div>
  );
}
