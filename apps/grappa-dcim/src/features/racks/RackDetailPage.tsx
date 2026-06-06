import { Icon, Skeleton } from '@mrsmith/ui';
import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useEquipment, useRackDetail, useRackPowerSummary } from '../../api/queries';
import type { EquipmentItem, RackDetail, RackPowerSummaryPoint, RackSocket } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import {
  buildSlotEntries,
  buildSparklinePath,
  formatDate,
  formatRelativeTime,
  type SlotEntry,
} from './rackDetailHelpers';
import styles from './rackDetail.module.css';

const powerFmt = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 2 });

export function RackDetailPage() {
  const { rackId: rackIdStr } = useParams<{ rackId: string }>();
  const rackId = rackIdStr && !Number.isNaN(Number(rackIdStr)) ? Number(rackIdStr) : null;

  const rackQuery = useRackDetail(rackId);
  const powerSummaryQuery = useRackPowerSummary(rackId);
  const equipmentQuery = useEquipment({ rackId });

  if (rackQuery.isLoading) {
    return (
      <main className={styles.page}>
        <div className={styles.skeletonWrap}>
          <Skeleton rows={5} />
        </div>
      </main>
    );
  }

  if (rackQuery.error || !rackQuery.data) {
    return (
      <main className={styles.page}>
        <ViewState
          title="Rack non trovato"
          message="Non è stato possibile caricare il rack richiesto."
          tone="error"
        />
      </main>
    );
  }

  const rack = rackQuery.data;
  const powerSummary = powerSummaryQuery.data ?? [];
  const equipment = equipmentQuery.data ?? [];

  return (
    <main className={styles.page}>
      <RackDetailHeader rack={rack} />
      <div className={styles.body}>
        <RackColumn rack={rack} equipment={equipment} />
        <aside className={styles.sidebar}>
          <RackSidebar rack={rack} powerSummary={powerSummary} />
        </aside>
      </div>
    </main>
  );
}

function RackDetailHeader({ rack }: { rack: RackDetail }) {
  const powerPercent =
    rack.soldPower && rack.committedPower
      ? Math.min(100, Math.round((rack.committedPower / rack.soldPower) * 100))
      : null;

  const metaParts = [
    rack.customerName,
    rack.orderCode,
    rack.serialNumber,
    rackFormatLabel(rack.type, rack.position),
    rack.unitCount ? `${rack.unitCount}U` : null,
  ].filter((v): v is string => Boolean(v));

  return (
    <header className={styles.detailHeader}>
      <nav className={styles.breadcrumb} aria-label="Navigazione rack">
        <Link to="/rack" className={styles.breadcrumbBack}>
          <Icon name="arrow-left" size={13} />
          Rack
        </Link>
        {rack.datacenterName ? (
          <>
            <span className={styles.breadcrumbSep} aria-hidden="true">
              ›
            </span>
            <span>{rack.datacenterName}</span>
          </>
        ) : null}
        {rack.island ? (
          <>
            <span className={styles.breadcrumbSep} aria-hidden="true">
              ›
            </span>
            <span>Isola {rack.island}</span>
          </>
        ) : null}
      </nav>

      <div className={styles.titleRow}>
        <h1 className={styles.rackName}>{rack.name}</h1>
        <RackStatusBadge status={rack.status} />
      </div>

      {metaParts.length > 0 ? <div className={styles.metaRow}>{metaParts.join(' · ')}</div> : null}

      {powerPercent !== null && rack.committedPower !== undefined && rack.soldPower !== undefined ? (
        <div className={styles.powerBarWrap}>
          <div className={styles.powerBarTrack}>
            <div className={styles.powerBarFill} style={{ width: `${powerPercent}%` }} />
          </div>
          <span className={styles.powerLabel}>
            Impegnata {powerFmt.format(rack.committedPower)} / {powerFmt.format(rack.soldPower)} kW
          </span>
        </div>
      ) : null}

      {rack.activatedAt ? (
        <div className={styles.activatedRow}>Attivato: {formatDate(rack.activatedAt)}</div>
      ) : null}
    </header>
  );
}

function RackStatusBadge({ status }: { status?: string }) {
  const text = status?.trim() || 'Non indicato';
  const isDanger = ['cessato', 'cessata', 'spento', 'chiuso'].includes(text.toLowerCase());
  return <span className={isDanger ? styles.badgeDanger : styles.badgeActive}>{text}</span>;
}

