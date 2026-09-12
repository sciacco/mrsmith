// Catalogo · Percorsi (#162, §Catalogo 5, scomposta dalle sottoviste di
// CatalogPage): elenco, creazione/modifica dell'anagrafica. Passi, assegnatari
// e progresso vivono nel dettaglio (PathDetailDrawer.tsx), indirizzato con lo
// stesso idioma a parametro di query di corsi e certificazioni (onOpen).

import { useSearchParams } from 'react-router-dom';
import { PathDetailDrawer } from '../../components/catalog/PathDetailDrawer';
import { PathsCatalogSection } from '../../components/catalog/PathsCatalogSection';
import listStyles from '../RequestsPage/listPage.module.css';

export function PathsPage() {
  const [params, setParams] = useSearchParams();
  const selectedPathId = params.get('pathId');

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
          <h1 className={listStyles.title}>Percorsi</h1>
          <p className={listStyles.subtitle}>Sequenze di corsi o certificazioni assegnabili alle persone.</p>
        </div>
      </header>
      <PathsCatalogSection onOpen={openPath} />
      {selectedPathId && <PathDetailDrawer id={selectedPathId} onClose={closePath} />}
    </main>
  );
}
