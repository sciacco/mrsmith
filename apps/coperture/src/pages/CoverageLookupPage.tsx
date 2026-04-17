import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { SingleSelect, Skeleton } from '@mrsmith/ui';
import { useAddresses, useCities, useCoverage, useHouseNumbers, useStates } from '../api/queries';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import type { CoverageResult, LocationOption } from '../types';
import shared from './shared.module.css';
import styles from './CoverageLookupPage.module.css';

interface SubmittedSearch {
  houseNumberId: number;
  labels: [string, string, string, string];
}

function resolveLabel(options: LocationOption[], selected: number | null): string {
  if (selected === null) return '';
  return options.find((option) => option.id === selected)?.name ?? '';
}

function normalizeApiError(error: unknown): ApiError | null {
  return error instanceof ApiError ? error : null;
}

function firstActiveError(errors: Array<unknown>): ApiError | null {
  for (const error of errors) {
    const apiError = normalizeApiError(error);
    if (apiError) return apiError;
  }
  return null;
}

function operatorInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  return (parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '');
}

function OperatorBadge({ result }: { result: CoverageResult }) {
  const [failed, setFailed] = useState(false);
  const initials = operatorInitials(result.operator_name).toUpperCase() || 'OP';

  return (
    <div className={styles.operatorCell}>
      {result.logo_url && !failed ? (
        <img
          className={styles.operatorLogo}
          src={result.logo_url}
          alt=""
          aria-hidden="true"
          onError={() => setFailed(true)}
        />
      ) : (
        <span className={styles.operatorFallback}>{initials}</span>
      )}
      <span className={styles.operatorName}>{result.operator_name}</span>
    </div>
  );
}

