export interface TimooDailyStat {
  tenant_id: number;
  tenant_name: string;
  day: string;
  users: number;
  service_extensions: number;
}

export const timooDailyStatsKeys = {
  all: ['reports', 'timoo-daily-stats'] as const,
};
