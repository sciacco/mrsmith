import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import type { CustomerRef, OrderDetail, SendToERPResponse, UpdateHeaderPayload } from '../api/types';
import { dateInputValue, formatDate, formatDurRin, formatEmpty, formatFatturazione, formatFatturazioneAtt, formatSiNo, formatTipoDoc, formatTipoProposta } from '../lib/formatters';
import { CustomerSelect } from './CustomerSelect';
import { SendToErpResultPanel } from './SendToErpResultPanel';
import styles from '../pages/OrderDetailPage.module.css';

interface InfoTabProps {
  order: OrderDetail;
  customers: CustomerRef[];
  customersLoading: boolean;
  canEdit: boolean;
  canUploadPdf: boolean;
  saving: boolean;
  sending: boolean;
  result: SendToERPResponse | null;
  onSaveHeader: (payload: UpdateHeaderPayload) => void;
  onSendToErp: (file: File) => void;
}

export function InfoTab({
  order,
  customers,
  customersLoading,
  canEdit,
  canUploadPdf,
  saving,
  sending,
  result,
  onSaveHeader,
  onSendToErp,
}: InfoTabProps) {
  const [customerPO, setCustomerPO] = useState(order.cdlan_rif_ordcli ?? '');
  const [confirmationDate, setConfirmationDate] = useState(dateInputValue(order.cdlan_dataconferma));
  const [customerID, setCustomerID] = useState<number | null>(order.cdlan_cliente_id);
  const [file, setFile] = useState<File | null>(null);

  useEffect(() => {
    setCustomerPO(order.cdlan_rif_ordcli ?? '');
    setConfirmationDate(dateInputValue(order.cdlan_dataconferma));
    setCustomerID(order.cdlan_cliente_id);
  }, [order.cdlan_cliente_id, order.cdlan_dataconferma, order.cdlan_rif_ordcli]);

  const selectedCustomerName = useMemo(
    () => customers.find((customer) => customer.id === customerID)?.name ?? order.cdlan_cliente,
    [customerID, customers, order.cdlan_cliente],
  );
  const readyToSend = canEdit && Boolean(confirmationDate) && Boolean(selectedCustomerName) && Boolean(file);

  function saveHeader() {
    if (customerID == null) return;
    onSaveHeader({ customer_po: customerPO, confirmation_date: confirmationDate, customer_id: customerID });
  }

  return (
    <div className={styles.tabStack}>
      <section className={styles.cardSection}>
        <div className={styles.infoGroups}>
          <div className={styles.infoGroup}>
            <h3 className={styles.infoGroupTitle}>Dettagli Ordine</h3>
            <div className={styles.badgeRow}>
              <div className={styles.metaBadge}>
                <span className={styles.metaBadgeLabel}>Lingua</span>
                <span className={styles.metaBadgeValue}>{formatEmpty(order.profile_lang)}</span>
              </div>
            </div>
            <div className={styles.factGrid}>
              <Field label="Tipo documento" value={formatTipoDoc(order.cdlan_tipodoc)} />
              <Field label="Tipo proposta" value={formatTipoProposta(order.cdlan_tipo_ord)} />
              <Field label="Redatto da" value={order.written_by} />
              <Field label="Sostituisce" value={order.cdlan_sost_ord} mono />
            </div>
          </div>

          <div className={styles.infoGroup}>
            <h3 className={styles.infoGroupTitle}>Condizioni & Fatturazione</h3>
            <div className={styles.badgeRow}>
              <div className={styles.metaBadge}>
                <span className={styles.metaBadgeLabel}>Tacito Rinnovo</span>
                <span className={styles.metaBadgeValue}>{formatSiNo(order.cdlan_tacito_rin)}</span>
              </div>
            </div>
            <div className={styles.factGrid}>
              <Field label="Condizioni pagamento" value={order.cdlan_cod_termini_pag} mono />
              <Field label="Fatturazione canoni" value={formatFatturazione(order.cdlan_int_fatturazione)} />
              <Field label="Fatturazione attivazione" value={formatFatturazioneAtt(order.cdlan_int_fatturazione_att)} />
              <Field label="Durata rinnovo" value={formatDurRin(order.cdlan_dur_rin)} />
              <Field label="Note legali" value={order.cdlan_note} wide collapsible />
            </div>
          </div>

          <div className={styles.infoGroup}>
            <h3 className={styles.infoGroupTitle}>Tempistiche & Consegna</h3>
            <div className={styles.factGrid}>
              <Field label="Data decorrenza" value={formatDate(order.data_decorrenza)} />
              <Field label="Tempi rilascio" value={order.cdlan_tempi_ril} />
              <Field label="Durata servizio (Mesi)" value={order.cdlan_durata_servizio} />
            </div>
          </div>
        </div>
        {order.arx_doc_number ? (
          <a className={styles.arxivarLink} href={`https://arxivar.cdlan.it/#!/view/${encodeURIComponent(order.arx_doc_number)}`} target="_blank" rel="noreferrer">
            <Icon name="external-link" size={15} />
            Apri documento in Arxivar
          </a>
        ) : null}
      </section>

      <section className={styles.cardSection}>
        <div className={styles.sectionHeader}>
          <h2>Dati conferma</h2>
          {!canEdit ? <span className={styles.readonlyPill}>Solo lettura</span> : null}
        </div>
        <div className={styles.formGrid}>
          <label className={styles.fieldLabel}>
            <span>Rif. ordine cliente</span>
            <input className={styles.input} value={customerPO} disabled={!canEdit} onChange={(event) => setCustomerPO(event.target.value)} />
          </label>
          <label className={styles.fieldLabel}>
            <span>Data conferma</span>
            <input className={styles.input} type="date" value={confirmationDate} disabled={!canEdit} onChange={(event) => setConfirmationDate(event.target.value)} />
          </label>
          <div className={styles.customerField}>
            <span className={styles.formLabel}>Ragione sociale</span>
            <CustomerSelect customers={customers} value={customerID} currentName={order.cdlan_cliente} disabled={!canEdit || customersLoading} onChange={setCustomerID} />
            <small>ID cliente: {customerID ?? '—'}</small>
          </div>
        </div>
        <div className={styles.actionRow}>
          <Button loading={saving} disabled={!canEdit || customerID == null} onClick={saveHeader}>Salva</Button>
        </div>
      </section>

      <section className={styles.cardSection}>
        <div className={styles.sectionHeader}>
          <h2>Invio ordine</h2>
        </div>
        <div className={styles.sendBox}>
          {file ? (
            <div className={styles.fileCard}>
              <div className={styles.fileCardLeft}>
                <div className={styles.fileCardIconWrapper}>
                  <Icon name="file-text" size={24} />
                </div>
                <div className={styles.fileCardDetails}>
                  <strong className={styles.fileCardName} title={file.name}>{file.name}</strong>
                  <div className={styles.fileCardMeta}>
                    <span>{formatBytes(file.size)}</span>
                    <span className={styles.fileCardStatus}>
                      <Icon name="check" size={12} strokeWidth={3} />
                      Pronto per l'invio
                    </span>
                  </div>
                </div>
              </div>
              <button
                type="button"
                className={styles.fileCardRemove}
                title="Rimuovi file"
                onClick={() => setFile(null)}
              >
                <Icon name="trash" size={16} />
              </button>
            </div>
          ) : (
            <label className={`${styles.fileDrop} ${!canUploadPdf ? styles.fileDropDisabled : ''}`}>
              <Icon name="file-up" size={22} />
              <span>Seleziona PDF firmato</span>
              <small>Documento richiesto per inviare l'ordine in ERP.</small>
              <input type="file" accept="application/pdf,.pdf" disabled={!canUploadPdf} onChange={(event) => setFile(event.target.files?.[0] ?? null)} />
            </label>
          )}
          <Button loading={sending} disabled={!readyToSend || file == null} onClick={() => file && onSendToErp(file)}>
            Invia in ERP
          </Button>
        </div>
        <SendToErpResultPanel result={result} />
      </section>
    </div>
  );
}

