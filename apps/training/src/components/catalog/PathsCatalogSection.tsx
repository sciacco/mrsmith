// Percorsi di catalogo (#162, §Catalogo 5): elenco, creazione/modifica
// dell'anagrafica (disattivazione tramite il campo active, stesso idioma di
// CourseInput.active). Passi, assegnatari e progresso vivono nel dettaglio
// (PathDetailDrawer.tsx), indirizzato da CatalogPage con lo stesso idioma a
// parametro di query di corsi e certificazioni (onOpen).

import { useState } from 'react';
import { Button, Icon, Skeleton, StatusBadge } from '@mrsmith/ui';
import { useTrainingPaths } from '../../api/queries';
import type { PathListRow } from '../../api/types';
import { PathEditorModal } from './PathEditorModal';
import listStyles from '../../pages/RequestsPage/listPage.module.css';
import styles from './PathsCatalogSection.module.css';

export function PathsCatalogSection({ onOpen }: { onOpen: (id: string) => void }) {
  const paths = useTrainingPaths();
  const [showCreate, setShowCreate] = useState(false);

  return (
    <div className={styles.tableWrap}>
      <div className={listStyles.header}>
        <Button variant="primary" size="md" leftIcon={<Icon name="plus" size={16} />} onClick={() => setShowCreate(true)}>
          Nuovo percorso
        </Button>
      </div>

      {paths.isLoading ? (
        <Skeleton rows={5} />
      ) : paths.isError ? (
        <p className={listStyles.errorNotice}>Lettura dei percorsi non riuscita. Riprovare più tardi.</p>
      ) : (paths.data ?? []).length === 0 ? (
        <div className={listStyles.empty}>
          <div className={listStyles.emptyIcon}>
            <Icon name="route" size={32} />
          </div>
          <p className={listStyles.emptyTitle}>Nessun percorso a catalogo</p>
          <p className={listStyles.emptyDescription}>Crea il primo percorso per definire una sequenza di corsi o certificazioni.</p>
          <Button variant="primary" size="md" onClick={() => setShowCreate(true)}>
            Nuovo percorso
          </Button>
        </div>
      ) : (
        <div className={listStyles.tableWrap}>
          <table className={listStyles.table}>
            <thead>
              <tr>
                <th>Codice</th>
                <th>Nome</th>
                <th>Area</th>
                <th className={styles.numCell}>Passi</th>
                <th className={styles.numCell}>Assegnatari</th>
                <th>Attivo</th>
              </tr>
            </thead>
            <tbody>
              {(paths.data ?? []).map((p: PathListRow) => (
                <tr key={p.id} className={listStyles.row} onClick={() => onOpen(p.id)}>
                  <td>
                    <button
                      type="button"
                      className={listStyles.rowLink}
                      onClick={(e) => {
                        e.stopPropagation();
                        onOpen(p.id);
                      }}
                    >
                      {p.code}
                    </button>
                  </td>
                  <td>{p.name}</td>
                  <td>{p.skillAreaName || '—'}</td>
                  <td className={styles.numCell}>{p.stepsCount}</td>
                  <td className={styles.numCell}>{p.assigneesCount}</td>
                  <td>
                    <StatusBadge value={p.active ? 'active' : 'inactive'} label={p.active ? 'Sì' : 'No'} variant={p.active ? 'success' : 'neutral'} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <PathEditorModal
          mode="create"
          open={showCreate}
          onClose={() => setShowCreate(false)}
          onSaved={(id) => {
            setShowCreate(false);
            onOpen(id);
          }}
        />
      )}
    </div>
  );
}
