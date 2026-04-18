import { Skeleton } from '@mrsmith/ui';
import { useInvoiceLines } from '../api/queries';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';

export default function FatturePrometeusPage() {
  const q = useInvoiceLines();

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Fatture Prometeus</h1>
      <p className={shared.info}>Ultime 2000 righe fattura trasmesse da WHMCS ad Alyante.</p>

      {q.isLoading && <Skeleton rows={10} />}
      {q.isError && <div className={shared.error}>Errore nel caricamento delle fatture.</div>}

      {q.data && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Raggruppamento</th>
                <th>Ragione sociale</th>
                <th>Nome</th>
                <th>Cognome</th>
                <th>Partita IVA</th>
                <th>Codice fiscale</th>
                <th>Iso</th>
                <th>PF</th>
                <th>Indirizzo</th>
                <th>Civico</th>
                <th>CAP</th>
                <th>Comune</th>
                <th>PV</th>
                <th>Nazione</th>
                <th>Documento</th>
                <th>Data doc.</th>
                <th>Causale</th>
                <th className={shared.numCol}>Linea</th>
                <th className={shared.numCol}>Qtà</th>
                <th>Descrizione</th>
                <th className={shared.numCol}>Prezzo</th>
                <th>Inizio periodo</th>
                <th>Fine periodo</th>
                <th>Modalità pagamento</th>
                <th className={shared.numCol}>IVA</th>
                <th className={shared.numCol}>Bollo</th>
                <th>Codice cliente ERP</th>
                <th>Tipo</th>
                <th className={shared.numCol}>Invoice ID</th>
                <th className={shared.numCol}>ID</th>
              </tr>
            </thead>
            <tbody>
              {q.data.map((l, i) => (
                <tr key={l.id} style={{ animationDelay: `${Math.min(i * 8, 300)}ms` }}>
                  <td>{l.raggruppamento ?? ''}</td>
                  <td>{l.ragionesocialecliente ?? ''}</td>
                  <td>{l.nomecliente ?? ''}</td>
                  <td>{l.cognomecliente ?? ''}</td>
                  <td className={shared.mono}>{l.partitaiva ?? ''}</td>
                  <td className={shared.mono}>{l.codicefiscale ?? ''}</td>
                  <td>{l.codiceiso ?? ''}</td>
                  <td>{l.flagpersonafisica ?? ''}</td>
                  <td>{l.indirizzo ?? ''}</td>
                  <td>{l.numerocivico ?? ''}</td>
                  <td>{l.cap ?? ''}</td>
                  <td>{l.comune ?? ''}</td>
                  <td>{l.provincia ?? ''}</td>
                  <td>{l.nazione ?? ''}</td>
                  <td className={shared.mono}>{l.numerodocumento ?? ''}</td>
                  <td>{l.datadocumento ?? ''}</td>
                  <td>{l.causale ?? ''}</td>
                  <td className={shared.numCol}>{l.numerolinea ?? ''}</td>
                  <td className={shared.numCol}>{l.quantita ?? ''}</td>
                  <td>{l.descrizioneriga ?? ''}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(l.prezzo)}</td>
                  <td>{l.datainizioperiodo ?? ''}</td>
                  <td>{l.datafineperiodo ?? ''}</td>
                  <td>{l.modalitapagamento ?? ''}</td>
                  <td className={shared.numCol}>{l.ivariga ?? ''}</td>
                  <td className={shared.numCol}>{l.bollo ?? ''}</td>
                  <td>{l.codiceclienteerp ?? ''}</td>
                  <td>{l.tipo ?? ''}</td>
                  <td className={shared.numCol}>{l.invoiceid ?? ''}</td>
                  <td className={shared.numCol}>{l.id}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {q.data.length === 0 && <div className={shared.empty}>Nessuna riga fattura.</div>}
        </div>
      )}
    </div>
  );
}
