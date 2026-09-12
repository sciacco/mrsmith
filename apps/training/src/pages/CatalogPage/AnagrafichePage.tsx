// Catalogo · Anagrafiche (#158, §Catalogo 5, scomposta dalle sottoviste di
// CatalogPage): fornitori, aree di competenza, certificazioni e team in
// gestione anagrafica di base.

import { AnagraficheSection } from '../../components/catalog/AnagraficheSection';
import listStyles from '../RequestsPage/listPage.module.css';

export function AnagrafichePage() {
  return (
    <main className={listStyles.page}>
      <header className={listStyles.header}>
        <div>
          <h1 className={listStyles.title}>Anagrafiche</h1>
          <p className={listStyles.subtitle}>Fornitori, aree di competenza, certificazioni e team.</p>
        </div>
      </header>
      <AnagraficheSection />
    </main>
  );
}
