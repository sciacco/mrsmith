import { ApiError } from '@mrsmith/api-client';
import { Icon, Skeleton } from '@mrsmith/ui';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type { MASessionDetail, MATarget, MATargetBucket, MAConfidence } from '../../api/types';
import { HiddenField } from './inspector/HiddenField';
import { SintesiTab } from './inspector/tabs/SintesiTab';
import { TesiTab } from './inspector/tabs/TesiTab';
import { ProvenienzaTab } from './inspector/tabs/ProvenienzaTab';
import { PunteggioTab } from './inspector/tabs/PunteggioTab';
import { WebValidationTab } from './inspector/tabs/WebValidationTab';
import { DeepTab } from './inspector/tabs/DeepTab';
import { RegistroTab } from './inspector/tabs/RegistroTab';
import { JsonTab } from './inspector/tabs/JsonTab';
import styles from './inspector/Inspector.module.css';

type TabKey = 'sintesi' | 'punteggio' | 'web' | 'deep' | 'tesi' | 'provenienza' | 'registro' | 'json';

const TABS: { key: TabKey; num: string; label: string }[] = [
  { key: 'sintesi', num: 'T1', label: 'Sintesi' },
  { key: 'punteggio', num: 'T2', label: 'Punteggio' },
  { key: 'web', num: 'T3', label: 'Web validation' },
  { key: 'deep', num: 'T4', label: 'Deep-dive' },
  { key: 'tesi', num: 'T5', label: 'Tesi' },
  { key: 'provenienza', num: 'T6', label: 'Provenienza' },
  { key: 'registro', num: 'T7', label: 'Registro & rating' },
  { key: 'json', num: 'T8', label: 'JSON' },
];

function bucketLabel(bucket?: MATargetBucket): string | null {
  switch (bucket) {
    case 'principale': return 'In tesi';
    case 'da_verificare': return 'Da verificare';
    case 'azionabile': return 'Azionabile';
    case 'soppresso': return 'Soppresso';
    default: return null;
  }
}

function bucketClass(bucket?: MATargetBucket): string {
  switch (bucket) {
    case 'principale': return styles.pillPrincipale ?? '';
    case 'da_verificare': return styles.pillDaVerificare ?? '';
    case 'azionabile': return styles.pillAzionabile ?? '';
    case 'soppresso': return styles.pillSoppresso ?? '';
    default: return styles.pillNeutral ?? '';
  }
}

function confidenceLabel(c?: MAConfidence): string | null {
  switch (c) {
    case 'alta': return 'Confidence alta';
    case 'media': return 'Confidence media';
    case 'bassa': return 'Confidence bassa';
    default: return null;
  }
}

function confidenceClass(c?: MAConfidence): string {
  switch (c) {
    case 'alta': return styles.pillConfHigh ?? '';
    case 'media': return styles.pillConfMed ?? '';
    case 'bassa': return styles.pillConfLow ?? '';
    default: return styles.pillNeutral ?? '';
  }
}

function isNotFound(err: unknown): boolean {
  return err instanceof ApiError && err.status === 404;
}