export function CoverageLookupPage() {
  const [stateId, setStateId] = useState<number | null>(null);
  const [cityId, setCityId] = useState<number | null>(null);
  const [addressId, setAddressId] = useState<number | null>(null);
  const [houseNumberId, setHouseNumberId] = useState<number | null>(null);
  const [submitted, setSubmitted] = useState<SubmittedSearch | null>(null);

  const statesQ = useStates();
  const citiesQ = useCities(stateId);
  const addressesQ = useAddresses(cityId);
  const houseNumbersQ = useHouseNumbers(addressId);
  const coverageQ = useCoverage(submitted?.houseNumberId ?? null);

  const stateOptions = statesQ.data ?? [];
  const cityOptions = citiesQ.data ?? [];
  const addressOptions = addressesQ.data ?? [];
  const houseNumberOptions = houseNumbersQ.data ?? [];

  const liveErrors = firstActiveError([
    statesQ.error,
    stateId !== null ? citiesQ.error : null,
    cityId !== null ? addressesQ.error : null,
    addressId !== null ? houseNumbersQ.error : null,
    submitted !== null ? coverageQ.error : null,
  ]);

  function handleStateChange(value: number | null) {
    setStateId(value);
    setCityId(null);
    setAddressId(null);
    setHouseNumberId(null);
    setSubmitted(null);
  }

  function handleCityChange(value: number | null) {
    setCityId(value);
    setAddressId(null);
    setHouseNumberId(null);
    setSubmitted(null);
  }

  function handleAddressChange(value: number | null) {
    setAddressId(value);
    setHouseNumberId(null);
    setSubmitted(null);
  }

  function handleHouseNumberChange(value: number | null) {
    setHouseNumberId(value);
    setSubmitted(null);
  }

  function handleSearch() {
    if (houseNumberId === null) return;
    const labels: [string, string, string, string] = [
      resolveLabel(stateOptions, stateId),
      resolveLabel(cityOptions, cityId),
      resolveLabel(addressOptions, addressId),
      resolveLabel(houseNumberOptions, houseNumberId),
    ];
    setSubmitted({ houseNumberId, labels });
  }

  function handleReset() {
    setStateId(null);
    setCityId(null);
    setAddressId(null);
    setHouseNumberId(null);
    setSubmitted(null);
  }

  const results = coverageQ.data ?? [];
  const submittedAddress = submitted?.labels.filter(Boolean).join(' · ') ?? '';

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Ricerca copertura</h1>

      <div className={shared.toolbar}>
        <div className={`${shared.field} ${styles.fieldWide}`}>
          <label>Provincia</label>
          <SingleSelect
            options={stateOptions.map((option) => ({ value: option.id, label: option.name }))}
            selected={stateId}
            onChange={handleStateChange}
            placeholder="Seleziona provincia..."
          />
        </div>

        <div className={`${shared.field} ${styles.fieldWide} ${stateId === null ? styles.selectDisabled : ''}`}>
          <label>Comune</label>
          <SingleSelect
            options={cityOptions.map((option) => ({ value: option.id, label: option.name }))}
            selected={cityId}
            onChange={handleCityChange}
            placeholder="Seleziona comune..."
          />
        </div>

        <div className={`${shared.field} ${styles.fieldWide} ${cityId === null ? styles.selectDisabled : ''}`}>
          <label>Indirizzo</label>
          <SingleSelect
            options={addressOptions.map((option) => ({ value: option.id, label: option.name }))}
            selected={addressId}
            onChange={handleAddressChange}
            placeholder="Seleziona indirizzo..."
          />
        </div>

        <div className={`${shared.field} ${styles.fieldNarrow} ${addressId === null ? styles.selectDisabled : ''}`}>
          <label>Numero civico</label>
          <SingleSelect
            options={houseNumberOptions.map((option) => ({ value: option.id, label: option.name }))}
            selected={houseNumberId}
            onChange={handleHouseNumberChange}
            placeholder="Seleziona numero civico..."
          />
        </div>

        <button className={shared.btnPrimary} onClick={handleSearch} disabled={houseNumberId === null}>
          Cerca
        </button>
        <button className={shared.btnSecondary} onClick={handleReset}>
          Reimposta filtri
        </button>
      </div>

      {submitted && (
        <div className={styles.summary}>
          <span className={styles.summaryLabel}>Indirizzo selezionato</span>
          <span className={styles.summaryValue}>{submittedAddress}</span>
        </div>
      )}

      {liveErrors?.status === 503 && <ServiceUnavailable service="Coperture" />}
      {liveErrors !== null && liveErrors.status !== 503 && (
        <div className={styles.error}>Errore nel caricamento dei dati. Riprova.</div>
      )}

      {!submitted && liveErrors === null && (
        <div className={shared.empty}>
          Seleziona un indirizzo completo e premi Cerca per visualizzare i profili disponibili.
        </div>
      )}

      {submitted && coverageQ.isLoading && liveErrors === null && <Skeleton rows={5} />}

      {submitted && !coverageQ.isLoading && liveErrors === null && results.length > 0 && (
        <>
          <div className={shared.info}>{results.length} profili disponibili</div>
          <div className={shared.tableWrap}>
            <table className={shared.table}>
              <thead>
                <tr>
                  <th>Operatore</th>
                  <th>Tecnologia</th>
                  <th>Profili</th>
                  <th>Dettagli</th>
                </tr>
              </thead>
              <tbody>
                {results.map((result, index) => (
                  <tr key={`${result.coverage_id}-${index}`} style={{ animationDelay: `${Math.min(index * 15, 300)}ms` }}>
                    <td>
                      <OperatorBadge result={result} />
                    </td>
                    <td className={styles.tech}>{result.tech}</td>
                    <td>
                      {result.profiles.length > 0 ? (
                        <ul className={styles.list}>
                          {result.profiles.map((profile, profileIndex) => (
                            <li key={`${result.coverage_id}-profile-${profileIndex}`}>{profile.name}</li>
                          ))}
                        </ul>
                      ) : (
                        <span className={styles.muted}>—</span>
                      )}
                    </td>
                    <td>
                      {result.details.length > 0 ? (
                        <ul className={styles.list}>
                          {result.details.map((detail, detailIndex) => (
                            <li key={`${result.coverage_id}-detail-${detailIndex}`} className={styles.listItem}>
                              <span className={styles.listItemLabel}>{detail.type_name}:</span>
                              <span>{detail.value}</span>
                            </li>
                          ))}
                        </ul>
                      ) : (
                        <span className={styles.muted}>—</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {submitted && !coverageQ.isLoading && liveErrors === null && results.length === 0 && (
        <div className={shared.empty}>Nessuna copertura disponibile per questo civico.</div>
      )}
    </div>
  );
}
