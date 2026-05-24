import { useEffect, useRef, useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import type { OrderDetail } from '../api/types';
import { formatDate, formatServiceTypes, orderCode } from '../lib/formatters';
import { StatusBadge } from './StatusBadge';
import { OrderTimeline } from './OrderTimeline';
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
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, []);

  const handleDownload = (kind: 'kickoff' | 'activation' | 'order' | 'signed') => {
    setIsOpen(false);
    onDownload(kind);
  };

  return (
    <section className={styles.detailHeader}>
      <div className={styles.detailHeaderTop}>
        <div className={styles.detailHeaderLeft}>
          <button type="button" className={styles.backButton} onClick={onBack}>
            <Icon name="arrow-left" size={16} />
            Torna agli ordini
          </button>
          <div className={styles.headerMetaGroup}>
            <span className={styles.headerOrderCode}>
              Codice ordine: <strong className={styles.codeCell}>{orderCode(order.cdlan_ndoc, order.cdlan_anno)}</strong>
            </span>
            <StatusBadge state={order.cdlan_stato} />
          </div>
        </div>
        <div className={styles.detailHeaderRight}>
          {order.arx_doc_number ? (
            <a
              className={styles.arxivarHeaderLink}
              href={`https://arxivar.cdlan.it/#!/view/${encodeURIComponent(order.arx_doc_number)}`}
              target="_blank"
              rel="noreferrer"
            >
              <Icon name="external-link" size={14} />
              Arxivar
            </a>
          ) : null}
          <div className={styles.dropdownContainer} ref={dropdownRef}>
            <Button variant="secondary" size="sm" rightIcon={<Icon name="chevron-down" size={14} />} onClick={() => setIsOpen(!isOpen)}>
              Documenti
            </Button>
            {isOpen && (
              <div className={styles.dropdownMenu}>
                <button
                  type="button"
                  className={styles.dropdownItem}
                  disabled={!canKickoff || downloading != null}
                  onClick={() => handleDownload('kickoff')}
                >
                  {downloading === 'kickoff' ? (
                    <Icon name="loader" size={16} className={styles.dropdownLoader} />
                  ) : (
                    <Icon name="file-plus" size={16} className={styles.dropdownItemIcon} />
                  )}
                  <span>Kickoff</span>
                </button>
                <button
                  type="button"
                  className={styles.dropdownItem}
                  disabled={!canActivationForm || downloading != null}
                  onClick={() => handleDownload('activation')}
                >
                  {downloading === 'activation' ? (
                    <Icon name="loader" size={16} className={styles.dropdownLoader} />
                  ) : (
                    <Icon name="clipboard-check" size={16} className={styles.dropdownItemIcon} />
                  )}
                  <span>Modulo di attivazione</span>
                </button>
                <button
                  type="button"
                  className={styles.dropdownItem}
                  disabled={!canOrderPdf || downloading != null}
                  onClick={() => handleDownload('order')}
                >
                  {downloading === 'order' ? (
                    <Icon name="loader" size={16} className={styles.dropdownLoader} />
                  ) : (
                    <Icon name="file-text" size={16} className={styles.dropdownItemIcon} />
                  )}
                  <span>PDF ordine</span>
                </button>
                <button
                  type="button"
                  className={styles.dropdownItem}
                  disabled={!canSignedPdf || downloading != null}
                  onClick={() => handleDownload('signed')}
                >
                  {downloading === 'signed' ? (
                    <Icon name="loader" size={16} className={styles.dropdownLoader} />
                  ) : (
                    <Icon name="file-check" size={16} className={styles.dropdownItemIcon} />
                  )}
                  <span>Ordine firmato</span>
                </button>
              </div>
            )}
          </div>
        </div>
      </div>
      <div className={styles.detailTitleRow}>
        <div className={styles.detailTitleCopy}>
          <h1>{order.cdlan_cliente ?? 'Ragione sociale non indicata'}</h1>
        </div>
      </div>

      <OrderTimeline state={order.cdlan_stato} hasArxDoc={Boolean(order.arx_doc_number)} />

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
