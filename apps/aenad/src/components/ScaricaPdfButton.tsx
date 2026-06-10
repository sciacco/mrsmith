import { Button, Icon, useToast } from '@mrsmith/ui';
import { useEffect, useRef, useState } from 'react';
import { useDocumentPdfDownload } from '../api/queries';
import { downloadBlob, offertaFilename } from '../utils/downloads';
import styles from './ScaricaPdfButton.module.css';

export interface ScaricaPdfDoc {
  IDDoc: number;
  NumDoc: string | null;
  DataDoc: string | null;
  Anagr_Nome: string | null;
}

export function ScaricaPdfButton({
  doc,
  size,
  variant = 'secondary',
}: {
  doc: ScaricaPdfDoc;
  size?: 'sm' | 'md';
  variant?: 'primary' | 'secondary';
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const downloadPdf = useDocumentPdfDownload();
  const { toast } = useToast();

  useEffect(() => {
    if (!menuOpen) return undefined;

    function handleClickOutside(event: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    }
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') setMenuOpen(false);
    }

    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [menuOpen]);

  async function handleDownload(condizioni: boolean) {
    setMenuOpen(false);
    setDownloading(true);
    try {
      const blob = await downloadPdf(doc.IDDoc, condizioni);
      downloadBlob(blob, offertaFilename(doc));
    } catch {
      toast('Generazione del PDF non riuscita. Riprova.', 'error');
    } finally {
      setDownloading(false);
    }
  }

  return (
    <div className={styles.wrapper} ref={wrapperRef}>
      <Button
        size={size}
        variant={variant}
        disabled={downloading}
        onClick={() => setMenuOpen((open) => !open)}
        aria-haspopup="menu"
        aria-expanded={menuOpen}
      >
        <Icon name="download" size={16} />
        {downloading ? 'Generazione...' : 'Scarica PDF'}
        <Icon name="chevron-down" size={14} />
      </Button>
      {menuOpen ? (
        <div className={styles.menu} role="menu">
          <button type="button" role="menuitem" className={styles.menuItem} onClick={() => void handleDownload(true)}>
            Con condizioni di fornitura
          </button>
          <button type="button" role="menuitem" className={styles.menuItem} onClick={() => void handleDownload(false)}>
            Senza condizioni
          </button>
        </div>
      ) : null}
    </div>
  );
}
