import { useEffect, useState, type FormEvent } from 'react';
import { Modal, Button, useToast } from '@mrsmith/ui';
import { ApiError } from '@mrsmith/api-client';
import {
  useCreateAdmin,
  type NotificationKey,
} from '../../hooks/useCreateAdmin';
import styles from './GestioneUtenti.module.css';

interface NuovoAdminModalProps {
  open: boolean;
  onClose: () => void;
  customerId: number;
}

// Internal UI keys for the notification checkbox group. These are local to
// the React tree and are translated to wire-format keys inside
// useCreateAdmin. They are NEVER sent on the wire.
interface NotificationOption {
  key: NotificationKey;
  label: string;
}

const NOTIFICATION_OPTIONS: readonly NotificationOption[] = [
  { key: 'maintenance', label: 'Comunicazioni di manutenzione' },
  { key: 'marketing', label: 'Comunicazioni marketing' },
];

export function NuovoAdminModal({
  open,
  onClose,
  customerId,
}: NuovoAdminModalProps) {
  const [nome, setNome] = useState('');
  const [cognome, setCognome] = useState('');
  const [email, setEmail] = useState('');
  const [telefono, setTelefono] = useState('');
  const [notifications, setNotifications] = useState<Set<NotificationKey>>(
    new Set(),
  );

  const { toast } = useToast();
  const createAdmin = useCreateAdmin();

  useEffect(() => {
    if (!open) {
      setNome('');
      setCognome('');
      setEmail('');
      setTelefono('');
      setNotifications(new Set());
    }
  }, [open]);

  function toggleNotification(key: NotificationKey) {
    setNotifications((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    createAdmin.mutate(
      {
        customerId,
        nome: nome.trim(),
        cognome: cognome.trim(),
        email: email.trim(),
        telefono: telefono.trim(),
        notifications,
      },
      {
        onSuccess: () => {
          toast('Admin creato');
          onClose();
        },
        onError: (error) => {
          const fallback = "Qualcosa e' andato storto";
          if (error instanceof ApiError) {
            const message = readMessage(error.body) ?? fallback;
            toast(`${error.status} — ${message}`, 'error');
          } else {
            toast(fallback, 'error');
          }
        },
      },
    );
  }

  const submitDisabled =
    createAdmin.isPending ||
    !nome.trim() ||
    !cognome.trim() ||
    !email.trim();

  return (
    <Modal open={open} onClose={onClose} title="Nuovo Admin">
      <form onSubmit={handleSubmit}>
        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="admin-nome">
            Nome
          </label>
          <input
            id="admin-nome"
            className={styles.input}
            type="text"
            value={nome}
            onChange={(e) => setNome(e.target.value)}
            required
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="admin-cognome">
            Cognome
          </label>
          <input
            id="admin-cognome"
            className={styles.input}
            type="text"
            value={cognome}
            onChange={(e) => setCognome(e.target.value)}
            required
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="admin-email">
            Em@il
          </label>
          <input
            id="admin-email"
            className={styles.input}
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </div>
        <div className={styles.formGroup}>
          <label className={styles.label} htmlFor="admin-telefono">
            Telefono
          </label>
          <input
            id="admin-telefono"
            className={styles.input}
            type="tel"
            value={telefono}
            onChange={(e) => setTelefono(e.target.value)}
          />
        </div>
        <div className={styles.checkboxGroup}>
          {NOTIFICATION_OPTIONS.map((opt) => (
            <label key={opt.key} className={styles.checkboxRow}>
              <input
                type="checkbox"
                checked={notifications.has(opt.key)}
                onChange={() => toggleNotification(opt.key)}
              />
              <span>{opt.label}</span>
            </label>
          ))}
        </div>
        <div className={styles.actions}>
          <Button
            type="button"
            variant="secondary"
            onClick={onClose}
            disabled={createAdmin.isPending}
          >
            Annulla
          </Button>
          <Button
            type="submit"
            variant="primary"
            loading={createAdmin.isPending}
            disabled={submitDisabled}
          >
            Crea
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function readMessage(body: unknown): string | undefined {
  if (typeof body === 'object' && body !== null && 'message' in body) {
    const raw = (body as { message: unknown }).message;
    if (typeof raw === 'string' && raw.trim().length > 0) return raw.trim();
  }
  return undefined;
}
