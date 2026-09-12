// Dettaglio certificazione (#162, §Catalogo 4): anagrafica, titolari con
// stato di validità e attestato, corsi che la rilasciano, regole collegate
// (rimandi). Gli export XLSX del registro e delle scadenze vivono qui, il
// primo con ricerca testuale sul codice (q= è sottostringa case-insensitive
// su più campi in filterCertificationRows, non un filtro sulla sola
// certificazione: nessun filtro proprio esiste ancora sulla scheda).

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Button, Drawer, Icon, Skeleton, StatusBadge } from '@mrsmith/ui';
import { useApiClient } from '../../api/client';
import { useCertificationDetail } from '../../api/queries';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly } from '../events/eventFormat';
import { AWARD_OUTCOME_LABELS, AWARD_STATUS_LABELS } from '../../lib/labels';
import { awardStatusVariant, downloadFile } from '../people/awardHelpers';
import styles from '../requests/drawerShared.module.css';
import tableStyles from './CourseDetailDrawer.module.css';
import localStyles from './CertificationDetailDrawer.module.css';

export function CertificationDetailDrawer({ id, onClose }: { id: string; onClose: () => void }) {
  const api = useApiClient();
  const detail = useCertificationDetail(id);
  const [exporting, setExporting] = useState<'certifications' | 'expiring-certifications' | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const certification = detail.data;

  async function exportXlsx(kind: 'certifications' | 'expiring-certifications', filename: string) {
    setExportError(null);
    setExporting(kind);
    try {
      const qs = kind === 'certifications' && certification ? `?q=${encodeURIComponent(certification.code)}` : '';
      await downloadFile(api, `/training/v1/exports/${kind}.xlsx${qs}`, filename);
    } catch (e) {
      setExportError(describeApiError(e, 'Esportazione non riuscita'));
    } finally {
      setExporting(null);
    }
  }

  return (
    <Drawer open onClose={onClose} title={certification?.name ?? 'Certificazione'} subtitle={certification?.code} size="lg">
      <div className={styles.body}>
        {detail.isLoading && <Skeleton rows={6} />}
        {detail.isError && (
          <p className={styles.errorNotice}>{describeApiError(detail.error, 'Lettura della certificazione non riuscita')}</p>
        )}
        {certification && (
          <>
            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Certificazione</h3>
              <dl className={styles.grid}>
                <div className={styles.item}>
                  <dt>Emittente</dt>
                  <dd>{certification.issuerVendorName || '—'}</dd>
                </div>
                <div className={styles.item}>
                  <dt>Area di competenza</dt>
                  <dd>{certification.skillAreaName || '—'}</dd>
                </div>
                <div className={styles.item}>
                  <dt>Validità tipica</dt>
                  <dd>{certification.typicalValidityMonths !== undefined ? `${certification.typicalValidityMonths} mesi` : '—'}</dd>
                </div>
                <div className={styles.item}>
                  <dt>Livello attestato</dt>
                  <dd>{certification.attestedLevel !== undefined ? `${certification.attestedLevel} / 5` : '—'}</dd>
                </div>
                <div className={styles.item}>
                  <dt>Stato</dt>
                  <dd>{certification.active ? 'Attiva' : 'Disattiva'}</dd>
                </div>
              </dl>
              {certification.description && <p>{certification.description}</p>}
              <ErrorPanel message={exportError} onDismiss={() => setExportError(null)} />
              <div className={localStyles.exportRow}>
                <Button
                  variant="secondary"
                  size="sm"
                  leftIcon={<Icon name="download" size={14} />}
                  loading={exporting === 'certifications'}
                  onClick={() => exportXlsx('certifications', `certificazione-${certification.code}.xlsx`)}
                >
                  Esporta registro (ricerca testuale «{certification.code}»)
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  leftIcon={<Icon name="download" size={14} />}
                  loading={exporting === 'expiring-certifications'}
                  onClick={() => exportXlsx('expiring-certifications', 'certificazioni-in-scadenza.xlsx')}
                >
                  Esporta scadenze (tutte le certificazioni)
                </Button>
              </div>
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Titolari</h3>
              {certification.holders.length === 0 ? (
                <p>Nessun titolare registrato.</p>
              ) : (
                <div className={tableStyles.tableWrap}>
                  <table className={tableStyles.miniTable}>
                    <thead>
                      <tr>
                        <th>Persona</th>
                        <th>Esito</th>
                        <th>Conseguito</th>
                        <th>Scadenza</th>
                        <th>Attestato</th>
                      </tr>
                    </thead>
                    <tbody>
                      {certification.holders.map((h) => (
                        <tr key={h.awardId}>
                          <td>
                            <Link to={`/persone/${h.employeeId}`}>{h.employeeName}</Link>
                          </td>
                          <td>{AWARD_OUTCOME_LABELS[h.outcome] ?? h.outcome}</td>
                          <td>{formatDateOnly(h.awardedOn)}</td>
                          <td>
                            {h.expiresOn ? formatDateOnly(h.expiresOn) : '—'}{' '}
                            <StatusBadge
                              value={h.currentStatus}
                              label={AWARD_STATUS_LABELS[h.currentStatus] ?? h.currentStatus}
                              variant={awardStatusVariant(h.currentStatus)}
                            />
                          </td>
                          <td>
                            {h.documentFilename ? (
                              <StatusBadge
                                value={h.documentValidated ? 'validated' : 'pending'}
                                label={h.documentValidated ? 'Validato' : 'Da validare'}
                                variant={h.documentValidated ? 'success' : 'warning'}
                              />
                            ) : (
                              '—'
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Corsi che la rilasciano</h3>
              {certification.courses.length === 0 ? (
                <p>Nessun corso collegato.</p>
              ) : (
                <ul className={localStyles.refList}>
                  {certification.courses.map((c) => (
                    <li key={c.id}>
                      <Link to={`/catalogo/corsi?id=${c.id}`}>{c.title}</Link>
                      <StatusBadge
                        value={c.active ? 'active' : 'inactive'}
                        label={c.active ? 'Attivo' : 'Archiviato'}
                        variant={c.active ? 'success' : 'neutral'}
                      />
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className={styles.card}>
              <h3 className={styles.cardTitle}>Regole collegate</h3>
              {certification.rules.length === 0 ? (
                <p>Nessuna regola collegata.</p>
              ) : (
                <ul className={localStyles.refList}>
                  {certification.rules.map((r) => (
                    <li key={r.id}>
                      <Link to={`/regole?id=${r.id}`}>{r.name}</Link>
                      <StatusBadge
                        value={r.isActive ? 'active' : 'inactive'}
                        label={r.isActive ? 'Attiva' : 'Disattiva'}
                        variant={r.isActive ? 'success' : 'neutral'}
                      />
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </>
        )}
      </div>
    </Drawer>
  );
}
