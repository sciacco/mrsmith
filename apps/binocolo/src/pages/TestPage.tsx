import { useMemo, useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useQuery } from '@tanstack/react-query';
import { Button, Icon, SearchInput, Skeleton } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import type { OpenAPIITEnvelope, Province } from '../api/types';
import styles from './TestPage.module.css';

const numberFormat = new Intl.NumberFormat('it-IT');
const areaFormat = new Intl.NumberFormat('it-IT', {
  maximumFractionDigits: 2,
});

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 503) return 'OpenAPI.it non e configurato in questo ambiente.';
    if (error.status === 502) return 'OpenAPI.it non ha risposto correttamente.';
    if (error.status === 401) return 'Sessione non valida.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    return `Richiesta non riuscita (${error.status}).`;
  }
  if (error instanceof Error) return error.message;
  return 'Richiesta non riuscita.';
}

export function TestPage() {
  const api = useApiClient();
  const [searchQuery, setSearchQuery] = useState('');

  const provincesQuery = useQuery({
    queryKey: ['binocolo', 'provinces'],
    queryFn: () => api.get<OpenAPIITEnvelope<Province[]>>('/binocolo/v1/provinces'),
  });

  const provinces = provincesQuery.data?.data ?? [];
  const filteredProvinces = useMemo(() => {
    const needle = searchQuery.trim().toLowerCase();
    if (!needle) return provinces;

    return provinces.filter((province) =>
      [
        province.sigla,
        province.provincia,
        province.regione,
        province.istat,
      ].some((value) => value.toLowerCase().includes(needle)),
    );
  }, [provinces, searchQuery]);

  return (
    <div className={styles.page}>
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>Test</h1>
          <p className={styles.pageSubtitle}>Verifica la lista delle province italiane.</p>
        </div>
        <Button
          variant="secondary"
          onClick={() => void provincesQuery.refetch()}
          loading={provincesQuery.isFetching}
          leftIcon={<Icon name="refresh-cw" />}
        >
          Aggiorna
        </Button>
      </div>

      <section className={styles.panel} aria-labelledby="province-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>GET</span>
              <span className={styles.path}>/binocolo/v1/provinces</span>
            </div>
            <h2 id="province-title" className={styles.sectionTitle}>Province</h2>
          </div>
          <div className={styles.searchWrap}>
            <SearchInput
              value={searchQuery}
              onChange={setSearchQuery}
              placeholder="Cerca provincia..."
            />
          </div>
        </div>

        {provincesQuery.isLoading ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={8} />
          </div>
        ) : provincesQuery.isError ? (
          <div className={styles.statePanel} role="alert">
            <div className={styles.stateIcon}>
              <Icon name="triangle-alert" size={22} />
            </div>
            <p className={styles.stateTitle}>Province non disponibili</p>
            <p className={styles.stateText}>{errorLabel(provincesQuery.error)}</p>
            <Button variant="secondary" size="sm" onClick={() => void provincesQuery.refetch()}>
              Riprova
            </Button>
          </div>
        ) : filteredProvinces.length === 0 ? (
          <div className={styles.statePanel}>
            <div className={styles.stateIcon}>
              <Icon name="search" size={22} />
            </div>
            <p className={styles.stateTitle}>Nessuna provincia trovata</p>
            <p className={styles.stateText}>Modifica la ricerca per vedere altri risultati.</p>
          </div>
        ) : (
          <>
            <div className={styles.responseBar}>
              <span>{filteredProvinces.length} province visualizzate</span>
              {provincesQuery.data?.message ? <span>{provincesQuery.data.message}</span> : null}
            </div>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Sigla</th>
                    <th>Provincia</th>
                    <th>Regione</th>
                    <th className={styles.numberCol}>Comuni</th>
                    <th className={styles.numberCol}>Residenti</th>
                    <th className={styles.numberCol}>Superficie</th>
                    <th>ISTAT</th>
                  </tr>
                </thead>
                <tbody>
                  {filteredProvinces.map((province) => (
                    <tr key={province.sigla}>
                      <td className={styles.codeCell}>{province.sigla}</td>
                      <td>{province.provincia}</td>
                      <td>{province.regione}</td>
                      <td className={styles.numberCol}>{numberFormat.format(province.num_comuni)}</td>
                      <td className={styles.numberCol}>{numberFormat.format(province.residenti)}</td>
                      <td className={styles.numberCol}>{areaFormat.format(province.superficie)} kmq</td>
                      <td className={styles.codeCell}>{province.istat}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </section>
    </div>
  );
}
