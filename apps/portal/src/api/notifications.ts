import type { ApiClient } from '@mrsmith/api-client';

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

export async function fetchNotificationSummary(api: ApiClient): Promise<NotificationSummary> {
  return api.get<NotificationSummary>('/notifications/v1/summary');
}

export async function fetchNotificationItems(api: ApiClient): Promise<NotificationItem[]> {
  const response = await api.get<NotificationListResponse>('/notifications/v1/items?status=all&limit=10');
  return response.items;
}

export async function markNotificationRead(api: ApiClient, id: number): Promise<void> {
  await api.post(`/notifications/v1/items/${id}/read`);
}

export async function markAllNotificationsRead(api: ApiClient): Promise<void> {
  await api.post('/notifications/v1/items/read-all');
}

export async function archiveNotification(api: ApiClient, id: number): Promise<void> {
  await api.post(`/notifications/v1/items/${id}/archive`);
}