export function TargetInspectorPage() {
  const { id, targetId } = useParams<{ id: string; targetId: string }>();
  const api = useApiClient();
  const [tab, setTab] = useState<TabKey>('sintesi');

  const targetQuery = useQuery({
    queryKey: ['ma-target-inspector', id, targetId],
    enabled: Boolean(id && targetId),
    queryFn: () => api.get<MATarget>(`/binocolo/v1/ma/sessions/${id!}/targets/${targetId!}`),
    retry: (failureCount, error) => !(isNotFound(error)) && failureCount < 2,
  });

  // Titolo sessione non presente nel MATarget: fetch lean separata (vincolo frontend-only).
  const sessionQuery = useQuery({
    queryKey: ['ma-session-lean', id],
    enabled: Boolean(id),
    queryFn: () => api.get<MASessionDetail>(`/binocolo/v1/ma/sessions/${id!}?targets=none`),
    retry: (failureCount, error) => !(isNotFound(error)) && failureCount < 2,
  });

  // Polling deep (F5 stato B): invalida la query target ogni 5s finché deep.status
  // resta queued/running. Si ferma a ready/failed/nil. Pattern di RicercaDetailPage/
  // IniziativaBoardPage. refetchType 'active' + refetch forzato.
  const deepStatus = targetQuery.data?.deep?.status;
  const deepRunning = deepStatus === 'queued' || deepStatus === 'running';
  useEffect(() => {
    if (!deepRunning) return;
    const handle = setInterval(() => {
      void targetQuery.refetch();
    }, 5000);
    return () => clearInterval(handle);
  }, [deepRunning, targetQuery]);

  if (!id || !targetId) {
    return (
      <main className={styles.page}>
        <div className={styles.danger} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>Ricerca o target non indicati.</span>
        </div>
      </main>
    );
  }

  if (targetQuery.isLoading) {
    return (
      <main className={styles.page}>
        <Skeleton rows={6} />
      </main>
    );
  }

  if (targetQuery.isError) {
    const notFound = isNotFound(targetQuery.error);
    return (
      <main className={styles.page}>
        <div className={styles.danger} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>{notFound ? 'Target non trovato.' : (targetQuery.error instanceof Error ? targetQuery.error.message : 'Errore di caricamento.')}</span>
        </div>
        <Link className={styles.backLink} to={`/ricerche/${id}`}>← Torna alla ricerca</Link>
      </main>
    );
  }

  const target = targetQuery.data!;
  const session = sessionQuery.data?.session;
  const sessionTitle = session?.title ?? 'Ricerca';
  const initiativeId = session?.initiativeId;
  const strategyVersionId = sessionQuery.data?.strategy?.id;

  const bkt = bucketLabel(target.bucket);
  const conf = confidenceLabel(target.confidence);
  const scoreHidden = target.bucket === 'soppresso' || !target.matchState;

  return (
    <main className={styles.page}>
      {/* app-head */}
      <header className={styles.header}>
        <span className={styles.logo}>B</span>
        <span className={styles.brand}>Binocolo</span>
        <Link className={styles.backLink} to={`/ricerche/${id}`}>
          ← <b>Ricerca · {sessionTitle}</b>
        </Link>
      </header>

      {/* banner modalità ispezione (viola) */}
      <div className={styles.banner} role="note">
        <span className={styles.ico}>!</span>
        <span>
          <b>Modalità ispezione.</b> Superficie di scoperta, non analista: dati grezzi e tecnici visibili. Read-only.
        </span>
        <a
          className={styles.bannerLink}
          href={typeof window !== 'undefined' ? window.location.href : '#'}
          target="_blank"
          rel="noopener noreferrer"
        >
          Apri in nuova tab <Icon name="external-link" size={14} />
        </a>
      </div>

      {/* identità target + score */}
      <div className={styles.ident}>
        <div className={styles.identMain}>
          <h1>
            {target.companyName} <HiddenField label="activityStatus" />
          </h1>
          <div className={styles.identMeta}>
            {target.vatCode ? <span>P.IVA <b>{target.vatCode}</b></span> : null}
            {target.taxCode ? <><span className={styles.sep}>·</span><span>C.F. <b>{target.taxCode}</b></span></> : null}
            {target.town || target.province ? (
              <><span className={styles.sep}>·</span><span>{[target.town, target.province].filter(Boolean).join(' (')}{target.town && target.province ? ')' : ''}</span></>
            ) : null}
            {target.atecoCode ? (
              <><span className={styles.sep}>·</span><span>ATECO <b>{target.atecoCode}</b>{target.atecoDescription ? ` — ${target.atecoDescription}` : ''}</span></>
            ) : null}
          </div>
          <div className={styles.identPills}>
            {bkt ? <span className={`${styles.pill} ${bucketClass(target.bucket)}`}>{bkt}</span> : null}
            {conf ? (
              <span className={`${styles.pill} ${confidenceClass(target.confidence)}`}>
                {conf} <HiddenField label="confidence" />
              </span>
            ) : null}
            <span className={`${styles.pill} ${styles.pillNeutral}`}>
              {target.matchState} <HiddenField label="matchState" />
            </span>
            {target.enrichmentLevel ? (
              <span className={`${styles.pill} ${styles.pillNeutral}`}>
                {target.enrichmentLevel} <HiddenField label="enrichmentLevel" />
              </span>
            ) : null}
          </div>
        </div>
        <div className={styles.scoreBlock}>
          {scoreHidden ? (
            <span className={styles.scoreAbsent}>—</span>
          ) : (
            <span className={styles.scoreN}>{target.score}</span>
          )}
          <span className={styles.scoreSub}>
            score{target.scoreVersion ? ` · v${target.scoreVersion}` : ''} <HiddenField label="scoreVersion" />
          </span>
        </div>
      </div>

      {/* tab bar */}
      <div className={styles.tabs} role="tablist" aria-label="Sezioni ispezione">
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={tab === t.key}
            className={`${styles.tab} ${tab === t.key ? styles.tabActive : ''}`}
            onClick={() => setTab(t.key)}
          >
            <span className={styles.tabNum}>{t.num}</span>
            {t.label}
          </button>
        ))}
      </div>

      {/* body — i tab vengono riempiti nelle fasi successive (F2–F7).
          F2 implementa T1 Sintesi, T5 Tesi, T6 Provenienza. */}
      {tab === 'sintesi' ? (
        <SintesiTab target={target} initiativeId={initiativeId} />
      ) : tab === 'web' ? (
        <WebValidationTab target={target} />
      ) : tab === 'deep' ? (
        <DeepTab
          deep={target.deep}
          initiativeId={initiativeId}
          companyKey={target.companyKey}
        />
      ) : tab === 'punteggio' ? (
        <PunteggioTab target={target} />
      ) : tab === 'json' ? (
        <JsonTab target={target} />
      ) : tab === 'registro' ? (
        <RegistroTab target={target} initiativeId={initiativeId} />
      ) : tab === 'tesi' ? (
        <TesiTab target={target} initiativeId={initiativeId} />
      ) : tab === 'provenienza' ? (
        <ProvenienzaTab target={target} strategyVersionId={strategyVersionId} />
      ) : (
        <div className={styles.placeholder} role="tabpanel">
          [… corpo del tab {TABS.find((t) => t.key === tab)?.num} — {TABS.find((t) => t.key === tab)?.label} …]
        </div>
      )}
    </main>
  );
}
