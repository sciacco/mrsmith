import { Skeleton } from '@mrsmith/ui';
import { useNavigate, useParams } from 'react-router-dom';
import { useOrderHeader, useOrderRows } from '../api/queries';
import { formatDate, formatMoneyEUR, isEmpty } from '../utils/format';
import {
  durRinLabel,
  paymentTermsLabel,
  tacitoRinLabel,
  tipoOrdLabel,
  tipodocLabel,
} from './labels';
import type { OrderHeader } from '../types';
import shared from './shared.module.css';
import styles from './OrdiniSalesDetailPage.module.css';

function Field({ label, value, muted = false }: { label: string; value: string; muted?: boolean }) {
  return (
    <div className={styles.field}>
      <span className={styles.label}>{label}</span>
      <span className={`${styles.value} ${muted ? styles.valueMuted : styles.valueStrong}`}>
        {value}
      </span>
    </div>
  );
}

function displayOrEmpty(v: string | null | undefined, placeholder: string): { value: string; muted: boolean } {
  if (isEmpty(v)) return { value: placeholder, muted: true };
  return { value: String(v), muted: false };
}

function renderHeader(order: OrderHeader) {
  const note = displayOrEmpty(order.cdlan_note, 'Nessuna nota legale');
  const decorrenza = displayOrEmpty(order.data_decorrenza, 'Nessun valore');

  return (
    <div className={styles.grid}>
      <Field label="Codice ordine" value={`${order.cdlan_ndoc ?? ''}/${order.cdlan_anno ?? ''}`} />
      <Field label="Cliente" value={order.cdlan_cliente ?? ''} />
      <Field label="Tipo di documento" value={tipodocLabel(order.cdlan_tipodoc)} />
      <Field label="Tipo di ordine" value={tipoOrdLabel(order.cdlan_tipo_ord)} />
      <Field label="Stato" value={order.cdlan_stato ?? ''} />
      <Field label="Data documento" value={formatDate(order.cdlan_datadoc)} />
      <Field label="Data conferma" value={formatDate(order.cdlan_dataconferma)} />
      <Field label="System ODV" value={order.cdlan_systemodv ?? ''} />
      <Field label="Commerciale" value={order.cdlan_commerciale ?? ''} />
      <Field label="Valuta" value={order.cdlan_valuta ?? ''} />
      <Field label="Durata rinnovo" value={durRinLabel(order.cdlan_dur_rin)} />
      <Field label="Tacito rinnovo" value={tacitoRinLabel(order.cdlan_tacito_rin)} />
      <Field label="Durata servizio" value={order.cdlan_durata_servizio ?? ''} />
      <Field label="Tempi rilascio" value={order.cdlan_tempi_ril ?? ''} />
      <Field
        label="Modalità di fatturazione canoni anticipata"
        value={order.cdlan_int_fatturazione_desc ?? ''}
      />
      <Field
        label="Modalità di fatturazione attivazione"
        value={order.cdlan_int_fatturazione_att_desc ?? ''}
      />
      <Field label="Condizioni di pagamento" value={paymentTermsLabel(order.cdlan_cod_termini_pag)} />
      <Field label="Riferimento ordine cliente" value={order.cdlan_rif_ordcli ?? ''} />
      <Field label="Referente tecnico" value={order.cdlan_rif_tech_nom ?? ''} />
      <Field label="Telefono tecnico" value={order.cdlan_rif_tech_tel ?? ''} />
      <Field label="Email tecnico" value={order.cdlan_rif_tech_email ?? ''} />
      <Field label="Altro referente tecnico" value={order.cdlan_rif_altro_tech_nom ?? ''} />
      <Field label="Telefono altro tecnico" value={order.cdlan_rif_altro_tech_tel ?? ''} />
      <Field label="Email altro tecnico" value={order.cdlan_rif_altro_tech_email ?? ''} />
      <Field label="Referente amministrativo" value={order.cdlan_rif_adm_nom ?? ''} />
      <Field label="Telefono amministrativo" value={order.cdlan_rif_adm_tech_tel ?? ''} />
      <Field label="Email amministrativa" value={order.cdlan_rif_adm_tech_email ?? ''} />
      <Field label="Partita IVA" value={order.profile_iva ?? ''} />
      <Field label="Codice fiscale" value={order.profile_cf ?? ''} />
      <Field label="Indirizzo" value={order.profile_address ?? ''} />
      <Field label="Città" value={order.profile_city ?? ''} />
      <Field label="CAP" value={order.profile_cap ?? ''} />
      <Field label="Provincia" value={order.profile_pv ?? ''} />
      <Field label="SDI" value={order.profile_sdi ?? ''} />
      <Field label="Data decorrenza" value={decorrenza.value} muted={decorrenza.muted} />
      <Field label="Note legali" value={note.value} muted={note.muted} />
      <Field label="Numero documento Arxivar" value={order.arx_doc_number ?? ''} />
    </div>
  );
}

export default function OrdiniSalesDetailPage() {
  const { id: idParam } = useParams<{ id: string }>();
  const id = Number(idParam);
  const navigate = useNavigate();

  const headerQ = useOrderHeader(id);
  const rowsQ = useOrderRows(id);

  return (
    <div className={shared.page}>
      <button className={shared.backLink} onClick={() => navigate('/ordini-sales')}>
        ← Torna alla lista ordini
      </button>
      <h1 className={shared.title}>Dettaglio ordine</h1>

      {headerQ.isLoading && <Skeleton rows={6} />}
      {headerQ.isError && <div className={shared.error}>Errore nel caricamento del dettaglio.</div>}
      {headerQ.data && renderHeader(headerQ.data)}

      <h2 className={styles.sectionTitle}>Righe ordine</h2>
      {rowsQ.isLoading && <Skeleton rows={6} />}
      {rowsQ.isError && <div className={shared.error}>Errore nel caricamento delle righe.</div>}
      {rowsQ.data && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th className={shared.numCol}>ID riga</th>
                <th>System ODV riga</th>
                <th>Codice articolo bundle</th>
                <th>Codice articolo</th>
                <th>Descrizione</th>
                <th className={shared.numCol}>Canone</th>
                <th className={shared.numCol}>Attivazione</th>
                <th className={shared.numCol}>Quantità</th>
                <th className={shared.numCol}>Prezzo cessazione</th>
                <th>Codice raggr. fatturazione</th>
                <th>Data attivazione</th>
                <th>Numero seriale</th>
                <th>Conferma data attivazione</th>
                <th>Data annullamento</th>
              </tr>
            </thead>
            <tbody>
              {rowsQ.data.map((r, i) => (
                <tr key={r.id_riga} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                  <td className={shared.numCol}>{r.id_riga}</td>
                  <td className={shared.mono}>{r.system_odv_riga ?? ''}</td>
                  <td className={shared.mono}>{r.codice_articolo_bundle ?? ''}</td>
                  <td className={shared.mono}>{r.codice_articolo ?? ''}</td>
                  <td>{r.descrizione_articolo ?? ''}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(r.canone)}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(r.attivazione)}</td>
                  <td className={shared.numCol}>{r.quantita ?? ''}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(r.prezzo_cessazione)}</td>
                  <td>{r.codice_raggruppamento_fatturazione ?? ''}</td>
                  <td>{formatDate(r.data_attivazione)}</td>
                  <td className={shared.mono}>{r.numero_seriale ?? ''}</td>
                  <td>{formatDate(r.confirm_data_attivazione)}</td>
                  <td>{formatDate(r.data_annullamento)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {rowsQ.data.length === 0 && <div className={shared.empty}>Nessuna riga.</div>}
        </div>
      )}
    </div>
  );
}
