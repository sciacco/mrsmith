import { Button, Modal } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import styles from '../facilities/workspace.module.css';

export const destructiveBody = { confirmPrimary: true, confirmSecondary: true };

export function valueOrDash(value: unknown) {
  if (value === undefined || value === null || value === '') return '-';
  return String(value);
}

export function errorText(error: unknown, fallback: string) {
  if (typeof error === 'object' && error && 'body' in error) {
    const body = (error as { body?: unknown }).body;
    if (typeof body === 'object' && body && 'message' in body) return String((body as { message?: unknown }).message);
  }
  return fallback;
}

export function Detail({ label, value }: { label: string; value: unknown }) {
  return <div className={styles.detailItem}><span className={styles.detailLabel}>{label}</span><span className={styles.detailValue}>{valueOrDash(value)}</span></div>;
}

export function TextField({ label, value, onChange, type = 'text' }: { label: string; value: string; onChange: (value: string) => void; type?: string }) {
  return <div className={styles.field}><label>{label}</label><input type={type} value={value} onChange={(event) => onChange(event.target.value)} /></div>;
}

export function NumberField({ label, value, onChange }: { label: string; value: number; onChange: (value: number) => void }) {
  return <div className={styles.field}><label>{label}</label><input type="number" value={value} onChange={(event) => onChange(Number(event.target.value))} /></div>;
}

export function SelectField({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: string[] }) {
  return <div className={styles.field}><label>{label}</label><select value={value} onChange={(event) => onChange(event.target.value)}>{options.map((option) => <option key={option} value={option}>{option}</option>)}</select></div>;
}

export function ConfirmModal({ open, title, message, onClose, onConfirm, loading }: { open: boolean; title: string; message: string; onClose: () => void; onConfirm: () => void; loading: boolean }) {
  const [first, setFirst] = useState(false);
  const [second, setSecond] = useState(false);
  useEffect(() => {
    if (open) {
      setFirst(false);
      setSecond(false);
    }
  }, [open]);
  return (
    <Modal open={open} onClose={onClose} title={title}>
      <p className={styles.emptyText}>{message}</p>
      <label className={styles.checkboxLine}><input type="checkbox" checked={first} onChange={(event) => setFirst(event.target.checked)} /> Ho verificato le dipendenze operative.</label>
      <label className={styles.checkboxLine}><input type="checkbox" checked={second} onChange={(event) => setSecond(event.target.checked)} /> Confermo l'azione richiesta.</label>
      <div className={styles.modalActions}>
        <Button variant="secondary" onClick={onClose}>Annulla</Button>
        <Button variant="danger" loading={loading} disabled={!first || !second} onClick={onConfirm}>Conferma</Button>
      </div>
    </Modal>
  );
}
