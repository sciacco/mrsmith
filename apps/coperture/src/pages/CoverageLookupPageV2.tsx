import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { Icon, SingleSelect, Skeleton } from '@mrsmith/ui';
import { useAddresses, useCities, useCoverage, useHouseNumbers, useStates } from '../api/queries';
import { ServiceUnavailable } from '../components/ServiceUnavailable';
import type { CoverageResult, LocationOption } from '../types';
import { DISTANCE_LABEL, rankCoverage, type DistancePerf, type RankedCoverage } from './ranking';
import shared from './shared.module.css';
import styles from './CoverageLookupPageV2.module.css';

const BAR_MAX_MBPS = 2500;

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

function displayTech(tech: string): string {
  const key = tech.trim().toUpperCase();
  if (key === 'FTTO') return 'FIBRA DEDICATA';
  return tech;
}

function techChipClass(tech: string): string {
  const key = tech.trim().toUpperCase();
  if (key === 'FTTO' || key === 'FIBRA DEDICATA') return styles.chipFtto ?? '';
  if (key === 'FTTH') return styles.chipFtth ?? '';
  if (key === 'XGSPON') return styles.chipXgspon ?? '';
  if (key === 'FTTC' || key === 'EVDSL' || key === 'VULA_EVDSL') return styles.chipFttc ?? '';
  if (key === 'VDSL' || key === 'VULA_VDSL') return styles.chipVdsl ?? '';
  if (key === 'FWA') return styles.chipFwa ?? '';
  if (key === 'SHDSL') return styles.chipShdsl ?? '';
  return styles.chipAdsl ?? '';
}

function distancePerfClass(perf: DistancePerf | null): string {
  if (!perf) return '';
  if (perf === 'ottimali') return styles.perfOttimali ?? '';
  if (perf === 'buone') return styles.perfBuone ?? '';
  if (perf === 'degradate') return styles.perfDegradate ?? '';
  return styles.perfInutilizzabili ?? '';
}

function statoChipClass(stato: string | null): string {
  if (!stato) return '';
  const v = stato.toLowerCase();
  if (v.includes('attivo') || v.includes('coperto')) return styles.statoOk ?? '';
  if (v.includes('passivo')) return styles.statoWarn ?? '';
  return styles.statoNeutral ?? '';
}

function formatMbps(value: number | null): string {
  if (value === null) return '—';
  if (value >= 100) return Math.round(value).toString();
  return value.toFixed(1).replace(/\.0$/, '');
}

function formatMeters(meters: number | null): string {
  if (meters === null) return '—';
  if (meters >= 1000) return `${(meters / 1000).toFixed(1)} km`;
  return `${Math.round(meters)} m`;
}

function OperatorLogo({
  result,
  size,
}: {
  result: CoverageResult;
  size: 'sm' | 'lg';
}) {
  const [failed, setFailed] = useState(false);
  const initials = operatorInitials(result.operator_name).toUpperCase() || 'OP';
  const wrapperClass = size === 'lg' ? styles.logoLg : styles.logoSm;

  if (result.logo_url && !failed) {
    return (
      <img
        className={wrapperClass}
        src={result.logo_url}
        alt=""
        aria-hidden="true"
        onError={() => setFailed(true)}
      />
    );
  }
  return <span className={`${wrapperClass} ${styles.logoFallback}`}>{initials}</span>;
}

