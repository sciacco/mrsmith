// Certificazioni del catalogo (#162, §Catalogo 4): elenco navigabile che
// apre il dettaglio (titolari, corsi, regole, export). L'anagrafica di base
// resta gestita anche da Anagrafiche; questa vista è il punto d'ingresso al
// dettaglio operativo, non duplica la modifica.

import { Button, Icon, Skeleton } from '@mrsmith/ui';
import { useTrainingCertifications } from '../../api/queries';
import listStyles from '../../pages/RequestsPage/listPage.module.css';

export function CertificationsSection({
  onOpen,
  onGoToAnagrafiche,
}: {
  onOpen: (id: string) => void;
  onGoToAnagrafiche: () => void;
}) {
  const certifications = useTrainingCertifications();

  if (certifications.isLoading) return <Skeleton rows={5} />;
  if (certifications.isError) {
    return <p className={listStyles.errorNotice}>Lettura delle certificazioni non riuscita. Riprovare più tardi.</p>;
  }
  const rows = certifications.data ?? [];
  if (rows.length === 0) {
    return (
      <div className={listStyles.empty}>
        <div className={listStyles.emptyIcon}>
          <Icon name="shield" size={32} />
        </div>
        <p className={listStyles.emptyTitle}>Nessuna certificazione a catalogo</p>
        <p className={listStyles.emptyDescription}>Le certificazioni si creano dalla scheda Anagrafiche.</p>
        <Button variant="primary" size="md" onClick={onGoToAnagrafiche}>
          Vai ad Anagrafiche
        </Button>
      </div>
    );
  }

  return (
    <div className={listStyles.tableWrap}>
      <table className={listStyles.table}>
        <thead>
          <tr>
            <th>Nome</th>
            <th>Codice</th>
            <th>Emittente</th>
            <th>Area</th>
            <th>Validità tipica</th>
            <th>Attiva</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((c) => (
            <tr key={c.id} className={listStyles.row} onClick={() => onOpen(c.id)}>
              <td>
                <button type="button" className={listStyles.rowLink} onClick={(e) => { e.stopPropagation(); onOpen(c.id); }}>
                  {c.name}
                </button>
              </td>
              <td>{c.code}</td>
              <td>{c.issuerVendorName || '—'}</td>
              <td>{c.skillAreaName || '—'}</td>
              <td>{c.typicalValidityMonths !== undefined ? `${c.typicalValidityMonths} mesi` : '—'}</td>
              <td>{c.active ? 'Sì' : 'No'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
