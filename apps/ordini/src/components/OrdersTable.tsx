import { Button, Icon } from '@mrsmith/ui';
import type { OrderSummary } from '../api/types';
import { formatDate, formatServiceTypes, formatSiNo, formatTipoDoc, formatTipoProposta, orderCode } from '../lib/formatters';
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
            <th>{header('code', 'Codice ordine', sortKey, sortDirection, onSort)}</th>
            <th>{header('customer', 'Ragione sociale', sortKey, sortDirection, onSort)}</th>
            <th>{header('state', 'Stato', sortKey, sortDirection, onSort)}</th>
            <th>{header('date', 'Data proposta', sortKey, sortDirection, onSort)}</th>
            <th>Tipo documento</th>
            <th>Tipo proposta</th>
            <th>Tipo servizi</th>
            <th>Conferma</th>
            <th>Evaso</th>
            <th>Dal CP</th>
            <th>Azioni</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((order, index) => (
            <tr key={order.id} style={{ animationDelay: `${Math.min(index * 16, 260)}ms` }} onDoubleClick={() => onOpen(order.id)}>
              <td>
                <span className={styles.codeCell}>{orderCode(order.cdlan_ndoc, order.cdlan_anno)}</span>
                <small>{order.cdlan_systemodv ? `System ODV ${order.cdlan_systemodv}` : '—'}</small>
              </td>
              <td className={styles.customerCell}>{order.cdlan_cliente ?? '—'}</td>
              <td><StatusBadge state={order.cdlan_stato} /></td>
              <td>{formatDate(order.cdlan_datadoc)}</td>
              <td>{formatTipoDoc(order.cdlan_tipodoc)}</td>
              <td>{formatTipoProposta(order.cdlan_tipo_ord)}</td>
              <td>{formatServiceTypes(order.service_type, order.is_colo)}</td>
              <td>{formatDate(order.cdlan_dataconferma)}</td>
              <td>{formatSiNo(order.cdlan_evaso)}</td>
              <td>{formatSiNo(order.from_cp)}</td>
              <td>
                <Button variant="secondary" size="sm" rightIcon={<Icon name="arrow-right" size={14} />} onClick={() => onOpen(order.id)}>
                  Visualizza
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function header(key: OrderSortKey, label: string, active: OrderSortKey, direction: SortDirection, onSort: (key: OrderSortKey) => void) {
  const isActive = key === active;
  return (
    <button type="button" className={`${styles.sortButton} ${isActive ? styles.sortButtonActive : ''}`} onClick={() => onSort(key)}>
      {label}
      <span aria-hidden="true">{isActive ? (direction === 'asc' ? '▲' : '▼') : '↕'}</span>
    </button>
  );
}
