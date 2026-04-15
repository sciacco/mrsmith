import { Fragment, useMemo, useState } from 'react';
import { Skeleton, Button, SingleSelect, Drawer, Icon } from '@mrsmith/ui';
import { useUpcomingRenewals, useRenewalRows } from '../api/queries';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';
import styles from './RinnoviInArrivoPage.module.css';

const monthsOptions = Array.from({ length: 12 }, (_, i) => ({
  value: i + 1,
  label: `${i + 1} ${i === 0 ? 'mese' : 'mesi'}`,
}));

export default function RinnoviInArrivoPage() {
  const [draftMonths, setDraftMonths] = useState(4);
  const [draftMinMrc, setDraftMinMrc] = useState(11);
  const [committedMonths, setCommittedMonths] = useState(4);
  const [committedMinMrc, setCommittedMinMrc] = useState(11);
  const [selectedCustomer, setSelectedCustomer] = useState<string | null>(null);
  const [expandedDetailRow, setExpandedDetailRow] = useState<number | null>(null);

  const renewalsQ = useUpcomingRenewals(committedMonths, committedMinMrc);
  const rowsQ = useRenewalRows(selectedCustomer, committedMonths, committedMinMrc);

  const isDirty = draftMonths !== committedMonths || draftMinMrc !== committedMinMrc;

  const selectedSummary = useMemo(
    () => renewalsQ.data?.find((r) => r.numero_azienda === selectedCustomer),
    [renewalsQ.data, selectedCustomer],
  );

  function handleExecute() {
    setSelectedCustomer(null);
    setExpandedDetailRow(null);
    setCommittedMonths(draftMonths);
    setCommittedMinMrc(draftMinMrc);
  }

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Rinnovi in arrivo</h1>

      <div className={shared.toolbar}>
        <div className={shared.field}>
          <label>MRC minimo</label>
          <input
            type="number"
            className={styles.numberInput}
            value={draftMinMrc}
            onChange={(e) => setDraftMinMrc(Number(e.target.value))}
          />
        </div>

        <div className={shared.field}>
          <label>Rinnovi entro N mesi</label>
          <SingleSelect
            options={monthsOptions}
            selected={draftMonths}
            onChange={(v) => setDraftMonths(v ?? 4)}
            placeholder="Mesi..."
          />
        </div>

        <Button
          variant="primary"
          loading={renewalsQ.isFetching}
          onClick={handleExecute}
          className={isDirty ? styles.btnDirty : undefined}
        >
          Esegui
        </Button>
      </div>

      {renewalsQ.isLoading && <Skeleton rows={8} />}

      {renewalsQ.error && <p>Errore nel caricamento dei dati.</p>}

      {renewalsQ.data && (
        <div className={renewalsQ.isFetching ? styles.refetching : undefined}>
          <div className={shared.info}>{renewalsQ.data.length} clienti</div>
          <div className={shared.tableWrap}>
            <table className={`${shared.table} ${styles.table}`}>
              <thead>
                <tr>
                  <th></th>
                  <th>Cliente</th>
                  <th>Rinnovi dal</th>
                  <th>Rinnovi al</th>
                  <th>Ordini/Servizi</th>
                  <th>Senza tacito rinnovo</th>
                  <th className={shared.numCol}>Canoni</th>
                </tr>
              </thead>
              <tbody>
                {renewalsQ.data.map((row, i) => (
                  <tr
                    key={row.numero_azienda}
                    className={selectedCustomer === row.numero_azienda ? styles.selectedRow : undefined}
                    onClick={() => setSelectedCustomer(
                      selectedCustomer === row.numero_azienda ? null : row.numero_azienda,
                    )}
                    style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
                  >
                    <td><div className={styles.accentBar} /></td>
                    <td>{row.ragione_sociale}</td>
                    <td>{row.rinnovi_dal?.slice(0, 10) ?? ''}</td>
                    <td>{row.rinnovi_al?.slice(0, 10) ?? ''}</td>
                    <td>{row.ordini_servizi}</td>
                    <td>{row.senza_tacito_rinnovo ? 'Si' : 'No'}</td>
                    <td className={shared.numCol}>{row.canoni != null ? formatMoneyEUR(row.canoni) : ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <Drawer
        open={!!selectedCustomer}
        onClose={() => {
          setSelectedCustomer(null);
          setExpandedDetailRow(null);
        }}
        size="xl"
        title={selectedSummary?.ragione_sociale ?? 'Dettaglio rinnovi'}
        subtitle={
          selectedSummary && (
            <div className={styles.drawerAggregates}>
              <span>
                {selectedSummary.rinnovi_dal?.slice(0, 10) ?? ''}
                {' → '}
                {selectedSummary.rinnovi_al?.slice(0, 10) ?? ''}
              </span>
              <span className={styles.sep}>·</span>
              <span>
                {selectedSummary.numero_ordini} Ordini / {selectedSummary.servizi_attivi} Servizi
              </span>
              <span className={styles.sep}>·</span>
              <span>
                Canoni{' '}
                <strong>
                  {selectedSummary.canoni != null ? formatMoneyEUR(selectedSummary.canoni) : '—'}
                </strong>
              </span>
              {selectedSummary.senza_tacito_rinnovo && (
                <span className={styles.chip}>Senza tacito rinnovo</span>
              )}
            </div>
          )
        }
      >
        {rowsQ.isLoading && <Skeleton rows={4} />}

        {rowsQ.error && <p>Errore nel caricamento delle righe.</p>}

        {rowsQ.data && (
          <div className={shared.tableWrap}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th>N. Ordine</th>
                  <th>Stato ordine</th>
                  <th className={shared.numCol}>Quantita</th>
                  <th className={shared.numCol}>NRC</th>
                  <th className={shared.numCol}>MRC</th>
                  <th>Stato riga</th>
                  <th>Serial</th>
                  <th>Note</th>
                  <th>Data attivazione</th>
                  <th>Durata</th>
                  <th>Prossimo rinnovo</th>
                  <th>Sost. ord.</th>
                  <th>Sostituito da</th>
                  <th>Tacito rinnovo</th>
                </tr>
              </thead>
              <tbody>
                {rowsQ.data.map((row, i) => {
                  const isExpanded = expandedDetailRow === i;
                  return (
                    <Fragment key={i}>
                      <tr
                        className={`${styles.detailRow} ${isExpanded ? styles.detailRowExpanded : ''}`}
                        onClick={() => setExpandedDetailRow(isExpanded ? null : i)}
                        style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
                      >
                        <td className={shared.mono}>{row.nome_testata_ordine}</td>
                        <td>{row.stato_ordine ?? ''}</td>
                        <td className={shared.numCol}>{row.quantita ?? ''}</td>
                        <td className={shared.numCol}>{row.nrc != null ? formatMoneyEUR(row.nrc) : ''}</td>
                        <td className={shared.numCol}>{row.mrc != null ? formatMoneyEUR(row.mrc) : ''}</td>
                        <td>{row.stato_riga ?? ''}</td>
                        <td className={shared.mono}>{row.serialnumber ?? ''}</td>
                        <td className={isExpanded ? styles.noteCellExpanded : styles.noteCell}>
                          {row.note_legali
                            ? isExpanded
                              ? row.note_legali
                              : (
                                <span
                                  className={styles.noteTrigger}
                                  title={row.note_legali}
                                >
                                  <Icon name="file-text" size={16} className={styles.noteIcon} />
                                </span>
                              )
                            : ''}
                        </td>
                        <td>{row.data_attivazione?.slice(0, 10) ?? ''}</td>
                        <td>{row.durata ?? ''}</td>
                        <td>{row.prossimo_rinnovo?.slice(0, 10) ?? ''}</td>
                        <td>{row.sost_ord ?? ''}</td>
                        <td>{row.sostituito_da ?? ''}</td>
                        <td>{row.tacito_rinnovo ?? ''}</td>
                      </tr>
                      <tr
                        className={`${styles.descRow} ${isExpanded ? styles.detailRowExpanded : ''}`}
                        onClick={() => setExpandedDetailRow(isExpanded ? null : i)}
                      >
                        <td colSpan={14} className={styles.descCell}>
                          {row.descrizione_long ?? '—'}
                        </td>
                      </tr>
                    </Fragment>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Drawer>
    </div>
  );
}
