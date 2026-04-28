import { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Icon } from '@mrsmith/ui';
import styles from './SettingsMenu.module.css';

const items = [
  { label: 'Impostazioni qualifica', path: '/impostazioni-qualifica' },
  { label: 'Pagamenti RDA', path: '/modalita-pagamenti-rda' },
  { label: 'Articoli-categorie', path: '/articoli-categorie' },
];

function isActive(pathname: string, path: string) {
  return pathname === path || pathname.startsWith(path + '/');
}

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
        <Icon name="settings" size={16} aria-hidden />
        <span>Impostazioni</span>
      </button>
      {open ? (
        <div className={styles.panel} role="menu">
          {items.map((item) => {
            const active = isActive(location.pathname, item.path);
            return (
              <Link
                key={item.path}
                to={item.path}
                role="menuitem"
                aria-current={active ? 'page' : undefined}
                className={`${styles.link} ${active ? styles.active : ''}`}
                onClick={() => setOpen(false)}
              >
                {item.label}
              </Link>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
