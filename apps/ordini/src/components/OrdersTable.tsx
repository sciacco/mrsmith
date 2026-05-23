import { Button, Icon } from '@mrsmith/ui';
import type { OrderSummary } from '../api/types';
import { formatDate, formatEmpty, formatServiceTypes, formatSiNo, formatTipoDoc, formatTipoProposta, orderCode } from '../lib/formatters';
import { StatusBadge } from './StatusBadge';
import styles from '../pages/OrderListPage.module.css';

export type OrderSortKey = 'id' | 'code' | 'customer' | 'date' | 'state';
export type SortDirection = 'asc' | 'desc';

interface OrdersTableProps {
  rows: OrderSummary[];
  sortKey: OrderSortKey;
  sortDirection: SortDirection;
  onSort: (key: OrderSortKey) => void;
  onOpen: (id: number) => void;
}

export function OrdersTable({ rows, sortKey, sortDirection, onSort, onOpen }: OrdersTableProps) {
  return (
    <div className={styles.tableScroll}>
      <table className={styles.ordersTable}>
        <thead>
          <tr>
            <th>{header('code', 'Codice', sortKey, sortDirection, onSort, 'Ordine')}</th>
            <th className={styles.tabletOptional}>ODV</th>
            <th>{header('customer', 'Cliente', sortKey, sortDirection, onSort)}</th>
            <th>{header('state', 'Stato', sortKey, sortDirection, onSort)}</th>
            <th className={styles.tabletOptional}>{header('date', 'Data prop.', sortKey, sortDirection, onSort)}</th>
            <th className={styles.narrowOptional}>Tipo doc.</th>
            <th className={styles.narrowOptional}>Proposta</th>
            <th className={styles.narrowOptional}>Servizi</th>
            <th className={styles.narrowOptional}>Conf.</th>
            <th className={styles.narrowOptional}>Evaso</th>
            <th className={styles.narrowOptional}>CP</th>
            <th className={styles.narrowOptional}>Sost.</th>
            <th className={styles.narrowOptional}>Lingua</th>
            <th className={styles.narrowOptional}>Doc.</th>
            <th>
              <span className={styles.headerFull}>Azioni</span>
              <span className={styles.headerCompact}>Apri</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((order, index) => (
            <tr key={order.id} style={{ animationDelay: `${Math.min(index * 16, 260)}ms` }} onDoubleClick={() => onOpen(order.id)}>
              <td>
                <span className={styles.codeCell}>{orderCode(order.cdlan_ndoc, order.cdlan_anno)}</span>
              </td>
              <td className={`${styles.monoCell} ${styles.tabletOptional}`}>{formatEmpty(order.cdlan_systemodv)}</td>
              <td className={styles.customerCell}>{order.cdlan_cliente ?? '—'}</td>
              <td><StatusBadge state={order.cdlan_stato} className={styles.compactStatus} /></td>
              <td className={styles.tabletOptional}>{formatDate(order.cdlan_datadoc)}</td>
              <td className={styles.narrowOptional}>{formatTipoDoc(order.cdlan_tipodoc)}</td>
              <td className={styles.narrowOptional}>{formatTipoProposta(order.cdlan_tipo_ord)}</td>
              <td className={styles.narrowOptional}>{formatServiceTypes(order.service_type, order.is_colo)}</td>
              <td className={styles.narrowOptional}>{formatDate(order.cdlan_dataconferma)}</td>
              <td className={styles.narrowOptional}>{formatSiNo(order.cdlan_evaso)}</td>
              <td className={styles.narrowOptional}>{formatSiNo(order.from_cp)}</td>
              <td className={`${styles.monoCell} ${styles.narrowOptional}`}>{formatEmpty(order.cdlan_sost_ord)}</td>
              <td className={styles.narrowOptional}>{formatEmpty(order.profile_lang)}</td>
              <td className={`${styles.monoCell} ${styles.narrowOptional}`}>{formatEmpty(order.arx_doc_number)}</td>
              <td>
                <Button variant="secondary" size="sm" aria-label={`Apri ordine ${orderCode(order.cdlan_ndoc, order.cdlan_anno)}`} rightIcon={<Icon name="arrow-right" size={14} />} onClick={() => onOpen(order.id)}>
                  <span className={styles.actionText}>Apri</span>
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function header(key: OrderSortKey, label: string, active: OrderSortKey, direction: SortDirection, onSort: (key: OrderSortKey) => void, compactLabel = label) {
  const isActive = key === active;
  return (
    <button type="button" className={`${styles.sortButton} ${isActive ? styles.sortButtonActive : ''}`} onClick={() => onSort(key)}>
      <span className={styles.headerFull}>{label}</span>
      <span className={styles.headerCompact}>{compactLabel}</span>
      <span className={styles.sortGlyph} aria-hidden="true">{isActive ? (direction === 'asc' ? '▲' : '▼') : '↕'}</span>
    </button>
  );
}
