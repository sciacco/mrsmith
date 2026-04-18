import { Skeleton } from '@mrsmith/ui';
import { useNavigate } from 'react-router-dom';
import { useSalesOrders } from '../api/queries';
import { formatDate } from '../utils/format';
import shared from './shared.module.css';

export default function OrdiniSalesPage() {
  const q = useSalesOrders();
  const navigate = useNavigate();

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Ordini Sales</h1>
      <p className={shared.info}>Ordini attivi o inviati.</p>

      {q.isLoading && <Skeleton rows={10} />}
      {q.isError && <div className={shared.error}>Errore nel caricamento degli ordini.</div>}

      {q.data && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th className={shared.numCol}>ID</th>
                <th>Tipo documento</th>
                <th>Codice ordine</th>
                <th>Sostituisce</th>
                <th>Cliente</th>
                <th>Data documento</th>
                <th>Tipo servizi</th>
                <th>Tipo ordine</th>
                <th>Data conferma</th>
                <th>Stato</th>
                <th>Dal CP?</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {q.data.map((o, i) => (
                <tr key={o.id} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                  <td className={shared.numCol}>{o.id}</td>
                  <td>{o.cdlan_tipodoc ?? ''}</td>
                  <td className={shared.mono}>{o.codice_ordine ?? ''}</td>
                  <td className={shared.mono}>{o.cdlan_sost_ord ?? ''}</td>
                  <td>{o.cdlan_cliente ?? ''}</td>
                  <td>{formatDate(o.cdlan_datadoc)}</td>
                  <td>{o.tipo_di_servizi ?? ''}</td>
                  <td>{o.tipo_di_ordine ?? ''}</td>
                  <td>{formatDate(o.cdlan_dataconferma)}</td>
                  <td>{o.cdlan_stato ?? ''}</td>
                  <td>{o.dal_cp ?? ''}</td>
                  <td>
                    <button
                      className={shared.rowAction}
                      onClick={() => navigate(`/ordini-sales/${o.id}`)}
                      aria-label={`Apri ordine ${o.id}`}
                      title="Apri dettaglio ordine"
                    >
                      →
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {q.data.length === 0 && <div className={shared.empty}>Nessun ordine attivo o inviato.</div>}
        </div>
      )}
    </div>
  );
}
