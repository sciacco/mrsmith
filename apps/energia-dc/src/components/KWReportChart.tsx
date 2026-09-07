import {
  Area,
  AreaChart,
  CartesianGrid,
  LabelList,
  ReferenceLine,
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
  unit: 'kW' | 'A';
  series: KWReportPoint[];
}

function bucketLabel(bucket: string) {
  if (bucket.length === 7) return `${bucket.slice(5)}/${bucket.slice(0, 4)}`;
  return formatLocalDate(bucket) ?? bucket;
}

// Media aritmetica delle letture valide; null se la serie è vuota.
export function kwSeriesMean(series: KWReportPoint[]): number | null {
  const values = series
    .map((point) => point.kilowatt)
    .filter(
      (value): value is number => value !== null && Number.isFinite(value),
    );
  if (values.length === 0) return null;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

// L'asse Y è centrato sull'intervallo dei dati: il grafico a linea non richiede
// baseline zero, quindi differenze di pochi kW restano leggibili invece di
// appiattirsi in cima alla scala. Pad + arrotondamento a passi da 0,5 kW.
function zoomedYDomain(values: number[]): [number, number] {
  const min = Math.min(...values);
  const max = Math.max(...values);
  const pad = Math.max((max - min) * 0.1, 0.25);
  return [Math.floor((min - pad) * 2) / 2, Math.ceil((max + pad) * 2) / 2];
}

function formatKw1(value: number) {
  return formatNumber(value, { format: { maximumFractionDigits: 1 } });
}

function formatKw2(value: number) {
  return formatNumber(value, {
    format: { minimumFractionDigits: 2, maximumFractionDigits: 2 },
  });
}

// Tooltip custom: recharts 2.15 non supporta il formatter multi-riga, e il
// layout di default comprime valore ed etichetta sulla stessa riga.
function KWTooltip({
  active,
  payload,
  label,
  mean,
  unit,
}: {
  active?: boolean;
  payload?: Array<{ value?: number | string | null }>;
  label?: string | number;
  mean: number | null;
  unit: 'kW' | 'A';
}) {
  if (!active || !payload?.length) return null;
  const raw = payload[0]?.value;
  if (typeof raw !== 'number' || !Number.isFinite(raw)) return null;
  const delta = mean !== null ? raw - mean : null;
  const deltaSign = delta !== null && delta > 0 ? '+' : '';
  return (
    <div className={styles.tooltip}>
      <div className={styles.tooltipTitle}>{label}</div>
      <div className={styles.tooltipValue}>{formatKw2(raw)} {unit}</div>
      <div className={styles.tooltipMeta}>Valore medio</div>
      {delta !== null && (
        <div className={styles.tooltipMeta}>
          {deltaSign}
          {formatKw2(delta)} {unit} vs media periodo
        </div>
      )}
    </div>
  );
}

export function KWReportChart({
  series,
  fixed = false,
  unit = 'kW',
}: {
  series: KWReportPoint[];
  fixed?: boolean;
  unit?: 'kW' | 'A';
}) {
  const data = series.map((point) => ({
    ...point,
    label: bucketLabel(point.bucket),
  }));
  const dense = series.length > 12;
  const values = series
    .map((point) => point.kilowatt)
    .filter(
      (value): value is number => value !== null && Number.isFinite(value),
    );
  const mean = kwSeriesMean(series);
  function valueLabel({
    x,
    y,
    width,
    value,
  }: {
    x?: number | string;
    y?: number | string;
    width?: number | string;
    value?: unknown;
  }) {
    if (typeof value !== 'number' || !Number.isFinite(value)) return <g />;
    // Le barre passano x = bordo sinistro + width; la linea/area passa x già
    // centrato sul punto e nessuna width.
    const widthNum = Number(width);
    const left = Number.isFinite(widthNum)
      ? Number(x) + widthNum / 2
      : Number(x);
    const top = Number(y) - 8;
    return (
      <text
        x={left}
        y={top}
        fill="var(--color-text-muted)"
        fontSize={11}
        textAnchor={dense ? 'start' : 'middle'}
        transform={dense ? `rotate(-45 ${left} ${top})` : undefined}
      >
        {formatNumber(value, {
          format: { minimumFractionDigits: 2, maximumFractionDigits: 2 },
        })}
      </text>
    );
  }
  const chart = (
    <AreaChart
      width={fixed ? 1100 : undefined}
      height={fixed ? 420 : undefined}
      data={data}
      accessibilityLayer
      margin={{ top: 60, right: 48, bottom: 12, left: 12 }}
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
        domain={values.length > 0 ? zoomedYDomain(values) : undefined}
        tick={{ fill: 'var(--color-text-muted)', fontSize: 12 }}
        tickFormatter={(value: number) =>
          formatNumber(value, { format: { maximumFractionDigits: 1 } }) ?? ''
        }
        width={70}
      />
      {mean !== null && (
        // L'ancora visiva della media; il valore è annotato nella legenda sopra
        // il grafico e nella didascalia dell'export (qui collide con le label
        // dei punti, che per costruzione abitano la stessa banda dell'asse).
        <ReferenceLine
          y={mean}
          stroke="var(--color-text-faint)"
          strokeDasharray="4 4"
        />
      )}
      {!fixed && (
        <Tooltip
          cursor={{ stroke: 'var(--color-border)' }}
          content={<KWTooltip mean={mean} unit={unit} />}
        />
      )}
      <Area
        type="monotone"
        dataKey="kilowatt"
        name={`${unit} medi`}
        stroke="var(--color-accent)"
        strokeWidth={2}
        fill="var(--color-accent)"
        fillOpacity={0.08}
        // Con meno di tre letture la linea non ha un profilo leggibile:
        // si mostra il punto invece della sola linea.
        dot={
          series.length < 3
            ? { r: 4, fill: 'var(--color-accent)', strokeWidth: 0 }
            : false
        }
        connectNulls={false}
        isAnimationActive={false}
      >
        <LabelList dataKey="kilowatt" content={valueLabel} />
      </Area>
    </AreaChart>
  );
  if (fixed) return chart;
  return (
    <div>
      {mean !== null && (
        <div className={styles.legend}>
          <span className={styles.legendItem}>
            <i className={styles.legendLine} aria-hidden="true" />
            {unit} medi
          </span>
          <span className={styles.legendItem}>
            <i className={styles.legendDash} aria-hidden="true" />
            media periodo {formatKw1(mean)} {unit}
          </span>
        </div>
      )}
      <div
        className={styles.scroller}
        role="region"
        tabIndex={0}
        aria-label={`Andamento del valore medio in ${unit}; gli intervalli senza letture non hanno valore.`}
      >
      <div
        className={styles.chart}
        style={{ minWidth: Math.max(560, series.length * 36) }}
      >
          <ResponsiveContainer width="100%" height="100%">
            {chart}
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}
