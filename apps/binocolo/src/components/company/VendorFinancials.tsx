import { Icon } from '@mrsmith/ui';
import type { MATarget } from '../../api/types';
import styles from './CompanyPanels.module.css';

const numberFormat = new Intl.NumberFormat('it-IT');
const moneyFormat = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});

type VendorSheet = {
  year?: number;
  turnover?: number | null;
  employees?: number | null;
  netWorth?: number | null;
  totalAssets?: number | null;
};

function EmptyState({ title, text }: { title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyStateIcon} aria-hidden="true">
        <Icon name="file-text" size={28} />
      </span>
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

function vendorFinancialSheets(target: MATarget): VendorSheet[] {
  const rawPayload = target.vendorPayload;
  const allSheets = rawPayload?.balanceSheets?.all;
  if (Array.isArray(allSheets) && allSheets.length > 0) {
    return [...allSheets].sort((a: VendorSheet, b: VendorSheet) => (a.year ?? 0) - (b.year ?? 0));
  }
  if (target.turnoverYear && (target.turnover != null || target.employees != null)) {
    return [
      {
        year: target.turnoverYear,
        turnover: target.turnover,
        employees: target.employees,
        netWorth: null,
        totalAssets: null,
      },
    ];
  }
  return [];
}

export function hasVendorFinancialsData(target?: MATarget): boolean {
  return Boolean(target && vendorFinancialSheets(target).length > 0);
}

export function vendorFinancialSheetsCount(target?: MATarget): number {
  return target ? vendorFinancialSheets(target).length : 0;
}

export function VendorFinancials({ target }: { target: MATarget }) {
  const sheets = vendorFinancialSheets(target);

  if (sheets.length === 0) {
    return <EmptyState title="Dati finanziari non disponibili" text="Nessun dato di bilancio storico presente per questo target." />;
  }

  const latestTurnoverSheet = [...sheets].reverse().find((sheet) => sheet.turnover != null);
  const latestNetWorthSheet = [...sheets].reverse().find((sheet) => sheet.netWorth != null);
  const latestTotalAssetsSheet = [...sheets].reverse().find((sheet) => sheet.totalAssets != null);
  const latestEmployeesSheet = [...sheets].reverse().find((sheet) => sheet.employees != null);

  const getTrend = (key: 'turnover' | 'netWorth' | 'employees' | 'totalAssets') => {
    const validSheets = sheets.filter((sheet) => sheet[key] != null);
    if (validSheets.length < 2) return null;
    const lastIndex = validSheets.length - 1;
    const lastVal = validSheets[lastIndex]?.[key];
    const prevVal = validSheets[lastIndex - 1]?.[key];
    if (lastVal != null && prevVal != null && prevVal > 0) {
      return ((lastVal - prevVal) / prevVal) * 100;
    }
    return null;
  };

  const formatTrend = (pct: number | null) => {
    if (pct === null) return null;
    const sign = pct >= 0 ? '+' : '';
    const className = pct >= 0 ? styles.finTrendPositive : styles.finTrendNegative;
    return (
      <small className={className}>
        {sign}
        {pct.toLocaleString('it-IT', { maximumFractionDigits: 1 })}% anno su anno
      </small>
    );
  };

  const turnoverSheets = sheets.filter((sheet) => sheet.turnover != null);
  const maxTurnover = Math.max(...turnoverSheets.map((sheet) => sheet.turnover ?? 0), 1);

  // DECISIONE PROPRIETARIO 2026-07-08: il grafico dei bilanci camerali si
  // CONSERVA — deroga esplicita alla regola "no dataviz decorativa".
  // Non rimuovere e non segnalare nei QA.
  const renderBarChart = () => {
    if (turnoverSheets.length === 0) return null;

    const width = 600;
    const height = 200;
    const paddingLeft = 65;
    const paddingRight = 20;
    const paddingTop = 25;
    const paddingBottom = 35;

    const chartWidth = width - paddingLeft - paddingRight;
    const chartHeight = height - paddingTop - paddingBottom;

    const barSpacing = chartWidth / turnoverSheets.length;
    const barWidth = Math.min(barSpacing * 0.5, 45);

    return (
      <div className={styles.chartContainer}>
        <div className={styles.chartTitle}>Andamento fatturato</div>
        <svg viewBox={`0 0 ${width} ${height}`} className={styles.chartSvg} role="img" aria-label="Andamento del fatturato per anno">
          <defs>
            <linearGradient id="vendorFinancialBarGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="var(--color-accent)" />
              <stop offset="100%" stopColor="var(--color-accent-hover)" />
            </linearGradient>
          </defs>

          <line className={styles.chartGridLine} x1={paddingLeft} y1={paddingTop} x2={width - paddingRight} y2={paddingTop} />
          <text className={styles.chartScaleLabel} x={paddingLeft - 10} y={paddingTop + 4} textAnchor="end">
            {moneyFormat.format(maxTurnover)}
          </text>

          <line
            className={styles.chartGridLine}
            x1={paddingLeft}
            y1={paddingTop + chartHeight / 2}
            x2={width - paddingRight}
            y2={paddingTop + chartHeight / 2}
          />
          <text className={styles.chartScaleLabel} x={paddingLeft - 10} y={paddingTop + chartHeight / 2 + 4} textAnchor="end">
            {moneyFormat.format(maxTurnover / 2)}
          </text>

          <line className={styles.chartAxisLine} x1={paddingLeft} y1={paddingTop + chartHeight} x2={width - paddingRight} y2={paddingTop + chartHeight} />
          <text className={styles.chartScaleLabel} x={paddingLeft - 10} y={paddingTop + chartHeight + 4} textAnchor="end">
            0 €
          </text>

          {turnoverSheets.map((sheet, index) => {
            const value = sheet.turnover ?? 0;
            const barHeight = (value / maxTurnover) * chartHeight;
            const x = paddingLeft + index * barSpacing + (barSpacing - barWidth) / 2;
            const y = paddingTop + chartHeight - barHeight;

            return (
              <g key={sheet.year}>
                <rect x={x} y={y} width={barWidth} height={Math.max(barHeight, 2)} rx={4} ry={4} fill="url(#vendorFinancialBarGradient)" />
                <text className={styles.chartValueLabel} x={x + barWidth / 2} y={y - 6} textAnchor="middle">
                  {value >= 1000000 ? `${(value / 1000000).toLocaleString('it-IT', { maximumFractionDigits: 2 })}M` : `${Math.round(value / 1000).toLocaleString('it-IT')}k`}
                </text>
                <text className={styles.chartYearLabel} x={x + barWidth / 2} y={paddingTop + chartHeight + 18} textAnchor="middle">
                  {sheet.year}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  };

  return (
    <div>
      <div className={styles.finCardsGrid}>
        <div className={styles.finCard}>
          <span>Fatturato {latestTurnoverSheet ? `(${latestTurnoverSheet.year})` : ''}</span>
          <strong>{latestTurnoverSheet?.turnover != null ? moneyFormat.format(latestTurnoverSheet.turnover) : '-'}</strong>
          {formatTrend(getTrend('turnover'))}
        </div>
        <div className={styles.finCard}>
          <span>Patrimonio netto {latestNetWorthSheet ? `(${latestNetWorthSheet.year})` : ''}</span>
          <strong>{latestNetWorthSheet?.netWorth != null ? moneyFormat.format(latestNetWorthSheet.netWorth) : '-'}</strong>
          {formatTrend(getTrend('netWorth'))}
        </div>
        <div className={styles.finCard}>
          <span>Attivo totale {latestTotalAssetsSheet ? `(${latestTotalAssetsSheet.year})` : ''}</span>
          <strong>{latestTotalAssetsSheet?.totalAssets != null ? moneyFormat.format(latestTotalAssetsSheet.totalAssets) : '-'}</strong>
          {formatTrend(getTrend('totalAssets'))}
        </div>
        <div className={styles.finCard}>
          <span>Dipendenti {latestEmployeesSheet ? `(${latestEmployeesSheet.year})` : ''}</span>
          <strong>{latestEmployeesSheet?.employees != null ? numberFormat.format(latestEmployeesSheet.employees) : '-'}</strong>
          {formatTrend(getTrend('employees'))}
        </div>
      </div>

      {renderBarChart()}

      <div className={styles.tableContainer}>
        <table className={styles.finTable}>
          <thead>
            <tr>
              <th>Anno</th>
              <th>Fatturato</th>
              <th>Patrimonio netto</th>
              <th>Attivo totale</th>
              <th>Dipendenti</th>
            </tr>
          </thead>
          <tbody>
            {[...sheets].reverse().map((sheet) => (
              <tr key={sheet.year}>
                <td>
                  <strong>{sheet.year}</strong>
                </td>
                <td>{sheet.turnover != null ? moneyFormat.format(sheet.turnover) : '-'}</td>
                <td>{sheet.netWorth != null ? moneyFormat.format(sheet.netWorth) : '-'}</td>
                <td>{sheet.totalAssets != null ? moneyFormat.format(sheet.totalAssets) : '-'}</td>
                <td>{sheet.employees != null ? numberFormat.format(sheet.employees) : '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
