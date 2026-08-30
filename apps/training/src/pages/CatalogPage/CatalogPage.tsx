// Catalogo (#158, §Catalogo 4-5): corsi (con filtro "Da curare" = corsi
// inattivi importati dal sync formativo Factorial) e anagrafiche, in due
// viste sullo stesso pattern query-param di FactorialPage.

import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { formatCurrency } from '@mrsmith/format';
import { Button, Icon, Skeleton, StatusBadge, ToggleSwitch } from '@mrsmith/ui';
import { useTrainingCourses } from '../../api/queries';
import type { CourseListRow } from '../../api/types';
import { AnagraficheSection } from '../../components/catalog/AnagraficheSection';
import { CertificationDetailDrawer } from '../../components/catalog/CertificationDetailDrawer';
import { CertificationsSection } from '../../components/catalog/CertificationsSection';
import { CourseDetailDrawer } from '../../components/catalog/CourseDetailDrawer';
import { CourseEditorModal } from '../../components/catalog/CourseEditorModal';
import { PathDetailDrawer } from '../../components/catalog/PathDetailDrawer';
import { PathsCatalogSection } from '../../components/catalog/PathsCatalogSection';
import { DELIVERY_MODE_LABELS, PROVIDER_KIND_LABELS } from '../../lib/labels';
import listStyles from '../RequestsPage/listPage.module.css';
import viewStyles from '../FactorialPage.module.css';

// Percorsi (#162, §Catalogo 5) e Certificazioni (#162, §Catalogo 4) sono
// nuove sottosezioni allo stesso livello di Corsi/Anagrafiche: la seconda è
// il punto d'ingresso operativo (titolari, corsi, regole, export) —
// Anagrafiche resta la gestione anagrafica di base, non duplicata qui.
type View = 'corsi' | 'anagrafiche' | 'certificazioni' | 'percorsi';

