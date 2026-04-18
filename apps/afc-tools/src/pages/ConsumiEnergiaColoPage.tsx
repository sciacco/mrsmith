import { useState } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { useEnergiaColoDetail, useEnergiaColoPivot } from '../api/queries';
import { formatDate, formatNumber } from '../utils/format';
import shared from './shared.module.css';

function currentYear(): string {
  return String(new Date().getFullYear());
}

export default function ConsumiEnergiaColoPage() {
  // Deviation 1:1d: default to current year (instead of empty Appsmith input).
  const [year, setYear] = useState(currentYear);
  const [submittedYear, setSubmittedYear] = useState(currentYear);

  const pivotQ = useEnergiaColoPivot(submittedYear, submittedYear !== '');
  const detailQ = useEnergiaColoDetail(submittedYear, submittedYear !== '');

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Consumi Energia Colo</h1>

      <div className={shared.toolbar}>
        <div className={shared.field}>
          <label>Anno</label>
          <input
            type="number"
            min={2000}
            max={2100}
            value={year}
            onChange={(e) => setYear(e.target.value)}
          />
        </div>
        <button
          className={shared.btnPrimary}
          onClick={() => setSubmittedYear(year)}
          disabled={year === ''}
        >
          Cerca
        </button>
      </div>

      <h2 style={{ fontSize: '1.1rem', margin: 'var(--space-6) 0 var(--space-3)' }}>Riepilogo mensile per cliente</h2>
      {pivotQ.isLoading && <Skeleton rows={6} />}
      {pivotQ.isError && <div className={shared.error}>Errore nel caricamento del riepilogo.</div>}
      {pivotQ.data && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Cliente</th>
                <th className={shared.numCol}>Gennaio</th>
                <th className={shared.numCol}>Febbraio</th>
                <th className={shared.numCol}>Marzo</th>
                <th className={shared.numCol}>Aprile</th>
                <th className={shared.numCol}>Maggio</th>
                <th className={shared.numCol}>Giugno</th>
                <th className={shared.numCol}>Luglio</th>
                <th className={shared.numCol}>Agosto</th>
                <th className={shared.numCol}>Settembre</th>
                <th className={shared.numCol}>Ottobre</th>
                <th className={shared.numCol}>Novembre</th>
                <th className={shared.numCol}>Dicembre</th>
              </tr>
            </thead>
            <tbody>
              {pivotQ.data.map((p, i) => (
                <tr key={`${p.customer ?? 'row'}-${i}`} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                  <td>{p.customer ?? ''}</td>
                  <td className={shared.numCol}>{formatNumber(p.gennaio)}</td>
                  <td className={shared.numCol}>{formatNumber(p.febbraio)}</td>
                  <td className={shared.numCol}>{formatNumber(p.marzo)}</td>
                  <td className={shared.numCol}>{formatNumber(p.aprile)}</td>
                  <td className={shared.numCol}>{formatNumber(p.maggio)}</td>
                  <td className={shared.numCol}>{formatNumber(p.giugno)}</td>
                  <td className={shared.numCol}>{formatNumber(p.luglio)}</td>
                  <td className={shared.numCol}>{formatNumber(p.agosto)}</td>
                  <td className={shared.numCol}>{formatNumber(p.settembre)}</td>
                  <td className={shared.numCol}>{formatNumber(p.ottobre)}</td>
                  <td className={shared.numCol}>{formatNumber(p.novembre)}</td>
                  <td className={shared.numCol}>{formatNumber(p.dicembre)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {pivotQ.data.length === 0 && <div className={shared.empty}>Nessun consumo registrato per l&apos;anno selezionato.</div>}
        </div>
      )}

      <h2 style={{ fontSize: '1.1rem', margin: 'var(--space-6) 0 var(--space-3)' }}>Dettaglio per periodo</h2>
      {detailQ.isLoading && <Skeleton rows={6} />}
      {detailQ.isError && <div className={shared.error}>Errore nel caricamento del dettaglio.</div>}
      {detailQ.data && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Cliente</th>
                <th>Inizio periodo</th>
                <th>Fine periodo</th>
                <th className={shared.numCol}>Consumo</th>
                <th className={shared.numCol}>Importo</th>
                <th className={shared.numCol}>PUN</th>
                <th className={shared.numCol}>Coefficiente</th>
                <th className={shared.numCol}>Fisso CU</th>
                <th className={shared.numCol}>Eccedenti</th>
                <th className={shared.numCol}>Importo eccedenti</th>
                <th>Tipo variabile</th>
              </tr>
            </thead>
            <tbody>
              {detailQ.data.map((d, i) => (
                <tr key={`${d.customer ?? 'row'}-${d.start_period ?? ''}-${i}`} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                  <td>{d.customer ?? ''}</td>
                  <td>{formatDate(d.start_period)}</td>
                  <td>{formatDate(d.end_period)}</td>
                  <td className={shared.numCol}>{formatNumber(d.consumo)}</td>
                  <td className={shared.numCol}>{formatNumber(d.amount)}</td>
                  <td className={shared.numCol}>{formatNumber(d.pun)}</td>
                  <td className={shared.numCol}>{formatNumber(d.coefficiente)}</td>
                  <td className={shared.numCol}>{formatNumber(d.fisso_cu)}</td>
                  <td className={shared.numCol}>{formatNumber(d.eccedenti)}</td>
                  <td className={shared.numCol}>{formatNumber(d.importo_eccedenti)}</td>
                  <td>{d.tipo_variabile ?? ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {detailQ.data.length === 0 && <div className={shared.empty}>Nessun dettaglio nel periodo.</div>}
        </div>
      )}
    </div>
  );
}
