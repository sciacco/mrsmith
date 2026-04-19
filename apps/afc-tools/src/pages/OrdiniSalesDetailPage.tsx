import { useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Icon, Skeleton, StatusBadge, type StatusBadgeVariant } from '@mrsmith/ui';
import { useNavigate, useParams } from 'react-router-dom';
import { useOrderHeader, useOrderRows } from '../api/queries';
import { formatDate, formatMoney, isEmpty } from '../utils/format';
import {
  durRinLabel,
  paymentTermsLabel,
  tacitoRinLabel,
  tipoOrdLabel,
  tipodocLabel,
} from './labels';
import type { OrderHeader, OrderRow } from '../types';
import shared from './shared.module.css';
import styles from './OrdiniSalesDetailPage.module.css';

type FieldValue = string | number | null | undefined;

function Field({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: FieldValue;
  mono?: boolean;
}) {
  const empty = isEmpty(value) || (typeof value === 'string' && value.trim() === '');
  return (
    <div className={styles.field}>
      <span className={styles.label}>{label}</span>
      <span
        className={`${styles.value} ${mono ? styles.valueMono : ''} ${
          empty ? styles.valueMuted : styles.valueStrong
        }`}
      >
        {empty ? '—' : String(value)}
      </span>
    </div>
  );
}

function tipoOrdVariant(code: string | null | undefined): StatusBadgeVariant {
  switch (code) {
    case 'N':
      return 'success';
    case 'A':
      return 'warning';
    case 'R':
      return 'accent';
    default:
      return 'neutral';
  }
}

function renderCommerciale(o: OrderHeader) {
  return (
    <div className={styles.grid}>
      <Field label="Data conferma" value={formatDate(o.cdlan_dataconferma)} />
      <Field label="System ODV" value={o.cdlan_systemodv} mono />
      <Field label="Commerciale" value={o.cdlan_commerciale} />
      <Field label="Durata rinnovo" value={durRinLabel(o.cdlan_dur_rin)} />
      <Field label="Tacito rinnovo" value={tacitoRinLabel(o.cdlan_tacito_rin)} />
      <Field label="Durata servizio" value={o.cdlan_durata_servizio} />
      <Field label="Tempi rilascio" value={o.cdlan_tempi_ril} />
      <Field label="Rif. ordine cliente" value={o.cdlan_rif_ordcli} />
      <Field label="Fatturazione canoni anticipata" value={o.cdlan_int_fatturazione_desc} />
      <Field label="Fatturazione attivazione" value={o.cdlan_int_fatturazione_att_desc} />
      <Field label="Condizioni di pagamento" value={paymentTermsLabel(o.cdlan_cod_termini_pag)} />
      <Field label="Data decorrenza" value={formatDate(o.data_decorrenza)} />
    </div>
  );
}

function renderReferenti(o: OrderHeader) {
  return (
    <div className={styles.grid}>
      <Field label="Referente tecnico" value={o.cdlan_rif_tech_nom} />
      <Field label="Telefono tecnico" value={o.cdlan_rif_tech_tel} />
      <Field label="Email tecnico" value={o.cdlan_rif_tech_email} />
      <Field label="Altro referente tecnico" value={o.cdlan_rif_altro_tech_nom} />
      <Field label="Telefono altro tecnico" value={o.cdlan_rif_altro_tech_tel} />
      <Field label="Email altro tecnico" value={o.cdlan_rif_altro_tech_email} />
      <Field label="Referente amministrativo" value={o.cdlan_rif_adm_nom} />
      <Field label="Telefono amministrativo" value={o.cdlan_rif_adm_tech_tel} />
      <Field label="Email amministrativa" value={o.cdlan_rif_adm_tech_email} />
    </div>
  );
}

function renderFatturazione(o: OrderHeader) {
  return (
    <div className={styles.grid}>
      <Field label="Partita IVA" value={o.profile_iva} mono />
      <Field label="Codice fiscale" value={o.profile_cf} mono />
      <Field label="Indirizzo" value={o.profile_address} />
      <Field label="Città" value={o.profile_city} />
      <Field label="CAP" value={o.profile_cap} />
      <Field label="Provincia" value={o.profile_pv} />
      <Field label="SDI" value={o.profile_sdi} mono />
    </div>
  );
}

function renderTecnico(o: OrderHeader) {
  return (
    <div className={styles.grid}>
      <Field label="Arxivar" value={o.is_arxivar} />
      <Field label="Numero documento Arxivar" value={o.arx_doc_number} mono />
      <Field label="Creato da" value={o.written_by} />
      <Field label="Evaso" value={o.cdlan_evaso} />
      <Field label="Chiuso" value={o.cdlan_chiuso} />
      <Field label="Lingua profilo" value={o.profile_lang} />
      <Field label="Origin cod. termini pag." value={o.origin_cod_termini_pag} />
      <Field label="Tacito rinn. in PDF" value={o.cdlan_tacito_rin_in_pdf} />
      <Field label="Is colo" value={o.is_colo} />
      <Field label="Sostituisce" value={o.cdlan_sost_ord} mono />
      <Field label="Service type" value={o.service_type} />
      <Field label="Cliente ID" value={o.cdlan_cliente_id} />
    </div>
  );
}

