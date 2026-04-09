import styles from './ShortcutCheatSheet.module.css';

interface ShortcutCheatSheetProps {
  onClose: () => void;
}

const shortcuts = [
  { key: 'Cmd+S', action: 'Salva' },
  { key: 'Cmd+Enter', action: 'Pubblica' },
  { key: '1-4', action: 'Cambia tab' },
  { key: '/', action: 'Cerca (lista)' },
  { key: '?', action: 'Scorciatoie' },
];

export function ShortcutCheatSheet({ onClose }: ShortcutCheatSheetProps) {
  return (
    <div className={styles.overlay} onClick={onClose}>
      <div className={styles.modal} onClick={e => e.stopPropagation()}>
        <div className={styles.title}>Scorciatoie da tastiera</div>
        {shortcuts.map(s => (
          <div key={s.key} className={styles.row}>
            <span>{s.action}</span>
            <span className={styles.key}>{s.key}</span>
          </div>
        ))}
        <button className={styles.close} onClick={onClose}>Chiudi</button>
      </div>
    </div>
  );
}
