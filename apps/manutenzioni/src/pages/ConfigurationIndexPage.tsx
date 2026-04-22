import { Button, Icon } from '@mrsmith/ui';
import { useNavigate } from 'react-router-dom';
import shared from './shared.module.css';

const resources = [
  { key: 'sites', title: 'Siti', description: 'Sedi e data center usati nelle manutenzioni.' },
  { key: 'technical-domains', title: 'Domini tecnici', description: 'Aree tecniche e operative.' },
  { key: 'maintenance-kinds', title: 'Tipi manutenzione', description: 'Classificazione principale della manutenzione.' },
  { key: 'customer-scopes', title: 'Ambiti clienti', description: 'Perimetro clienti coinvolto.' },
  { key: 'service-taxonomy', title: 'Servizi', description: 'Tassonomia servizi collegata ai domini.' },
  { key: 'reason-classes', title: 'Motivi', description: 'Motivazioni operative ricorrenti.' },
  { key: 'impact-effects', title: 'Effetti impatto', description: 'Effetti attesi sui servizi.' },
  { key: 'quality-flags', title: 'Segnali qualità', description: 'Controlli e indicatori editoriali.' },
  { key: 'target-types', title: 'Tipi target', description: 'Categorie di oggetti impattati.' },
  { key: 'notice-channels', title: 'Canali comunicazione', description: 'Canali disponibili per le comunicazioni.' },
];

export function ConfigurationIndexPage() {
  const navigate = useNavigate();
  return (
    <section className={shared.page}>
      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>Configurazione</h1>
          <p className={shared.pageSubtitle}>
            Gestisci valori attivi e non attivi usati nel registro manutenzioni.
          </p>
        </div>
      </div>
      <div className={shared.tableCard}>
        <div className={shared.tableScroll}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Risorsa</th>
                <th>Descrizione</th>
                <th className={shared.actionsCell}>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {resources.map((resource) => (
                <tr key={resource.key}>
                  <td>
                    <strong>{resource.title}</strong>
                  </td>
                  <td className={shared.muted}>{resource.description}</td>
                  <td className={shared.actionsCell}>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => navigate(`/manutenzioni/configurazione/${resource.key}`)}
                      rightIcon={<Icon name="chevron-right" size={16} />}
                    >
                      Apri
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}
