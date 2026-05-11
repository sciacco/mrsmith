import { useCallback, useEffect, useRef, useState } from 'react';
import { Icon } from '../Icon/Icon';
import styles from './NotificationBell.module.css';

const POLL_INTERVAL_MS = 60_000;

export type NotificationBellVariant = 'clean' | 'matrix';
export type NotificationBellVisibility = 'always' | 'unread';
export type NotificationListStatus = 'all' | 'unread';

export interface NotificationBellAuth {
  authenticated?: boolean;
  getAccessToken?: (minValidity?: number) => Promise<string | undefined> | string | undefined;
  forceRefreshToken?: () => Promise<string | undefined> | string | undefined;
}

export interface NotificationBellProps {
  auth?: NotificationBellAuth;
  variant?: NotificationBellVariant;
  visibility?: NotificationBellVisibility;
  status?: NotificationListStatus;
  pollIntervalMs?: number;
}

export type NotificationSummary = {
  totalUnread: number;
  unreadByApp: Record<string, number>;
};

export type NotificationItem = {
  id: number;
  notificationId: number;
  typeKey: string;
  appId: string;
  severity: 'info' | 'success' | 'warning' | 'critical';
  title: string;
  body: string;
  entityType: string;
  entityId: string;
  deepLink: string;
  metadata: Record<string, unknown>;
  createdAt: string;
  readAt?: string;
  archivedAt?: string;
  resolvedAt?: string;
};

type NotificationListResponse = {
  items: NotificationItem[];
  nextCursor: string;
};

type NotificationsState = {
  summary: NotificationSummary;
  items: NotificationItem[];
  loading: boolean;
  error: boolean;
  refresh: () => Promise<void>;
  markAllRead: () => Promise<void>;
  openNotification: (item: NotificationItem) => Promise<void>;
  archive: (item: NotificationItem) => Promise<void>;
};

const emptySummary: NotificationSummary = {
  totalUnread: 0,
  unreadByApp: {},
};

const labels = {
  clean: {
    actionLabel: 'Notifiche',
    panelLabel: 'Notifiche',
    panelTitle: 'Notifiche',
    readAll: 'Segna lette',
    loading: 'Sincronizzazione',
    empty: 'Nessuna notifica',
    error: 'Notifiche non disponibili',
    archive: 'Archivia notifica',
  },
  matrix: {
    actionLabel: 'Notifications',
    panelLabel: 'Notifications',
    panelTitle: 'SIGNALS',
    readAll: 'READ ALL',
    loading: 'SYNCING',
    empty: 'NO SIGNALS',
    error: 'SIGNAL ERROR',
    archive: 'Archive notification',
  },
} as const;