function computeTotals(rows: OrderRow[] | undefined) {
  if (!rows) return null;
  return rows.reduce(
    (acc, r) => ({
      canone: acc.canone + (r.canone ?? 0),
      attivazione: acc.attivazione + (r.attivazione ?? 0),
    }),
    { canone: 0, attivazione: 0 },
  );
}

export default function OrdiniSalesDetailPage() {
  const { id: idParam } = useParams<{ id: string }>();
  const id = Number(idParam);
  const navigate = useNavigate();

  const headerQ = useOrderHeader(id);
  const rowsQ = useOrderRows(id);

  const [showTech, setShowTech] = useState(false);
  const [showTechCols, setShowTechCols] = useState(false);
  const [notesExpanded, setNotesExpanded] = useState(false);
  const [notesOverflow, setNotesOverflow] = useState(false);
  const notesRef = useRef<HTMLDivElement>(null);

  const note = headerQ.data?.cdlan_note ?? null;

  useLayoutEffect(() => {
    if (!note || notesExpanded) return;
    const el = notesRef.current;
    if (!el) return;
    setNotesOverflow(el.scrollHeight > el.clientHeight + 1);
  }, [note, notesExpanded]);

  const totals = useMemo(() => computeTotals(rowsQ.data), [rowsQ.data]);

  const valuta = headerQ.data?.cdlan_valuta ?? null;
  const money = (v: number | null | undefined) => formatMoney(v, valuta);

  const codiceOrdine = headerQ.data
    ? `${headerQ.data.cdlan_ndoc ?? ''}/${headerQ.data.cdlan_anno ?? ''}`
    : '';

  return (
    <div className={shared.page}>
      <nav className={styles.breadcrumb} aria-label="Breadcrumb">
        <button
          type="button"
          className={styles.breadcrumbLink}
          onClick={() => navigate('/ordini-sales')}
        >
          Ordini Sales
        </button>
        <span className={styles.breadcrumbSep} aria-hidden="true">/</span>
        <h1 className={styles.breadcrumbCode}>{codiceOrdine || '…'}</h1>
        {headerQ.data?.cdlan_stato && <StatusBadge value={headerQ.data.cdlan_stato} />}
        {headerQ.data?.cdlan_tipo_ord && (
          <StatusBadge
            value={tipoOrdLabel(headerQ.data.cdlan_tipo_ord)}
            variant={tipoOrdVariant(headerQ.data.cdlan_tipo_ord)}
          />
        )}
        {!!headerQ.data?.from_cp && (
          <StatusBadge
            value="Customer Portal"
            variant="neutral"
            tooltip="Ordine creato dal Customer Portal"
          />
        )}
      </nav>

      {headerQ.isLoading && <Skeleton rows={4} />}
      {headerQ.isError && <div className={shared.error}>Errore nel caricamento del dettaglio.</div>}

      {headerQ.data && (
        <>
          <div className={styles.summaryStrip}>
            <div className={styles.summaryItem}>
              <span className={styles.summaryLabel}>Cliente</span>
              <span className={styles.summaryValue}>{headerQ.data.cdlan_cliente ?? '—'}</span>
            </div>
            <div className={styles.summaryItem}>
              <span className={styles.summaryLabel}>Data documento</span>
              <span className={styles.summaryValue}>{formatDate(headerQ.data.cdlan_datadoc) || '—'}</span>
            </div>
            <div className={styles.summaryItem}>
              <span className={styles.summaryLabel}>Tipo documento</span>
              <span className={styles.summaryValue}>{tipodocLabel(headerQ.data.cdlan_tipodoc) || '—'}</span>
            </div>
            <div className={styles.summaryKpi}>
              <div className={styles.summaryItem}>
                <span className={styles.summaryLabel}>Canone totale</span>
                <span className={styles.kpiValue}>
                  {rowsQ.isLoading ? <Skeleton rows={1} /> : totals ? money(totals.canone) : '—'}
                </span>
              </div>
              <div className={styles.summaryItem}>
                <span className={styles.summaryLabel}>Attivazione totale</span>
                <span className={styles.kpiValue}>
                  {rowsQ.isLoading ? <Skeleton rows={1} /> : totals ? money(totals.attivazione) : '—'}
                </span>
              </div>
            </div>
          </div>

          <section className={styles.section}>{renderCommerciale(headerQ.data)}</section>
          <section className={styles.section}>{renderFatturazione(headerQ.data)}</section>

          {!isEmpty(note) && (
            <section className={styles.section}>
              <div className={styles.notesCard}>
                <span className={styles.label}>Note legali</span>
                <div
                  ref={notesRef}
                  className={`${styles.notesText} ${!notesExpanded ? styles.notesClamp : ''}`}
                  dangerouslySetInnerHTML={{ __html: note ?? '' }}
                />
                {notesOverflow && (
                  <button
                    type="button"
                    className={`${shared.btnLink} ${styles.moreToggle}`}
                    onClick={() => setNotesExpanded((v) => !v)}
                    aria-expanded={notesExpanded}
                  >
                    {notesExpanded ? 'Mostra meno' : 'Mostra tutto'}
                    <Icon name={notesExpanded ? 'chevron-up' : 'chevron-down'} size={14} />
                  </button>
                )}
              </div>
            </section>
          )}

          <div className={styles.technicalToggle}>
            <button
              type="button"
              className={`${shared.btnLink} ${styles.moreToggle}`}
              onClick={() => setShowTech((v) => !v)}
              aria-expanded={showTech}
            >
              {showTech ? 'Nascondi altre informazioni' : 'Mostra altre informazioni'}
              <Icon name={showTech ? 'chevron-up' : 'chevron-down'} size={14} />
            </button>
          </div>

          {showTech && (
            <>
              <section className={styles.section}>{renderTecnico(headerQ.data)}</section>
              <section className={styles.section}>{renderReferenti(headerQ.data)}</section>
            </>
          )}
        </>
      )}

      <div className={styles.rowsHeader}>
        <h2 className={styles.rowsTitle}>Righe ordine</h2>
        {rowsQ.data && rowsQ.data.length > 0 && (
          <button
            type="button"
            className={shared.btnLink}
            onClick={() => setShowTechCols((v) => !v)}
            aria-expanded={showTechCols}
          >
            {showTechCols ? 'Nascondi colonne tecniche' : 'Mostra colonne tecniche'}
          </button>
        )}
      </div>

      {rowsQ.isLoading && <Skeleton rows={6} />}
      {rowsQ.isError && <div className={shared.error}>Errore nel caricamento delle righe.</div>}

      {rowsQ.data && rowsQ.data.length === 0 && (
        <div className={shared.empty}>Nessuna riga su questo ordine.</div>
      )}

      {rowsQ.data && rowsQ.data.length > 0 && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Codice bundle</th>
                <th>Codice articolo</th>
                <th>Descrizione</th>
                <th className={shared.numCol}>Canone</th>
                <th className={shared.numCol}>Attivazione</th>
                <th className={shared.numCol}>Quantità</th>
                <th>Data attivazione</th>
                <th>Data annullamento</th>
                {showTechCols && (
                  <>
                    <th className={shared.numCol}>ID riga</th>
                    <th>System ODV</th>
                    <th className={shared.numCol}>Prezzo cessazione</th>
                    <th>Cod. raggrupp.</th>
                    <th>Seriale</th>
                    <th>Conferma data attivazione</th>
                  </>
                )}
              </tr>
            </thead>
            <tbody>
              {rowsQ.data.map((r, i) => {
                const isBundle = !isEmpty(r.codice_articolo_bundle);
                return (
                  <tr key={r.id_riga} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                    <td className={shared.mono}>{r.codice_articolo_bundle ?? ''}</td>
                    <td className={`${shared.mono} ${isBundle ? styles.bundleCell : ''}`}>
                      {r.codice_articolo ?? ''}
                    </td>
                    <td>
                      {r.descrizione_articolo ? (
                        <div
                          className={styles.rowDesc}
                          dangerouslySetInnerHTML={{ __html: r.descrizione_articolo }}
                        />
                      ) : (
                        ''
                      )}
                    </td>
                    <td className={shared.numCol}>{money(r.canone)}</td>
                    <td className={shared.numCol}>{money(r.attivazione)}</td>
                    <td className={shared.numCol}>{r.quantita ?? ''}</td>
                    <td>{formatDate(r.data_attivazione)}</td>
                    <td>{formatDate(r.data_annullamento)}</td>
                    {showTechCols && (
                      <>
                        <td className={shared.numCol}>{r.id_riga}</td>
                        <td className={shared.mono}>{r.system_odv_riga ?? ''}</td>
                        <td className={shared.numCol}>{money(r.prezzo_cessazione)}</td>
                        <td>{r.codice_raggruppamento_fatturazione ?? ''}</td>
                        <td className={shared.mono}>{r.numero_seriale ?? ''}</td>
                        <td>{formatDate(r.confirm_data_attivazione)}</td>
                      </>
                    )}
                  </tr>
                );
              })}
            </tbody>
            {totals && (
              <tfoot className={styles.tfoot}>
                <tr>
                  <td colSpan={3} className={styles.tfootLabel}>Totali</td>
                  <td className={shared.numCol}>{money(totals.canone)}</td>
                  <td className={shared.numCol}>{money(totals.attivazione)}</td>
                  <td colSpan={showTechCols ? 9 : 3} />
                </tr>
              </tfoot>
            )}
          </table>
        </div>
      )}
    </div>
  );
}
