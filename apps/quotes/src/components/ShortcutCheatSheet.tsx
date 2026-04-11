import { Modal, Button } from '@mrsmith/ui';
import styles from './ShortcutCheatSheet.module.css';

interface ShortcutCheatSheetProps {
  open: boolean;
  onClose: () => void;
}

const shortcuts: { section: string; items: { key: string; action: string }[] }[] = [
  {
    section: 'Lista',
    items: [
      { key: '/', action: 'Focus ricerca' },
      { key: 'N', action: 'Nuova proposta' },
    ],
  },
  {
    section: 'Dettaglio',
    items: [
      { key: 'Cmd+S', action: 'Salva modifiche' },
      { key: 'Cmd+⏎', action: 'Pubblica' },
      { key: '1 – 4', action: 'Cambia tab' },
    ],
  },
  {
    section: 'Globale',
    items: [
      { key: 'Esc', action: 'Chiudi modale' },
      { key: '?', action: 'Scorciatoie' },
    ],
  },
];

export function ShortcutCheatSheet({ open, onClose }: ShortcutCheatSheetProps) {
  return (
    <Modal open={open} onClose={onClose} title="Scorciatoie da tastiera" size="md">
      <div className={styles.content}>
        {shortcuts.map(group => (
          <section key={group.section} className={styles.group}>
            <div className={styles.groupTitle}>{group.section}</div>
            <div className={styles.rows}>
              {group.items.map(s => (
                <div key={s.key} className={styles.row}>
                  <span className={styles.action}>{s.action}</span>
                  <kbd className={styles.key}>{s.key}</kbd>
                </div>
              ))}
            </div>
          </section>
        ))}
      </div>
      <div className={styles.actions}>
        <Button variant="primary" onClick={onClose}>
          Chiudi
        </Button>
      </div>
    </Modal>
  );
}
