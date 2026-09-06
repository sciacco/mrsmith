import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { formatLocalDate, formatNumber } from '@mrsmith/format';
import type { KWReportPoint } from '../api/types';
import styles from './KWReportChart.module.css';

export interface KWReportChartSpec {
  key: string;
  title: string;
  customer: string;
  period: string;
  series: KWReportPoint[];
}

function bucketLabel(bucket: string) {
  if (bucket.length === 7) return `${bucket.slice(5)}/${bucket.slice(0, 4)}`;
  return formatLocalDate(bucket) ?? bucket;
}

export function KWReportChart({
  series,
  fixed = false,
}: {
  series: KWReportPoint[];
  fixed?: boolean;
}) {
  const data = series.map((point) => ({
    ...point,
    label: bucketLabel(point.bucket),
  }));
  const chart = (
    <BarChart
      width={fixed ? 1100 : undefined}
      height={fixed ? 420 : undefined}
      data={data}
      accessibilityLayer
      margin={{ top: 20, right: 24, bottom: 12, left: 12 }}
    >
      <CartesianGrid
        strokeDasharray="3 3"
        vertical={false}
        stroke="var(--color-border-subtle)"
      />
      <XAxis
        dataKey="label"
        tick={{ fill: 'var(--color-text-muted)', fontSize: 12 }}
        minTickGap={20}
      />
      <YAxis
        tick={{ fill: 'var(--color-text-muted)', fontSize: 12 }}
        tickFormatter={(value: number) => formatNumber(value) ?? ''}
        width={70}
      />
      {!fixed && (
        <Tooltip
          formatter={(value: number) => [
            `${formatNumber(value, { format: { maximumFractionDigits: 2 } })} kW`,
            'Potenza media',
          ]}
          contentStyle={{
            background: 'var(--color-bg-elevated)',
            borderColor: 'var(--color-border)',
            borderRadius: 'var(--radius-md)',
          }}
        />
      )}
      <Bar
        dataKey="kilowatt"
        name="kW medi"
        fill="var(--color-accent)"
        isAnimationActive={false}
      />
    </BarChart>
  );
  if (fixed) return chart;
  return (
    <div
      className={styles.chart}
      role="img"
      aria-label="Andamento della potenza media in kW; gli intervalli senza letture non hanno valore."
    >
      <ResponsiveContainer width="100%" height="100%">
        {chart}
      </ResponsiveContainer>
    </div>
  );
}