export function CatalogPage() {
  const [params, setParams] = useSearchParams();
  const rawView = params.get('vista');
  const view: View =
    rawView === 'anagrafiche' || rawView === 'certificazioni' || rawView === 'percorsi' ? rawView : 'corsi';
  const selectedId = params.get('id');
  const selectedCertId = params.get('certId');
  const selectedPathId = params.get('pathId');

  const [showCreate, setShowCreate] = useState(false);
  const [onlyToCurate, setOnlyToCurate] = useState(false);

  const courses = useTrainingCourses();
  const filtered = useMemo(() => {
    const rows = courses.data ?? [];
    return onlyToCurate ? rows.filter((c) => !c.active && c.factorialTrainingId) : rows;
  }, [courses.data, onlyToCurate]);

  function setView(next: View) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('vista', next);
    setParams(nextParams, { replace: true });
  }

  function openDetail(id: string) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('id', id);
    setParams(nextParams, { replace: true });
  }

  function closeDetail() {
    const nextParams = new URLSearchParams(params);
    nextParams.delete('id');
    setParams(nextParams, { replace: true });
  }

  function openCertification(id: string) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('certId', id);
    setParams(nextParams, { replace: true });
  }

  function closeCertification() {
    const nextParams = new URLSearchParams(params);
    nextParams.delete('certId');
    setParams(nextParams, { replace: true });
  }

  function openPath(id: string) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('pathId', id);
    setParams(nextParams, { replace: true });
  }

  function closePath() {
    const nextParams = new URLSearchParams(params);
    nextParams.delete('pathId');
    setParams(nextParams, { replace: true });
  }

  return (
    <main className={listStyles.page}>
      <header className={listStyles.header}>
        <div>
          <h1 className={listStyles.title}>Catalogo</h1>
          <p className={listStyles.subtitle}>Corsi, percorsi, fornitori, aree di competenza e certificazioni.</p>
        </div>
        <nav className={viewStyles.viewSwitch} aria-label="Vista catalogo">
          <button
            type="button"
            className={view === 'corsi' ? viewStyles.viewButtonActive : viewStyles.viewButton}
            aria-current={view === 'corsi' ? 'true' : undefined}
            onClick={() => setView('corsi')}
          >
            Corsi
          </button>
          <button
            type="button"
            className={view === 'anagrafiche' ? viewStyles.viewButtonActive : viewStyles.viewButton}
            aria-current={view === 'anagrafiche' ? 'true' : undefined}
            onClick={() => setView('anagrafiche')}
          >
            Anagrafiche
          </button>
          <button
            type="button"
            className={view === 'certificazioni' ? viewStyles.viewButtonActive : viewStyles.viewButton}
            aria-current={view === 'certificazioni' ? 'true' : undefined}
            onClick={() => setView('certificazioni')}
          >
            Certificazioni
          </button>
          <button
            type="button"
            className={view === 'percorsi' ? viewStyles.viewButtonActive : viewStyles.viewButton}
            aria-current={view === 'percorsi' ? 'true' : undefined}
            onClick={() => setView('percorsi')}
          >
            Percorsi
          </button>
        </nav>
      </header>

      {view === 'corsi' ? (
        <>
          <div className={listStyles.header}>
            <ToggleSwitch
              id="courses-to-curate"
              checked={onlyToCurate}
              onChange={setOnlyToCurate}
              label="Solo da curare (importati dal sync)"
            />
            <Button variant="primary" size="md" leftIcon={<Icon name="plus" size={16} />} onClick={() => setShowCreate(true)}>
              Nuovo corso
            </Button>
          </div>

          {courses.isLoading ? (
            <Skeleton rows={6} />
          ) : courses.isError ? (
            <p className={listStyles.errorNotice}>Lettura dei corsi non riuscita. Riprovare più tardi.</p>
          ) : (courses.data ?? []).length === 0 ? (
            <div className={listStyles.empty}>
              <div className={listStyles.emptyIcon}>
                <Icon name="package" size={32} />
              </div>
              <p className={listStyles.emptyTitle}>Nessun corso a catalogo</p>
              <p className={listStyles.emptyDescription}>Crea il primo corso o attendi il sync formativo da Factorial.</p>
              <Button variant="primary" size="md" onClick={() => setShowCreate(true)}>
                Nuovo corso
              </Button>
            </div>
          ) : filtered.length === 0 ? (
            <div className={listStyles.empty}>
              <p className={listStyles.emptyTitle}>Nessun corso da curare</p>
              <p className={listStyles.emptyDescription}>Nessun corso importato dal sync in attesa di attivazione.</p>
            </div>
          ) : (
            <div className={listStyles.tableWrap}>
              <table className={listStyles.table}>
                <thead>
                  <tr>
                    <th>Titolo</th>
                    <th>Area</th>
                    <th>Fornitore</th>
                    <th>Modalità</th>
                    <th>Durata / prezzo</th>
                    <th>Certificazione</th>
                    <th>Compliance</th>
                    <th>Attivo</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((row: CourseListRow) => (
                    <tr key={row.id} className={listStyles.row} onClick={() => openDetail(row.id)}>
                      <td>
                        <button
                          type="button"
                          className={listStyles.rowLink}
                          onClick={(e) => {
                            e.stopPropagation();
                            openDetail(row.id);
                          }}
                        >
                          {row.title}
                        </button>
                      </td>
                      <td>{row.skillAreas.length > 0 ? row.skillAreas.map((a) => a.name).join(', ') : '—'}</td>
                      <td>{row.vendorName || PROVIDER_KIND_LABELS[row.providerKind] || '—'}</td>
                      <td>{DELIVERY_MODE_LABELS[row.deliveryMode] ?? row.deliveryMode}</td>
                      <td>
                        {row.defaultHours !== undefined ? `${row.defaultHours} h` : '—'}
                        {row.defaultCost !== undefined ? ` · ${formatCurrency(row.defaultCost) ?? '—'}` : ''}
                      </td>
                      <td>
                        {row.leadsToCertId ? (
                          <button
                            type="button"
                            className={listStyles.rowLink}
                            onClick={(e) => {
                              e.stopPropagation();
                              openCertification(row.leadsToCertId as string);
                            }}
                          >
                            {row.leadsToCertName}
                          </button>
                        ) : (
                          '—'
                        )}
                      </td>
                      <td>
                        {row.complianceRelated ? (
                          <StatusBadge value="compliance" label={row.complianceFramework || 'Sì'} variant="warning" />
                        ) : (
                          '—'
                        )}
                      </td>
                      <td>{row.active ? 'Sì' : 'No'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      ) : view === 'anagrafiche' ? (
        <AnagraficheSection />
      ) : view === 'certificazioni' ? (
        <CertificationsSection onOpen={openCertification} onGoToAnagrafiche={() => setView('anagrafiche')} />
      ) : (
        <PathsCatalogSection onOpen={openPath} />
      )}

      {showCreate && (
        <CourseEditorModal
          mode="create"
          open={showCreate}
          onClose={() => setShowCreate(false)}
          onSaved={(id) => {
            setShowCreate(false);
            openDetail(id);
          }}
        />
      )}
      {selectedId && <CourseDetailDrawer id={selectedId} onClose={closeDetail} />}
      {selectedCertId && <CertificationDetailDrawer id={selectedCertId} onClose={closeCertification} />}
      {selectedPathId && <PathDetailDrawer id={selectedPathId} onClose={closePath} />}
    </main>
  );
}