function RackColumn({ rack, equipment }: { rack: RackDetail; equipment: EquipmentItem[] }) {
  const [side, setSide] = useState<'front' | 'back'>('front');
  const [expandedRuns, setExpandedRuns] = useState<Set<number>>(new Set());

  const deviceByUnit = new Map<number, EquipmentItem>();
  for (const eq of equipment) {
    const pos = eq.unitPosition ?? eq.unit;
    if (pos !== undefined && pos !== null) deviceByUnit.set(pos, eq);
  }

  const unitCount = rack.unitCount || 42;
  // U01 is physically at the bottom — render highest unit numbers first
  const entries = buildSlotEntries(rack.units, unitCount).reverse();
  const hasMedia = rack.media.some((m) => m.side === side && m.path);

  function toggleRun(from: number) {
    setExpandedRuns((prev) => {
      const next = new Set(prev);
      if (next.has(from)) next.delete(from);
      else next.add(from);
      return next;
    });
  }

  return (
    <section className={styles.columnSection}>
      <div className={styles.columnToolbar}>
        <span className={styles.columnLabel}>Unità rack — {unitCount}U</span>
        <div className={styles.sideToggle}>
          <button
            type="button"
            className={`${styles.sideBtn} ${side === 'front' ? styles.sideBtnActive : ''}`}
            onClick={() => setSide('front')}
          >
            Fronte
          </button>
          <button
            type="button"
            className={`${styles.sideBtn} ${side === 'back' ? styles.sideBtnActive : ''}`}
            onClick={() => setSide('back')}
          >
            Retro
          </button>
        </div>
      </div>

      <div className={styles.unitTable}>
        <div className={styles.unitHeader}>
          <span>U</span>
          <span>Dispositivo</span>
          {hasMedia ? <span>Foto</span> : null}
        </div>

        {entries.map((entry) => {
          if (entry.kind === 'empty-run') {
            return (
              <EmptyRunRow
                key={`run-${entry.from}`}
                entry={entry}
                expanded={expandedRuns.has(entry.from)}
                hasMedia={hasMedia}
                reversed
                onToggle={() => toggleRun(entry.from)}
              />
            );
          }

          if (entry.kind === 'empty-single') {
            return (
              <div key={`unit-${entry.unitNum}`} className={styles.unitRowFree}>
                <span className={styles.unitNum}>U{pad(entry.unitNum)}</span>
                <span className={styles.unitEmpty}>— libero —</span>
                {hasMedia ? <span /> : null}
              </div>
            );
          }

          const device = deviceByUnit.get(entry.unitNum);
          const unitId = entry.unit.id;
          const unitMedia = rack.media.filter((m) => m.unitId === unitId && m.side === side);
          const mediaItem = unitMedia[0];

          return (
            <div key={`unit-${entry.unitNum}`} className={styles.unitRowOccupied}>
              <span className={styles.unitNum}>U{pad(entry.unitNum)}</span>
              <span className={styles.unitDevice}>
                {device?.name ?? `Apparato #${entry.unit.deviceId ?? entry.unit.id}`}
              </span>
              {hasMedia ? (
                <span className={styles.unitMedia}>
                  {mediaItem?.path ? (
                    <a href={mediaItem.path} target="_blank" rel="noreferrer" className={styles.mediaLink}>
                      <Icon name="eye" size={13} />
                    </a>
                  ) : (
                    <span className={styles.mediaEmpty}>—</span>
                  )}
                </span>
              ) : null}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function EmptyRunRow({
  entry,
  expanded,
  hasMedia,
  reversed = false,
  onToggle,
}: {
  entry: Extract<SlotEntry, { kind: 'empty-run' }>;
  expanded: boolean;
  hasMedia: boolean;
  reversed?: boolean;
  onToggle: () => void;
}) {
  const label = reversed
    ? `U${pad(entry.to)}–U${pad(entry.from)}`
    : `U${pad(entry.from)}–U${pad(entry.to)}`;

  const expandedSlots = Array.from({ length: entry.count }, (_, i) => {
    const unitNum = reversed ? entry.to - i : entry.from + i;
    return (
      <div key={unitNum} className={styles.unitRowFree}>
        <span className={styles.unitNum}>U{pad(unitNum)}</span>
        <span className={styles.unitEmpty}>— libero —</span>
        {hasMedia ? <span /> : null}
      </div>
    );
  });

  return (
    <>
      <button type="button" className={styles.emptyRun} onClick={onToggle} aria-expanded={expanded}>
        <span className={styles.unitNum}>{label}</span>
        <span className={styles.emptyRunLabel}>{entry.count} unità libere</span>
        {hasMedia ? <span /> : null}
        <Icon name={expanded ? 'chevron-up' : 'chevron-down'} size={13} />
      </button>
      {expanded && expandedSlots}
    </>
  );
}

function RackSidebar({ rack, powerSummary }: { rack: RackDetail; powerSummary: RackPowerSummaryPoint[] }) {
  return (
    <>
      <PowerCard sockets={rack.sockets} />
      <SparklineCard points={powerSummary} />
      <AnagraphicCard rack={rack} />
    </>
  );
}

function PowerCard({ sockets }: { sockets: RackSocket[] }) {
  if (sockets.length === 0) {
    return (
      <div className={styles.sideCard}>
        <h2 className={styles.sideCardTitle}>Potenza</h2>
        <p className={styles.sideCardEmpty}>Nessun socket configurato</p>
      </div>
    );
  }

  const latestReading = [...sockets]
    .map((s) => s.latestReadingAt)
    .filter((d): d is string => Boolean(d))
    .sort()
    .at(-1);

  return (
    <div className={styles.sideCard}>
      <div className={styles.sideCardHeader}>
        <h2 className={styles.sideCardTitle}>Potenza</h2>
        {latestReading ? <span className={styles.sideCardMeta}>{formatRelativeTime(latestReading)}</span> : null}
      </div>
      <div className={styles.socketList}>
        {sockets.map((socket) => (
          <div key={socket.id} className={styles.socketRow}>
            <span className={isSocketOn(socket.status, socket.latestAmpere) ? styles.socketOn : styles.socketOff} />
            <span className={styles.socketName}>
              {socket.magnetotermico || socket.position || `Socket ${socket.id}`}
            </span>
            {socket.latestAmpere !== undefined ? (
              <span className={styles.socketAmpere}>{powerFmt.format(socket.latestAmpere)} A</span>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

function SparklineCard({ points }: { points: RackPowerSummaryPoint[] }) {
  const recent = points.slice(-7);
  const path = buildSparklinePath(recent, 100, 28);
  const values = recent.filter((p) => p.kilowatt !== undefined).map((p) => p.kilowatt!);

  return (
    <div className={styles.sideCard}>
      <h2 className={styles.sideCardTitle}>kWh ultimi 7 gg</h2>
      {path ? (
        <div className={styles.sparklineWrap}>
          <span className={styles.sparklineLabel}>{powerFmt.format(Math.min(...values))}</span>
          <svg viewBox="0 0 100 28" className={styles.sparkline} aria-hidden="true">
            <polyline
              points={path}
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <span className={styles.sparklineLabel}>{powerFmt.format(Math.max(...values))}</span>
        </div>
      ) : (
        <p className={styles.sideCardEmpty}>Dati non disponibili</p>
      )}
    </div>
  );
}

function AnagraphicCard({ rack }: { rack: RackDetail }) {
  const rows: Array<[string, string | undefined | null]> = [
    ['Seriale', rack.serialNumber],
    ['Ordine', rack.orderCode],
    ['Formato', rackFormatLabel(rack.type, rack.position)],
    ['Piano', rack.floor !== undefined ? String(rack.floor) : undefined],
    ['Isola', rack.island !== undefined ? String(rack.island) : undefined],
    ['Fatturazione', rack.variableBilling === 1 ? 'Variabile' : 'Fissa'],
    ['Note', rack.note],
  ];

  return (
    <div className={styles.sideCard}>
      <h2 className={styles.sideCardTitle}>Anagrafica</h2>
      <dl className={styles.anagraphicGrid}>
        {rows
          .filter(([, v]) => v !== undefined && v !== null)
          .map(([label, value]) => (
            <div key={label} className={styles.anagraphicRow}>
              <dt>{label}</dt>
              <dd>{value || '—'}</dd>
            </div>
          ))}
      </dl>
    </div>
  );
}

function isSocketOn(status: string, latestAmpere?: number): boolean {
  if (status === 'Acceso') return true;
  if (status === 'Spento') return false;
  return latestAmpere !== undefined && latestAmpere > 0;
}

function rackFormatLabel(type?: string, position?: string) {
  if (type === 'Half' && position === 'A') return '1/2 alto';
  if (type === 'Half' && position === 'B') return '1/2 basso';
  if (type === 'Full') return 'Full';
  return [type, position].filter(Boolean).join(' ') || 'Formato non indicato';
}

function pad(n: number) {
  return String(n).padStart(2, '0');
}