function formatBytes(bytes: number, decimals = 1): string {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i] ?? ''}`;
}

function Field({
  label,
  value,
  mono,
  wide,
  collapsible,
}: {
  label: string;
  value: string | number | null | undefined;
  mono?: boolean;
  wide?: boolean;
  collapsible?: boolean;
}) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [isCollapsible, setIsCollapsible] = useState(false);
  const textRef = useRef<HTMLElement>(null);
  
  const text = value != null ? String(value).trim() : '';
  const hasText = text !== '';

  useEffect(() => {
    if (!collapsible || !hasText) return;

    function checkOverflow() {
      const el = textRef.current;
      if (!el) return;
      if (!isExpanded) {
        setIsCollapsible(el.scrollHeight > el.clientHeight);
      }
    }

    const timer = setTimeout(checkOverflow, 50);
    window.addEventListener('resize', checkOverflow);
    return () => {
      clearTimeout(timer);
      window.removeEventListener('resize', checkOverflow);
    };
  }, [text, collapsible, isExpanded, hasText]);

  return (
    <div className={`${styles.factItem} ${wide ? styles.factItemWide : ''}`}>
      <span>{label}</span>
      {collapsible && hasText ? (
        <div>
          <strong
            ref={textRef}
            className={`${mono ? styles.mono : ''} ${styles.expandableText} ${
              !isExpanded ? styles.textClamped : ''
            }`}
          >
            {text}
          </strong>
          {isCollapsible && (
            <button
              type="button"
              className={styles.expandToggle}
              onClick={() => setIsExpanded(!isExpanded)}
            >
              {isExpanded ? 'Contrai' : 'Espandi'}
            </button>
          )}
        </div>
      ) : (
        <strong className={mono ? styles.mono : undefined}>{formatEmpty(value)}</strong>
      )}
    </div>
  );
}
