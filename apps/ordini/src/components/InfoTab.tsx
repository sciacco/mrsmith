import { useEffect, useMemo, useState } from 'react';
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
        <div className={styles.sectionHeader}>
          <h2>Informazioni ordine</h2>
        </div>
        <div className={styles.factGrid}>
          <Field label="Tipo documento" value={formatTipoDoc(order.cdlan_tipodoc)} />
          <Field label="Tipo proposta" value={formatTipoProposta(order.cdlan_tipo_ord)} />
          <Field label="ODV" value={order.cdlan_systemodv} mono />
          <Field label="Commerciale" value={order.cdlan_commerciale} />
          <Field label="Redatto da" value={order.written_by} />
          <Field label="Condizioni pagamento" value={order.cdlan_cod_termini_pag} mono />
          <Field label="Durata rinnovo" value={formatDurRin(order.cdlan_dur_rin)} />
          <Field label="Tacito rinnovo" value={formatSiNo(order.cdlan_tacito_rin)} />
          <Field label="Fatturazione canoni" value={formatFatturazione(order.cdlan_int_fatturazione)} />
          <Field label="Fatturazione attivazione" value={formatFatturazioneAtt(order.cdlan_int_fatturazione_att)} />
          <Field label="Data decorrenza" value={formatDate(order.data_decorrenza)} />
          <Field label="Tempi rilascio" value={order.cdlan_tempi_ril} />
          <Field label="Durata servizio" value={order.cdlan_durata_servizio} />
          <Field label="Sostituisce" value={order.cdlan_sost_ord} mono />
          <Field label="Lingua" value={order.profile_lang} />
          <Field label="Note legali" value={order.cdlan_note} wide />
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
          <label className={`${styles.fileDrop} ${!canUploadPdf ? styles.fileDropDisabled : ''}`}>
            <Icon name="file-up" size={22} />
            <span>{file ? file.name : 'Seleziona PDF firmato'}</span>
            <small>Documento richiesto per inviare l'ordine in ERP.</small>
            <input type="file" accept="application/pdf,.pdf" disabled={!canUploadPdf} onChange={(event) => setFile(event.target.files?.[0] ?? null)} />
          </label>
          <Button loading={sending} disabled={!readyToSend || file == null} onClick={() => file && onSendToErp(file)}>
            Invia in ERP
          </Button>
        </div>
        <SendToErpResultPanel result={result} />
      </section>
    </div>
  );
}

function Field({ label, value, mono, wide }: { label: string; value: string | number | null | undefined; mono?: boolean; wide?: boolean }) {
  return (
    <div className={`${styles.factItem} ${wide ? styles.factItemWide : ''}`}>
      <span>{label}</span>
      <strong className={mono ? styles.mono : undefined}>{formatEmpty(value)}</strong>
    </div>
  );
}
