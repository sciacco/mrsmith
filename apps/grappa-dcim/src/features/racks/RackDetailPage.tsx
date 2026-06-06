import { Icon, Skeleton } from '@mrsmith/ui';
import { Link, useParams } from 'react-router-dom';
import { useEquipment, useRackDetail, useRackPowerSummary } from '../../api/queries';
import type { EquipmentItem, RackDetail, RackPowerSummaryPoint, RackSocket } from '../../api/types';
import { ViewState } from '../../components/ViewState';
import {
  buildSparklinePath,
  formatDate,
  formatRelativeTime,
} from './rackDetailHelpers';
import { EquipmentTypeBadge } from '../equipment/EquipmentTypeBadge';
import { buildRackUnitMap, normalizeRackUnitCount } from './rackUnitMap';
import styles from './rackDetail.module.css';

const powerFmt = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 2 });

export function RackDetailPage() {
  const { rackId: rackIdStr } = useParams<{ rackId: string }>();
  const rackId = rackIdStr && !Number.isNaN(Number(rackIdStr)) ? Number(rackIdStr) : null;

  const rackQuery = useRackDetail(rackId);
  const powerSummaryQuery = useRackPowerSummary(rackId);
  const equipmentQuery = useEquipment({ rackId, status: 'occupancy' });

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
  const unitCount = normalizeRackUnitCount(rack.unitCount);
  const unitRows = buildRackUnitMap(unitCount, equipment);

  return (
    <section className={styles.columnSection}>
      <div className={styles.columnToolbar}>
        <span className={styles.columnLabel}>Unità rack — {unitCount}U</span>
      </div>

      <div className={styles.unitTable}>
        <div className={styles.unitHeader}>
          <span>U</span>
          <span>Dispositivo</span>
        </div>

        <div className={styles.unitGrid}>
          {unitRows.map((row) => (
            <UnitGridRow key={`unit-${row.unitNum}`} row={row} />
          ))}
        </div>
      </div>
    </section>
  );
}

function UnitGridRow({ row }: { row: ReturnType<typeof buildRackUnitMap>[number] }) {
  return (
    <>
      <span className={styles.unitNumCell}>U{pad(row.unitNum)}</span>
      {row.kind === 'free' ? <span className={styles.unitFreeCell} /> : null}
      {row.kind === 'device' ? (
        <div className={styles.unitDeviceBlock} style={{ gridRow: `span ${row.span}` }}>
          <span className={styles.unitDeviceInfo}>
            <span className={styles.unitDeviceMain}>{row.device.name}</span>
            <EquipmentTypeBadge type={row.device.type} visual={row.device.typeVisual} compact />
          </span>
          <span className={styles.unitDeviceMeta}>
            {row.span > 1 ? `U${pad(row.startUnit)}-U${pad(row.endUnit)} · ${row.span}U` : `U${pad(row.startUnit)}`}
          </span>
        </div>
      ) : null}
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
