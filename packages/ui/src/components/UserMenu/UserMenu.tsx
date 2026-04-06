import { useState, useRef, useEffect } from 'react';
import styles from './UserMenu.module.css';

interface UserMenuProps {
  userName: string;
  avatarUrl?: string;
  onLogout?: () => void;
}

function SmithAvatar({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 120 120" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="60" cy="60" r="58" fill="#1a2a1a" stroke="currentColor" strokeWidth="1.5" strokeOpacity="0.3" />
      <path d="M20 110 Q20 85 40 78 L50 74 L60 80 L70 74 L80 78 Q100 85 100 110" fill="#1a1a1a" stroke="#2a2a2a" strokeWidth="0.5" />
      <path d="M52 74 L60 82 L68 74" fill="none" stroke="#e8e8e8" strokeWidth="1.5" />
      <path d="M60 78 L57 95 L60 110 L63 95 Z" fill="#1a1a1a" stroke="#333" strokeWidth="0.5" />
      <path d="M56 76 L60 80 L64 76" fill="#333" stroke="#444" strokeWidth="0.5" />
      <rect x="54" y="62" width="12" height="14" rx="4" fill="#c8a882" />
      <ellipse cx="60" cy="45" rx="22" ry="26" fill="#d4b896" />
      <path d="M38 38 Q38 20 60 18 Q82 20 82 38 L82 34 Q80 22 60 20 Q40 22 38 34 Z" fill="#2a2218" />
      <path d="M36 42 Q36 28 42 24" fill="none" stroke="#2a2218" strokeWidth="3" />
      <path d="M84 42 Q84 28 78 24" fill="none" stroke="#2a2218" strokeWidth="3" />
      <ellipse cx="38" cy="46" rx="4" ry="6" fill="#c8a882" />
      <ellipse cx="82" cy="46" rx="4" ry="6" fill="#c8a882" />
      <rect x="41" y="38" width="15" height="10" rx="2" fill="#111" stroke="#333" strokeWidth="1" />
      <rect x="64" y="38" width="15" height="10" rx="2" fill="#111" stroke="#333" strokeWidth="1" />
      <path d="M56 42 Q60 38 64 42" stroke="#333" strokeWidth="1" fill="none" />
      <line x1="41" y1="40" x2="38" y2="38" stroke="#333" strokeWidth="1" />
      <line x1="79" y1="40" x2="82" y2="38" stroke="#333" strokeWidth="1" />
      <rect x="43" y="40" width="4" height="2" rx="0.5" fill="#1a3a1a" opacity="0.4" />
      <rect x="66" y="40" width="4" height="2" rx="0.5" fill="#1a3a1a" opacity="0.4" />
      <path d="M60 48 L58 54 L62 54" fill="none" stroke="#b8956e" strokeWidth="1" strokeLinecap="round" />
      <path d="M53 60 Q60 62 67 60" fill="none" stroke="#8a6e52" strokeWidth="1.2" strokeLinecap="round" />
      <path d="M42 36 Q48 33 56 35" fill="none" stroke="#2a2218" strokeWidth="1.8" strokeLinecap="round" />
      <path d="M64 35 Q72 33 78 36" fill="none" stroke="#2a2218" strokeWidth="1.8" strokeLinecap="round" />
    </svg>
  );
}

export function UserMenu({ userName, avatarUrl, onLogout }: UserMenuProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const displayName = userName.startsWith('Agent ') ? userName : `Agent ${userName}`;

  useEffect(() => {
    if (!open) return;

    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    function handleEscape(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }

    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [open]);

  return (
    <div className={styles.wrapper} ref={ref}>
      <button
        type="button"
        className={styles.trigger}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="menu"
      >
        <span className={styles.userName}>{displayName}</span>
        {avatarUrl ? (
          <img src={avatarUrl} alt="Avatar" className={styles.avatar} />
        ) : (
          <SmithAvatar className={styles.avatar} />
        )}
      </button>
      {open && onLogout && (
        <div className={styles.menu} role="menu">
          <button
            type="button"
            className={styles.menuItem}
            role="menuitem"
            onClick={() => {
              setOpen(false);
              onLogout();
            }}
          >
            Logout
          </button>
        </div>
      )}
    </div>
  );
}