export function NotificationBell({
  auth,
  variant = 'clean',
  visibility = 'always',
  status = 'all',
  pollIntervalMs = POLL_INTERVAL_MS,
}: NotificationBellProps) {
  const enabled = Boolean(auth?.getAccessToken) && auth?.authenticated !== false;
  const notifications = useNotifications(auth, enabled, status, pollIntervalMs);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const unreadCount = Math.min(notifications.summary.totalUnread, 99);
  const copy = labels[variant];
  const showBell = enabled && (visibility === 'always' || notifications.summary.totalUnread > 0);

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

  useEffect(() => {
    if (showBell) return;
    setOpen(false);
  }, [showBell]);

  if (!showBell) return null;

  return (
    <div className={`${styles.wrapper} ${styles[variant]}`} ref={ref}>
      <button
        type="button"
        className={styles.trigger}
        aria-label={copy.actionLabel}
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((value) => !value)}
      >
        <Icon className={styles.icon} name="bell" size={21} strokeWidth={1.9} />
        {unreadCount > 0 ? <span className={styles.badge}>{unreadCount}</span> : null}
      </button>

      {open ? (
        <div className={styles.panel} role="dialog" aria-label={copy.panelLabel}>
          <div className={styles.panelHeader}>
            <span className={styles.panelTitle}>{copy.panelTitle}</span>
            {notifications.summary.totalUnread > 0 ? (
              <button
                type="button"
                className={styles.readAllButton}
                onClick={() => void notifications.markAllRead()}
              >
                {copy.readAll}
              </button>
            ) : null}
          </div>
          <div className={styles.list}>
            {notifications.error ? (
              <div className={styles.state}>{copy.error}</div>
            ) : notifications.loading && notifications.items.length === 0 ? (
              <div className={styles.state}>{copy.loading}</div>
            ) : notifications.items.length === 0 ? (
              <div className={styles.state}>{copy.empty}</div>
            ) : (
              notifications.items.map((item) => (
                <div className={item.readAt ? styles.item : styles.itemUnread} key={item.id}>
                  <button
                    type="button"
                    className={styles.openButton}
                    onClick={() => void notifications.openNotification(item)}
                  >
                    <span className={styles.meta}>
                      {item.appId.toUpperCase()} / {formatNotificationTime(item.createdAt)}
                    </span>
                    <span className={styles.title}>{item.title}</span>
                    {item.body ? <span className={styles.body}>{item.body}</span> : null}
                  </button>
                  <button
                    type="button"
                    className={styles.archiveButton}
                    aria-label={copy.archive}
                    onClick={(event) => {
                      event.stopPropagation();
                      void notifications.archive(item);
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

function useNotifications(
  auth: NotificationBellAuth | undefined,
  enabled: boolean,
  status: NotificationListStatus,
  pollIntervalMs: number,
): NotificationsState {
  const mountedRef = useRef(false);
  const [summary, setSummary] = useState<NotificationSummary>(emptySummary);
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const request = useCallback(
    async <T,>(path: string, init?: RequestInit): Promise<T> => {
      const token = await auth?.getAccessToken?.(30);
      if (!token) throw new Error('missing_token');

      let response = await sendNotificationFetch(path, token, init);
      if (response.status === 401 && auth?.forceRefreshToken) {
        const fresh = await auth.forceRefreshToken();
        if (fresh) response = await sendNotificationFetch(path, fresh, init);
      }

      if (!response.ok) throw new Error(`notification_request_failed_${response.status}`);
      if (response.status === 204) return undefined as T;
      return response.json() as Promise<T>;
    },
    [auth],
  );

  const refresh = useCallback(async () => {
    if (!enabled || document.visibilityState !== 'visible') return;
    setLoading(true);
    try {
      const [nextSummary, nextItems] = await Promise.all([
        request<NotificationSummary>('/api/notifications/v1/summary'),
        request<NotificationListResponse>(`/api/notifications/v1/items?status=${status}&limit=10`),
      ]);
      if (!mountedRef.current) return;
      setSummary(nextSummary);
      setItems(nextItems.items);
      setError(false);
    } catch {
      if (mountedRef.current) setError(true);
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, [enabled, request, status]);

  useEffect(() => {
    if (!enabled) {
      setSummary(emptySummary);
      setItems([]);
      setLoading(false);
      setError(false);
      return undefined;
    }

    void refresh();
    const interval = window.setInterval(() => {
      if (document.visibilityState === 'visible') void refresh();
    }, pollIntervalMs);

    const handleVisibility = () => {
      if (document.visibilityState === 'visible') void refresh();
    };
    document.addEventListener('visibilitychange', handleVisibility);
    return () => {
      window.clearInterval(interval);
      document.removeEventListener('visibilitychange', handleVisibility);
    };
  }, [enabled, pollIntervalMs, refresh]);

  const markAllRead = useCallback(async () => {
    await request<void>('/api/notifications/v1/items/read-all', { method: 'POST' });
    await refresh();
  }, [refresh, request]);

  const openNotification = useCallback(
    async (item: NotificationItem) => {
      await request<void>(`/api/notifications/v1/items/${item.id}/read`, { method: 'POST' });
      await refresh();
      if (item.deepLink) window.location.assign(item.deepLink);
    },
    [refresh, request],
  );

  const archive = useCallback(
    async (item: NotificationItem) => {
      await request<void>(`/api/notifications/v1/items/${item.id}/archive`, { method: 'POST' });
      await refresh();
    },
    [refresh, request],
  );

  return {
    summary,
    items,
    loading,
    error,
    refresh,
    markAllRead,
    openNotification,
    archive,
  };
}

function sendNotificationFetch(path: string, token: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  headers.set('Authorization', `Bearer ${token}`);

  return fetch(path, {
    ...init,
    headers,
  });
}

function formatNotificationTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '--:--';
  return new Intl.DateTimeFormat(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}
