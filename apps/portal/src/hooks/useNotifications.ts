import type { ApiClient } from '@mrsmith/api-client';
import { useCallback, useEffect, useState } from 'react';
import {
  archiveNotification,
  fetchNotificationItems,
  fetchNotificationSummary,
  markAllNotificationsRead,
  markNotificationRead,
  type NotificationItem,
  type NotificationSummary,
} from '../api/notifications';

const POLL_INTERVAL_MS = 60_000;

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

export function useNotifications(api: ApiClient, enabled: boolean): NotificationsState {
  const [summary, setSummary] = useState<NotificationSummary>(emptySummary);
  const [items, setItems] = useState<NotificationItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  const refresh = useCallback(async () => {
    if (!enabled || document.visibilityState !== 'visible') return;
    setLoading(true);
    try {
      const [nextSummary, nextItems] = await Promise.all([
        fetchNotificationSummary(api),
        fetchNotificationItems(api),
      ]);
      setSummary(nextSummary);
      setItems(nextItems);
      setError(false);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [api, enabled]);

  useEffect(() => {
    if (!enabled) {
      setSummary(emptySummary);
      setItems([]);
      setLoading(false);
      setError(false);
      return undefined;
    }

    let cancelled = false;
    const run = async () => {
      if (cancelled) return;
      await refresh();
    };

    void run();
    const interval = window.setInterval(() => {
      if (document.visibilityState === 'visible') void run();
    }, POLL_INTERVAL_MS);

    const handleVisibility = () => {
      if (document.visibilityState === 'visible') void run();
    };
    document.addEventListener('visibilitychange', handleVisibility);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
      document.removeEventListener('visibilitychange', handleVisibility);
    };
  }, [enabled, refresh]);

  const markAllRead = useCallback(async () => {
    await markAllNotificationsRead(api);
    await refresh();
  }, [api, refresh]);

  const openNotification = useCallback(
    async (item: NotificationItem) => {
      await markNotificationRead(api, item.id);
      await refresh();
      if (item.deepLink) {
        window.location.assign(item.deepLink);
      }
    },
    [api, refresh],
  );

  const archive = useCallback(
    async (item: NotificationItem) => {
      await archiveNotification(api, item.id);
      await refresh();
    },
    [api, refresh],
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