function SpeedBar({
  label,
  mbps,
  tone,
}: {
  label: string;
  mbps: number | null;
  tone: 'primary' | 'secondary';
}) {
  const pct = mbps === null ? 0 : Math.min(100, (mbps / BAR_MAX_MBPS) * 100);
  return (
    <div className={styles.speedRow}>
      <div className={styles.speedMeta}>
        <span className={styles.speedLabel}>{label}</span>
        <span className={styles.speedValue}>
          {formatMbps(mbps)} <span className={styles.speedUnit}>Mbps</span>
        </span>
      </div>
      <div className={styles.speedTrack}>
        <span
          className={`${styles.speedFill} ${tone === 'primary' ? styles.speedPrimary : styles.speedSecondary}`}
          style={{ ['--bar-width' as string]: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function AllProfiles({ result, compact }: { result: CoverageResult; compact?: boolean }) {
  if (result.profiles.length <= 1) return null;
  return (
    <details className={`${styles.cardProfiles} ${compact ? styles.cardProfilesCompact : ''}`}>
      <summary className={styles.profilesSummary}>
        Tutti i profili ({result.profiles.length})
      </summary>
      <ul className={styles.profilesList}>
        {result.profiles.map((profile, i) => (
          <li key={`${result.coverage_id}-p-${i}`}>{profile.name}</li>
        ))}
      </ul>
    </details>
  );
}

function HeroBadges({ metrics }: { metrics: RankedCoverage['metrics'] }) {
  return (
    <div className={styles.heroFooter}>
      {metrics.stato && (
        <span className={`${styles.heroBadge} ${statoChipClass(metrics.stato)}`}>
          <Icon name="wifi" size={14} />
          Stato: {metrics.stato}
        </span>
      )}
      {metrics.fascia && (
        <span className={styles.heroBadge}>Fascia {metrics.fascia}</span>
      )}
      {metrics.distanza !== null && (
        <span className={styles.heroBadge}>
          Armadio a {formatMeters(metrics.distanza)}
        </span>
      )}
      {metrics.distancePerf && (
        <span className={`${styles.heroBadge} ${distancePerfClass(metrics.distancePerf)}`}>
          {DISTANCE_LABEL[metrics.distancePerf]}
        </span>
      )}
      {metrics.statusNote && (
        <span className={`${styles.heroBadge} ${styles.badgeWarn}`}>{metrics.statusNote}</span>
      )}
    </div>
  );
}

function PremiumItem({ ranked }: { ranked: RankedCoverage }) {
  const { result, metrics } = ranked;
  const profileName = metrics.selectedProfile?.profile.name;
  return (
    <div className={styles.premiumItem}>
      <div className={styles.premiumHeader}>
        <OperatorLogo result={result} size="lg" />
        <div className={styles.premiumHeaderText}>
          <span className={styles.premiumOperator}>{result.operator_name}</span>
          <div className={styles.cardHeaderMeta}>
            <span className={`${styles.chip} ${techChipClass(result.tech)}`}>{displayTech(result.tech)}</span>
            {profileName && <span className={styles.heroProfile}>{profileName}</span>}
          </div>
        </div>
      </div>
      <div className={styles.premiumSpeed}>
        <span className={styles.premiumSpeedNumber}>
          {formatMbps(metrics.maxDown)}
          <span className={styles.heroSpeedSlash}> / </span>
          {formatMbps(metrics.maxUp)}
        </span>
        <span className={styles.heroSpeedUnit}>Mbps &nbsp;·&nbsp; Down / Up</span>
      </div>
      <HeroBadges metrics={metrics} />
      <AllProfiles result={result} compact />
    </div>
  );
}

function HeroGroup({ premium }: { premium: RankedCoverage[] }) {
  return (
    <section className={styles.hero} aria-label="Offerte premium equivalenti">
      <div className={styles.heroGlow} aria-hidden="true" />
      <div className={styles.heroHeader}>
        <span className={styles.heroEyebrow}>Fibra dedicata premium · Offerte equivalenti</span>
      </div>
      <div className={styles.premiumGrid}>
        {premium.map((r, i) => (
          <PremiumItem key={`${r.result.coverage_id}-prem-${i}`} ranked={r} />
        ))}
      </div>
    </section>
  );
}

function HeroCard({ ranked }: { ranked: RankedCoverage }) {
  const { result, metrics } = ranked;
  const profileName = metrics.selectedProfile?.profile.name;
  return (
    <section className={styles.hero} aria-label="Miglior profilo disponibile">
      <div className={styles.heroGlow} aria-hidden="true" />
      <div className={styles.heroHeader}>
        <span className={styles.heroEyebrow}>{metrics.tierLabel} · Miglior scelta</span>
      </div>
      <div className={styles.heroBody}>
        <div className={styles.heroOperator}>
          <OperatorLogo result={result} size="lg" />
          <div className={styles.heroOperatorText}>
            <span className={styles.heroOperatorName}>{result.operator_name}</span>
            <span className={`${styles.chip} ${techChipClass(result.tech)}`}>{displayTech(result.tech)}</span>
            {profileName && <span className={styles.heroProfile}>{profileName}</span>}
          </div>
        </div>
        <div className={styles.heroSpeed}>
          <span className={styles.heroSpeedNumber}>
            {formatMbps(metrics.maxDown)}
            <span className={styles.heroSpeedSlash}> / </span>
            {formatMbps(metrics.maxUp)}
          </span>
          <span className={styles.heroSpeedUnit}>Mbps &nbsp;·&nbsp; Down / Up</span>
        </div>
      </div>
      <HeroBadges metrics={metrics} />
      <AllProfiles result={result} />
    </section>
  );
}

function OperatorCard({
  ranked,
  index,
  variant,
}: {
  ranked: RankedCoverage;
  index: number;
  variant: 'alternative' | 'unavailable';
}) {
  const { result, metrics } = ranked;
  const delay = Math.min(index * 60, 360);
  const unavailable = variant === 'unavailable';
  return (
    <article
      className={`${styles.card} ${unavailable ? styles.cardUnavailable : ''}`}
      style={{ animationDelay: `${delay}ms` }}
    >
      <header className={styles.cardHeader}>
        <OperatorLogo result={result} size="sm" />
        <div className={styles.cardHeaderText}>
          <span className={styles.cardOperator}>{result.operator_name}</span>
          <div className={styles.cardHeaderMeta}>
            <span className={`${styles.chip} ${techChipClass(result.tech)}`}>{displayTech(result.tech)}</span>
            <span className={styles.cardTier}>{metrics.tierLabel}</span>
          </div>
        </div>
      </header>

      {unavailable && metrics.sellabilityNote && (
        <div className={styles.unavailableNotice}>{metrics.sellabilityNote}</div>
      )}

      {!unavailable && (
        <>
          <div className={styles.cardSpeeds}>
            <SpeedBar label="Download" mbps={metrics.maxDown} tone="primary" />
            <SpeedBar label="Upload" mbps={metrics.maxUp} tone="secondary" />
          </div>

          {metrics.selectedProfile && (
            <div className={styles.cardProfile}>
              <span className={styles.profilesEyebrow}>Profilo consigliato</span>
              <span className={styles.cardProfileName}>
                {metrics.selectedProfile.profile.name}
              </span>
            </div>
          )}
        </>
      )}

      <ul className={styles.cardMeta}>
        {metrics.distancePerf && (
          <li>
            <span className={styles.metaKey}>Prestazioni</span>
            <span className={`${styles.metaValue} ${distancePerfClass(metrics.distancePerf)}`}>
              {DISTANCE_LABEL[metrics.distancePerf]}
            </span>
          </li>
        )}
        {metrics.stato && (
          <li>
            <span className={styles.metaKey}>Stato</span>
            <span className={`${styles.metaValue} ${statoChipClass(metrics.stato)}`}>
              {metrics.stato}
            </span>
          </li>
        )}
        {metrics.fascia && (
          <li>
            <span className={styles.metaKey}>Fascia</span>
            <span className={styles.metaValue}>{metrics.fascia}</span>
          </li>
        )}
        {metrics.distanza !== null && (
          <li>
            <span className={styles.metaKey}>Armadio</span>
            <span className={styles.metaValue}>{formatMeters(metrics.distanza)}</span>
          </li>
        )}
      </ul>

      {metrics.statusNote && !unavailable && (
        <div className={styles.cardNote}>{metrics.statusNote}</div>
      )}

      {!unavailable && <AllProfiles result={result} />}
    </article>
  );
}

function ProgressRail({ step }: { step: 0 | 1 | 2 | 3 | 4 }) {
  const labels = ['Provincia', 'Comune', 'Indirizzo', 'Civico'];
  return (
    <div className={styles.rail} role="progressbar" aria-valuemin={0} aria-valuemax={4} aria-valuenow={step}>
      {labels.map((label, i) => {
        const state = i < step ? 'done' : i === step ? 'active' : 'pending';
        return (
          <div key={label} className={styles.railCell}>
            <span
              className={`${styles.railSegment} ${
                state === 'done'
                  ? styles.railDone
                  : state === 'active'
                  ? styles.railActive
                  : styles.railPending
              }`}
            />
            <span className={`${styles.railLabel} ${state !== 'pending' ? styles.railLabelActive : ''}`}>
              {label}
            </span>
          </div>
        );
      })}
    </div>
  );
}

export function CoverageLookupPageV2() {
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

  function handleEditAddress() {
    setSubmitted(null);
  }

  const collapsed = submitted !== null;
  const step: 0 | 1 | 2 | 3 | 4 =
    houseNumberId !== null ? 4 : addressId !== null ? 3 : cityId !== null ? 2 : stateId !== null ? 1 : 0;

  const results: CoverageResult[] = coverageQ.data ?? [];
  const ranked = rankCoverage(results);
  const sellable = ranked.filter((r) => r.metrics.sellable);
  const unavailable = ranked.filter((r) => !r.metrics.sellable);

  const EQUIVALENCE_DELTA = 5;
  const premium = sellable.filter((r) => r.metrics.tier === 'premium_dedicated');
  const obsolete = sellable.filter((r) => r.metrics.tier === 'last_resort');
  const intermediate = sellable.filter(
    (r) => r.metrics.tier !== 'premium_dedicated' && r.metrics.tier !== 'last_resort',
  );

  const topPremiumScore = premium[0]?.metrics.score ?? -Infinity;
  const premiumTop = premium.filter(
    (r) => topPremiumScore - r.metrics.score <= EQUIVALENCE_DELTA,
  );
  const premiumBelow = premium.filter(
    (r) => topPremiumScore - r.metrics.score > EQUIVALENCE_DELTA,
  );

  let hero: RankedCoverage | null = null;
  let heroGroup: RankedCoverage[] | null = null;
  let alternatives: RankedCoverage[] = [];
  let obsoleteToShow: RankedCoverage[] = obsolete;

  if (premiumTop.length >= 2) {
    heroGroup = premiumTop;
    alternatives = [...premiumBelow, ...intermediate];
  } else if (premiumTop.length === 1) {
    hero = premiumTop[0] ?? null;
    alternatives = [...premiumBelow, ...intermediate];
  } else if (intermediate.length > 0) {
    hero = intermediate[0] ?? null;
    alternatives = intermediate.slice(1);
  } else if (obsolete.length > 0) {
    hero = obsolete[0] ?? null;
    alternatives = obsolete.slice(1);
    obsoleteToShow = [];
  }

  const submittedAddress = submitted?.labels.filter(Boolean).join(' · ') ?? '';

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Ricerca copertura</h1>

      {!collapsed && (
        <div className={styles.searchArea}>
          <ProgressRail step={step} />

          <div className={styles.toolbar}>
            <div className={`${styles.field} ${styles.fieldWide}`}>
              <label>Provincia</label>
              <SingleSelect
                options={stateOptions.map((option) => ({ value: option.id, label: option.name }))}
                selected={stateId}
                onChange={handleStateChange}
                placeholder="Seleziona provincia..."
              />
            </div>

            <div
              className={`${styles.field} ${styles.fieldWide} ${stateId === null ? styles.selectDisabled : ''}`}
            >
              <label>Comune</label>
              <SingleSelect
                options={cityOptions.map((option) => ({ value: option.id, label: option.name }))}
                selected={cityId}
                onChange={handleCityChange}
                placeholder="Seleziona comune..."
              />
            </div>

            <div
              className={`${styles.field} ${styles.fieldWide} ${cityId === null ? styles.selectDisabled : ''}`}
            >
              <label>Indirizzo</label>
              <SingleSelect
                options={addressOptions.map((option) => ({ value: option.id, label: option.name }))}
                selected={addressId}
                onChange={handleAddressChange}
                placeholder="Seleziona indirizzo..."
              />
            </div>

            <div
              className={`${styles.field} ${styles.fieldNarrow} ${addressId === null ? styles.selectDisabled : ''}`}
            >
              <label>Numero civico</label>
              <SingleSelect
                options={houseNumberOptions.map((option) => ({ value: option.id, label: option.name }))}
                selected={houseNumberId}
                onChange={handleHouseNumberChange}
                placeholder="Seleziona civico..."
              />
            </div>

            <button
              className={styles.btnSearch}
              onClick={handleSearch}
              disabled={houseNumberId === null}
            >
              <Icon name="search" size={16} />
              Cerca
            </button>
            <button className={styles.btnReset} onClick={handleReset}>
              Reimposta
            </button>
          </div>
        </div>
      )}

      {collapsed && (
        <div className={styles.breadcrumb}>
          <div className={styles.breadcrumbBody}>
            <span className={styles.breadcrumbEyebrow}>Indirizzo</span>
            <span className={styles.breadcrumbValue}>{submittedAddress}</span>
          </div>
          <button
            type="button"
            className={styles.breadcrumbEdit}
            onClick={handleEditAddress}
            aria-label="Modifica indirizzo"
          >
            <Icon name="pencil" size={14} />
            Modifica
          </button>
        </div>
      )}

      {liveErrors?.status === 503 && <ServiceUnavailable service="Coperture" />}
      {liveErrors !== null && liveErrors.status !== 503 && (
        <div className={styles.error}>Errore nel caricamento dei dati. Riprova.</div>
      )}

      {!submitted && liveErrors === null && (
        <div className={styles.emptyState}>
          <span className={styles.emptyKicker}>Pronto</span>
          <p className={styles.emptyTitle}>Seleziona un indirizzo completo per vedere le coperture.</p>
          <p className={styles.emptyHint}>Operatori, tecnologie e velocità massime disponibili in tempo reale.</p>
        </div>
      )}

      {submitted && coverageQ.isLoading && liveErrors === null && (
        <div className={styles.loading}>
          <Skeleton rows={4} />
        </div>
      )}

      {submitted && !coverageQ.isLoading && liveErrors === null && ranked.length > 0 && (
        <div className={styles.results}>
          {heroGroup && <HeroGroup premium={heroGroup} />}
          {!heroGroup && hero && <HeroCard ranked={hero} />}

          {alternatives.length > 0 && (
            <div className={styles.alternatives}>
              <div className={styles.alternativesHeader}>
                <span className={styles.alternativesTitle}>Altri profili disponibili</span>
                <span className={styles.alternativesCount}>
                  {alternatives.length} alternativ{alternatives.length === 1 ? 'a' : 'e'}
                </span>
              </div>
              <div className={styles.grid}>
                {alternatives.map((r, i) => (
                  <OperatorCard
                    key={`${r.result.coverage_id}-${i}`}
                    ranked={r}
                    index={i}
                    variant="alternative"
                  />
                ))}
              </div>
            </div>
          )}

          {obsoleteToShow.length > 0 && (
            <div className={`${styles.alternatives} ${styles.obsoleteSection}`}>
              <div className={styles.alternativesHeader}>
                <span className={styles.alternativesTitle}>Tecnologie obsolete</span>
                <span className={styles.alternativesCount}>
                  {obsoleteToShow.length} voc{obsoleteToShow.length === 1 ? 'e' : 'i'}
                </span>
              </div>
              <div className={styles.grid}>
                {obsoleteToShow.map((r, i) => (
                  <OperatorCard
                    key={`${r.result.coverage_id}-obs-${i}`}
                    ranked={r}
                    index={i}
                    variant="alternative"
                  />
                ))}
              </div>
            </div>
          )}

          {unavailable.length > 0 && (
            <div className={styles.alternatives}>
              <div className={styles.alternativesHeader}>
                <span className={styles.alternativesTitle}>Coperture non vendibili</span>
                <span className={styles.alternativesCount}>
                  {unavailable.length} voc{unavailable.length === 1 ? 'e' : 'i'}
                </span>
              </div>
              <div className={styles.grid}>
                {unavailable.map((r, i) => (
                  <OperatorCard
                    key={`${r.result.coverage_id}-unavail-${i}`}
                    ranked={r}
                    index={i}
                    variant="unavailable"
                  />
                ))}
              </div>
            </div>
          )}

          {!hero && !heroGroup && unavailable.length === 0 && (
            <div className={styles.emptyState}>
              <span className={styles.emptyKicker}>Nessun profilo vendibile</span>
              <p className={styles.emptyTitle}>Nessuna copertura commercializzabile per questo civico.</p>
            </div>
          )}
        </div>
      )}

      {submitted && !coverageQ.isLoading && liveErrors === null && ranked.length === 0 && (
        <div className={styles.emptyState}>
          <span className={styles.emptyKicker}>Nessun risultato</span>
          <p className={styles.emptyTitle}>Nessuna copertura disponibile per questo civico.</p>
        </div>
      )}
    </div>
  );
}
