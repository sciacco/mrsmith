// Catalogo · Certificazioni (#162, §Catalogo 4, scomposta dalle sottoviste di
// CatalogPage): punto d'ingresso operativo (titolari, corsi, regole, export)
// — Anagrafiche resta la gestione anagrafica di base, non duplicata qui.

import { useNavigate, useSearchParams } from 'react-router-dom';
import { CertificationDetailDrawer } from '../../components/catalog/CertificationDetailDrawer';
import { CertificationsSection } from '../../components/catalog/CertificationsSection';
import listStyles from '../RequestsPage/listPage.module.css';

export function CertificationsPage() {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const selectedCertId = params.get('certId');

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

  return (
    <main className={listStyles.page}>
      <header className={listStyles.header}>
        <div>
          <h1 className={listStyles.title}>Certificazioni</h1>
          <p className={listStyles.subtitle}>Titolari, corsi che le rilasciano, regole collegate ed export.</p>
        </div>
      </header>
      <CertificationsSection onOpen={openCertification} onGoToAnagrafiche={() => navigate('/catalogo/anagrafiche')} />
      {selectedCertId && <CertificationDetailDrawer id={selectedCertId} onClose={closeCertification} />}
    </main>
  );
}
