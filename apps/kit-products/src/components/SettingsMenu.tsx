import { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import styles from './SettingsMenu.module.css';

const items = [
  { label: 'Categorie', path: '/settings/categories' },
  { label: 'Gruppi cliente', path: '/settings/customer-groups' },
  { label: 'Raggruppamento prodotti', path: '/settings/product-groups' },
];

export function SettingsMenu() {
  const location = useLocation();
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;

    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }

    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpen(false);
        triggerRef.current?.focus();
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [open]);

  return (
    <div className={styles.menu} ref={menuRef}>
      <button
        ref={triggerRef}
        type="button"
        className={styles.trigger}
        aria-label="Apri impostazioni"
        aria-expanded={open}
        aria-haspopup="menu"
        onClick={() => setOpen((current) => !current)}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path
            d="M10.325 4.317a1.724 1.724 0 0 1 3.35 0l.18.734a1.724 1.724 0 0 0 2.573 1.066l.647-.373a1.724 1.724 0 0 1 2.372.632l.442.766a1.724 1.724 0 0 1-.632 2.372l-.648.374a1.724 1.724 0 0 0 0 2.984l.648.374a1.724 1.724 0 0 1 .632 2.372l-.442.766a1.724 1.724 0 0 1-2.372.632l-.647-.373a1.724 1.724 0 0 0-2.573 1.066l-.18.734a1.724 1.724 0 0 1-3.35 0l-.18-.734a1.724 1.724 0 0 0-2.573-1.066l-.647.373a1.724 1.724 0 0 1-2.372-.632l-.442-.766a1.724 1.724 0 0 1 .632-2.372l.648-.374a1.724 1.724 0 0 0 0-2.984l-.648-.374a1.724 1.724 0 0 1-.632-2.372l.442-.766a1.724 1.724 0 0 1 2.372-.632l.647.373a1.724 1.724 0 0 0 2.573-1.066l.18-.734Z"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
          />
          <path
            d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
          />
        </svg>
        <span>Impostazioni</span>
      </button>
      {open ? (
        <div className={styles.panel} role="menu">
          {items.map((item) => (
            <Link
              key={item.path}
              to={item.path}
              role="menuitem"
              className={`${styles.link} ${location.pathname === item.path ? styles.active : ''}`}
              onClick={() => setOpen(false)}
            >
              {item.label}
            </Link>
          ))}
        </div>
      ) : null}
    </div>
  );
}
