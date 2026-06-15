import { useMemo, useState, type ChangeEvent, type FormEvent } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Button, Icon, SearchInput, Skeleton, ToggleSwitch } from '@mrsmith/ui';
import { useApiClient } from '../api/client';
import type { CompanySearchRow, OpenAPIITEnvelope, Province } from '../api/types';
import styles from './TestPage.module.css';

const numberFormat = new Intl.NumberFormat('it-IT');
const areaFormat = new Intl.NumberFormat('it-IT', {
  maximumFractionDigits: 2,
});
const defaultCompanyProvince = 'AG';
const countHints = ['count', 'record', 'total', 'found', 'result'];
const priceHints = ['price', 'cost', 'amount', 'prezzo'];

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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function displayValue(value: string | null | undefined): string {
  const normalized = value?.trim();
  return normalized ? normalized : '-';
}

function rawPreview(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function normalizeMetricKey(key: string): string {
  return key.replace(/[_\-\s]/g, '').toLowerCase();
}

function findMetric(value: unknown, hints: string[]): string | number | undefined {
  return findMetricInValue(value, hints, new Set<object>());
}

function findMetricInValue(
  value: unknown,
  hints: string[],
  seen: Set<object>,
): string | number | undefined {
  if (!isRecord(value) && !Array.isArray(value)) return undefined;
  if (seen.has(value)) return undefined;
  seen.add(value);

  const entries = Array.isArray(value)
    ? value.map((item, index) => [String(index), item] as const)
    : Object.entries(value);

  for (const [key, nested] of entries) {
    const normalizedKey = normalizeMetricKey(key);
    if (
      hints.some((hint) => normalizedKey.includes(hint)) &&
      (typeof nested === 'number' || typeof nested === 'string')
    ) {
      return nested;
    }
  }

  for (const [, nested] of entries) {
    const found = findMetricInValue(nested, hints, seen);
    if (found !== undefined) return found;
  }

  return undefined;
}

function formatMetric(value: string | number | undefined): string {
  if (typeof value === 'number') return numberFormat.format(value);
  const normalized = value?.trim();
  return normalized ? normalized : 'N/D';
}

function normalizeProvinceInput(value: string): string {
  return value.trim().toUpperCase();
}

export function TestPage() {
  const api = useApiClient();
  const [searchQuery, setSearchQuery] = useState('');
  const [companyProvince, setCompanyProvince] = useState(defaultCompanyProvince);
  const [companyDryRun, setCompanyDryRun] = useState(true);

  const provincesQuery = useQuery({
    queryKey: ['binocolo', 'provinces'],
    queryFn: () => api.get<OpenAPIITEnvelope<Province[]>>('/binocolo/v1/provinces'),
  });

  const companySearch = useMutation({
    mutationFn: () => {
      const params = new URLSearchParams({ dry_run: String(companyDryRun) });
      const province = normalizeProvinceInput(companyProvince);
      if (province) params.set('province', province);
      return api.get<OpenAPIITEnvelope<unknown>>(`/binocolo/v1/companies/search?${params.toString()}`);
    },
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
  const companyData = companySearch.data?.data;
  const companyRows: CompanySearchRow[] = Array.isArray(companyData)
    ? (companyData as CompanySearchRow[])
    : [];
  const dryRunCount = findMetric(companyData, countHints);
  const dryRunPrice = findMetric(companyData, priceHints);
  const companyProvinceFilter = normalizeProvinceInput(companyProvince);
  const companyScopeLabel = companyProvinceFilter ? `provincia ${companyProvinceFilter}` : 'tutte le province';

  function handleCompanyDryRunChange(value: boolean) {
    setCompanyDryRun(value);
    companySearch.reset();
  }

  function handleCompanyProvinceChange(event: ChangeEvent<HTMLInputElement>) {
    setCompanyProvince(event.target.value);
    companySearch.reset();
  }

  function handleCompanySubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    companySearch.mutate();
  }

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

      <section className={`${styles.panel} ${styles.provincePanel}`} aria-labelledby="province-title">
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

      <section className={`${styles.panel} ${styles.companyPanel}`} aria-labelledby="companies-title">
        <div className={styles.panelHeader}>
          <div>
            <div className={styles.endpointLine}>
              <span className={styles.method}>GET</span>
              <span className={styles.path}>/binocolo/v1/companies/search</span>
            </div>
            <h2 id="companies-title" className={styles.sectionTitle}>Company search</h2>
          </div>
          <form className={styles.companyForm} onSubmit={handleCompanySubmit}>
            <label className={styles.provinceField}>
              <span>Provincia</span>
              <input
                type="text"
                value={companyProvince}
                onChange={handleCompanyProvinceChange}
                maxLength={2}
                autoCapitalize="characters"
                autoComplete="off"
                spellCheck={false}
                aria-label="Provincia"
              />
            </label>
            <ToggleSwitch
              id="binocolo-company-dry-run"
              checked={companyDryRun}
              onChange={handleCompanyDryRunChange}
              label="dry_run"
            />
            <Button
              type="submit"
              loading={companySearch.isPending}
              leftIcon={<Icon name="search" />}
            >
              Esegui
            </Button>
          </form>
        </div>

        {companySearch.isIdle ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="search" size={22} />
            </div>
            <p className={styles.stateTitle}>Ricerca pronta</p>
            <p className={styles.stateText}>Premi Esegui per interrogare le aziende: {companyScopeLabel}.</p>
          </div>
        ) : companySearch.isPending ? (
          <div className={styles.skeletonWrap}>
            <Skeleton rows={5} />
          </div>
        ) : companySearch.isError ? (
          <div className={`${styles.statePanel} ${styles.companyState}`} role="alert">
            <div className={styles.stateIcon}>
              <Icon name="triangle-alert" size={22} />
            </div>
            <p className={styles.stateTitle}>Ricerca non disponibile</p>
            <p className={styles.stateText}>{errorLabel(companySearch.error)}</p>
            <Button variant="secondary" size="sm" onClick={() => companySearch.mutate()}>
              Riprova
            </Button>
          </div>
        ) : companyDryRun ? (
          <div className={styles.companyResult}>
            <div className={styles.responseBar}>
              <span>Simulazione: {companyScopeLabel}</span>
              {companySearch.data?.message ? <span>{companySearch.data.message}</span> : null}
            </div>
            <div className={styles.dryRunGrid}>
              <div className={styles.metricBox}>
                <span>Risultati</span>
                <strong>{formatMetric(dryRunCount)}</strong>
              </div>
              <div className={styles.metricBox}>
                <span>Prezzo</span>
                <strong>{formatMetric(dryRunPrice)}</strong>
              </div>
            </div>
            <div className={styles.rawBlock}>
              <span>Risposta</span>
              <pre>{rawPreview(companyData)}</pre>
            </div>
          </div>
        ) : companyRows.length === 0 ? (
          <div className={`${styles.statePanel} ${styles.companyState}`}>
            <div className={styles.stateIcon}>
              <Icon name="search" size={22} />
            </div>
            <p className={styles.stateTitle}>Nessuna azienda trovata</p>
            <p className={styles.stateText}>La ricerca non ha restituito aziende per {companyScopeLabel}.</p>
          </div>
        ) : (
          <>
            <div className={styles.responseBar}>
              <span>{companyRows.length} aziende visualizzate</span>
              {companySearch.data?.message ? <span>{companySearch.data.message}</span> : null}
            </div>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Azienda</th>
                    <th>Partita IVA</th>
                    <th>Codice fiscale</th>
                    <th>Comune</th>
                    <th>Stato</th>
                    <th>ID</th>
                  </tr>
                </thead>
                <tbody>
                  {companyRows.map((company, index) => (
                    <tr key={company.id || `${company.companyName ?? 'company'}-${index}`}>
                      <td>{displayValue(company.companyName)}</td>
                      <td className={styles.codeCell}>{displayValue(company.vatCode)}</td>
                      <td className={styles.codeCell}>{displayValue(company.taxCode)}</td>
                      <td>{displayValue(company.address?.registeredOffice?.town)}</td>
                      <td>{displayValue(company.activityStatus)}</td>
                      <td className={styles.codeCell}>{displayValue(company.id)}</td>
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
