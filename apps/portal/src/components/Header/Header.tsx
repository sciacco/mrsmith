import { Icon, UserMenu } from '@mrsmith/ui';
import { useEffect, useRef, useState } from 'react';
import type { NotificationItem } from '../../api/notifications';
import styles from './Header.module.css';

type HeaderProps = {
  appName?: string;
  userName?: string;
  onLogout?: () => void;
  notifications?: HeaderNotifications;
};

type HeaderNotifications = {
  unreadCount: number;
  items: NotificationItem[];
  loading: boolean;
  error: boolean;
  onMarkAllRead: () => Promise<void>;
  onOpen: (item: NotificationItem) => Promise<void>;
  onArchive: (item: NotificationItem) => Promise<void>;
};

export function Header({
  appName = 'MrSmith',
  userName = 'Agent J. Doe',
  onLogout,
  notifications,
}: HeaderProps) {
  return (
    <header className={styles.header}>
      <div className={styles.logo}>
        {appName}
        <span className={styles.glasses} aria-hidden="true">
          <svg
            viewBox="0 0 48 20"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.5}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <rect
              x="2"
              y="5"
              width="16"
              height="11"
              rx="2"
              fill="rgba(0,255,65,0.08)"
            />
            <rect
              x="30"
              y="5"
              width="16"
              height="11"
              rx="2"
              fill="rgba(0,255,65,0.08)"
            />
            <path d="M18 10 Q24 4 30 10" />
            <line x1="2" y1="8" x2="0" y2="5" />
            <line x1="46" y1="8" x2="48" y2="5" />
          </svg>
        </span>
      </div>
      <div className={styles.actions}>
        {notifications ? <NotificationMenu notifications={notifications} /> : null}
        <UserMenu userName={userName} onLogout={onLogout} />
      </div>
    </header>
  );
}

function NotificationMenu({ notifications }: { notifications: HeaderNotifications }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const unreadCount = Math.min(notifications.unreadCount, 99);

  useEffect(() => {
    if (!open) return undefined;
    function handleClickOutside(event: MouseEvent) {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false);
    }
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [open]);

  return (
    <div className={styles.notifications} ref={ref}>
      <button
        type="button"
        className={styles.notificationTrigger}
        aria-label="Notifications"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((value) => !value)}
      >
        <Icon className={styles.notificationIcon} name="bell" size={21} strokeWidth={1.9} />
        {unreadCount > 0 ? <span className={styles.badge}>{unreadCount}</span> : null}
      </button>

      {open ? (
        <div className={styles.notificationPanel} role="dialog" aria-label="Notifications">
          <div className={styles.notificationPanelHeader}>
            <span className={styles.panelTitle}>SIGNALS</span>
            {notifications.unreadCount > 0 ? (
              <button
                type="button"
                className={styles.readAllButton}
                onClick={() => void notifications.onMarkAllRead()}
              >
                READ ALL
              </button>
            ) : null}
          </div>
          <div className={styles.notificationList}>
            {notifications.error ? (
              <div className={styles.notificationState}>SIGNAL ERROR</div>
            ) : notifications.loading && notifications.items.length === 0 ? (
              <div className={styles.notificationState}>SYNCING</div>
            ) : notifications.items.length === 0 ? (
              <div className={styles.notificationState}>NO SIGNALS</div>
            ) : (
              notifications.items.map((item) => (
                <div
                  className={item.readAt ? styles.notificationItem : styles.notificationItemUnread}
                  key={item.id}
                >
                  <button
                    type="button"
                    className={styles.notificationOpen}
                    onClick={() => void notifications.onOpen(item)}
                  >
                    <span className={styles.notificationMeta}>
                      {item.appId.toUpperCase()} / {formatNotificationTime(item.createdAt)}
                    </span>
                    <span className={styles.notificationTitle}>{item.title}</span>
                    {item.body ? <span className={styles.notificationBody}>{item.body}</span> : null}
                  </button>
                  <button
                    type="button"
                    className={styles.archiveButton}
                    aria-label="Archive notification"
                    onClick={(event) => {
                      event.stopPropagation();
                      void notifications.onArchive(item);
                    }}
                  >
                    <Icon className={styles.archiveIcon} name="archive" size={16} strokeWidth={1.8} />
                  </button>
                </div>
              ))
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}

function formatNotificationTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '--:--';
  return new Intl.DateTimeFormat(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}
