import { Button, Icon } from '@mrsmith/ui';
import type { OrderDetail } from '../api/types';
import { formatDate, formatServiceTypes, orderCode } from '../lib/formatters';
import { StatusBadge } from './StatusBadge';
import styles from '../pages/OrderDetailPage.module.css';

interface DetailHeaderProps {
  order: OrderDetail;
  canKickoff: boolean;
  canActivationForm: boolean;
  canOrderPdf: boolean;
  canSignedPdf: boolean;
  downloading: string | null;
  onBack: () => void;
  onDownload: (kind: 'kickoff' | 'activation' | 'order' | 'signed') => void;
}

export function DetailHeader({
  order,
  canKickoff,
  canActivationForm,
  canOrderPdf,
  canSignedPdf,
  downloading,
  onBack,
  onDownload,
}: DetailHeaderProps) {
  return (
    <section className={styles.detailHeader}>
      <div className={styles.detailHeaderTop}>
        <button type="button" className={styles.backButton} onClick={onBack}>
          <Icon name="arrow-left" size={16} />
          Torna agli ordini
        </button>
        <div className={styles.pdfActions}>
          <Button variant="secondary" size="sm" disabled={!canKickoff} loading={downloading === 'kickoff'} onClick={() => onDownload('kickoff')}>
            Kickoff
          </Button>
          <Button variant="secondary" size="sm" disabled={!canActivationForm} loading={downloading === 'activation'} onClick={() => onDownload('activation')}>
            Modulo di attivazione
          </Button>
          <Button variant="secondary" size="sm" disabled={!canOrderPdf} loading={downloading === 'order'} onClick={() => onDownload('order')}>
            PDF ordine
          </Button>
          <Button variant="secondary" size="sm" disabled={!canSignedPdf} loading={downloading === 'signed'} onClick={() => onDownload('signed')}>
            Ordine firmato
          </Button>
        </div>
      </div>
      <div className={styles.detailTitleRow}>
        <div className={styles.detailTitleCopy}>
          <h1>Codice ordine: {orderCode(order.cdlan_ndoc, order.cdlan_anno)}</h1>
          <p>{order.cdlan_cliente ?? 'Ragione sociale non indicata'}</p>
        </div>
        <StatusBadge state={order.cdlan_stato} />
      </div>
      <div className={styles.headerFacts}>
        <span><strong>Data proposta</strong>{formatDate(order.cdlan_datadoc)}</span>
        <span><strong>Data conferma</strong>{formatDate(order.cdlan_dataconferma)}</span>
        <span><strong>Tipo servizi</strong>{formatServiceTypes(order.service_type, order.is_colo)}</span>
        {order.origin ? (
          <a href={order.origin.quote_url} className={styles.originLink}>
            Da proposta {order.origin.quote_code ?? order.origin.quote_id}
          </a>
        ) : null}
      </div>
    </section>
  );
}
