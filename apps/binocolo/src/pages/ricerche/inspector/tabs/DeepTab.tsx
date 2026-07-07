import { Button, Icon } from '@mrsmith/ui';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useApiClient } from '../../../../api/client';
import type { MADeepAnalysis } from '../../../../api/types';
import { DeepAnalysisContent } from '../../../../components/deep/DeepComponents';
import styles from '../Inspector.module.css';
import { dossierHref, formatDateTime } from './format';

type LaunchStatus = Extract<MADeepAnalysis['status'], 'queued' | 'running' | 'ready'>;

function launchErrorLabel(err: unknown): string {
  return err instanceof Error ? err.message : 'Analisi non avviata. Riprova tra poco.';
}

export function DeepTab({
  deep,
  initiativeId,
  initiativeTitle,
  companyKey,
  optimisticStatus,
  onAnalysisStarted,
}: {
  deep?: MADeepAnalysis;
  initiativeId?: string;
  initiativeTitle?: string;
  companyKey?: string;
  optimisticStatus?: LaunchStatus | null;
  onAnalysisStarted?: (status: MADeepAnalysis['status']) => void | Promise<unknown>;
}) {
  const api = useApiClient();
  const dossier = dossierHref({ initiativeId, companyKey });
  const [launching, setLaunching] = useState(false);
  const [launchError, setLaunchError] = useState<string | null>(null);
  const [launchStatus, setLaunchStatus] = useState<LaunchStatus | null>(null);

  useEffect(() => {
    if (!launchStatus) return;
    if (deep?.status === 'queued' || deep?.status === 'running' || deep?.status === 'ready') {
      setLaunchStatus(null);
    }
  }, [deep?.status, launchStatus]);

  async function launchAnalysis() {
    if (!companyKey) {
      setLaunchError("Chiave azienda non disponibile per avviare l'analisi.");
      return;
    }
    setLaunching(true);
    setLaunchError(null);
    try {
      const response = await api.post<{ status: MADeepAnalysis['status'] }>(
        `/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/deep-dive`,
        {},
      );
      if (response.status === 'failed') {
        setLaunchStatus(null);
        setLaunchError("Analisi non accodata. Riprova dopo aver verificato il target.");
      } else {
        setLaunchStatus(response.status);
      }
      void Promise.resolve(onAnalysisStarted?.(response.status)).catch(() => undefined);
    } catch (err) {
      setLaunchError(launchErrorLabel(err));
    } finally {
      setLaunching(false);
    }
  }

  const effectiveLaunchStatus = launchStatus ?? optimisticStatus ?? null;

  if (effectiveLaunchStatus && (!deep || deep.status === 'failed')) {
    return <DeepRunningState status={effectiveLaunchStatus} updatedAt={deep?.updatedAt} launched />;
  }

  // Stato D — analisi assente
  if (!deep) {
    return (
      <DeepEmptyNil
        initiativeId={initiativeId}
        initiativeTitle={initiativeTitle}
        companyKey={companyKey}
        launching={launching}
        launchError={launchError}
        onLaunch={launchAnalysis}
      />
    );
  }

  // Stato B — queued / running
  if (deep.status === 'queued' || deep.status === 'running') {
    return <DeepRunningState status={deep.status} updatedAt={deep.updatedAt} />;
  }

  // Stato C — failed
  if (deep.status === 'failed') {
    return (
      <div className={styles.tabBody}>
        <div className={styles.deepEmpty}>
          <span className={`${styles.deepEmptyPill} ${styles.deepEmptyFailed}`}>Analisi non riuscita</span>
          <p className={styles.deepEmptyText}>
            L'analisi è fallita in un'esecuzione precedente.
          </p>
          <span className={styles.deepEmptyMono}>Codice errore · {deep.errorCode ?? 'n/d'}</span>
          <span className={styles.deepEmptyMono}>Aggiornata il {formatDateTime(deep.updatedAt)}</span>
          <DeepLaunchAction
            companyKey={companyKey}
            launching={launching}
            launchError={launchError}
            onLaunch={launchAnalysis}
          />
          {dossier ? (
            <Link className={styles.dossierLink} to={dossier}>
              Apri dossier <Icon name="external-link" size={14} />
            </Link>
          ) : null}
        </div>
      </div>
    );
  }

  // Stato A — ready
  return (
    <div className={styles.tabBody}>
      <DeepAnalysisContent deep={deep} variant="full" showHiddenFields />

      <div className={styles.deepFooter}>
        <span>Analisi aggiornata il {formatDateTime(deep.updatedAt)}</span>
        {deep.errorCode ? <><span>·</span><span>Codice errore: {deep.errorCode}</span></> : null}
        {dossier ? (
          <Link className={styles.dossierLink} to={dossier} style={{ marginLeft: 'auto' }}>
            Apri dossier <Icon name="external-link" size={14} />
          </Link>
        ) : null}
      </div>
    </div>
  );
}

function DeepRunningState({ status, updatedAt, launched = false }: { status: LaunchStatus; updatedAt?: string; launched?: boolean }) {
  const isReady = status === 'ready';

  return (
    <div className={styles.deepEmpty}>
      <span className={`${styles.deepEmptyPill} ${styles.deepEmptyRun}`}>
        {!isReady ? <span className={styles.pulseDot} /> : null}
        {isReady ? 'Analisi disponibile' : status === 'queued' ? 'In coda' : 'Analisi in corso…'}
      </span>
      <p className={styles.deepEmptyText}>
        {isReady
          ? 'Analisi già disponibile. Aggiorno i dati.'
          : launched
            ? 'Analisi avviata. Lo stato si aggiorna automaticamente.'
            : "L'analisi è in produzione. Il risultato appare qui automaticamente quando è disponibile."}
      </p>
      {updatedAt ? <span className={styles.deepEmptyMono}>Aggiornata il {formatDateTime(updatedAt)}</span> : null}
    </div>
  );
}

function DeepLaunchAction({
  companyKey,
  launching,
  launchError,
  onLaunch,
}: {
  companyKey?: string;
  launching: boolean;
  launchError: string | null;
  onLaunch: () => void | Promise<void>;
}) {
  return (
    <>
      <Button onClick={() => void onLaunch()} loading={launching} disabled={!companyKey || launching}>
        Avvia analisi
      </Button>
      {!companyKey ? (
        <div className={styles.dangerBox} role="alert">
          <Icon name="triangle-alert" size={16} />
          <span>Chiave azienda non disponibile per avviare l'analisi.</span>
        </div>
      ) : null}
      {launchError ? (
        <div className={styles.dangerBox} role="alert">
          <Icon name="triangle-alert" size={16} />
          <span>{launchError}</span>
        </div>
      ) : null}
    </>
  );
}

// Stato D — nil: percorso ramificato (initiativeId → dossier; altrimenti → ricerche)
function DeepEmptyNil({
  initiativeId,
  initiativeTitle,
  companyKey,
  launching,
  launchError,
  onLaunch,
}: {
  initiativeId?: string;
  initiativeTitle?: string;
  companyKey?: string;
  launching: boolean;
  launchError: string | null;
  onLaunch: () => void | Promise<void>;
}) {
  const dossier = dossierHref({ initiativeId, companyKey });
  return (
    <div className={styles.deepEmpty}>
      <span className={`${styles.deepEmptyPill} ${styles.deepEmptyNa}`}>Nessuna analisi completa</span>
      <p className={styles.deepEmptyText}>
        Nessuna analisi completa per questa azienda.
      </p>
      <DeepLaunchAction companyKey={companyKey} launching={launching} launchError={launchError} onLaunch={onLaunch} />
      {dossier ? (
        <Link className={styles.dossierLink} to={dossier}>
          Apri dossier <Icon name="external-link" size={14} />
          {initiativeTitle ? <small>nell'iniziativa «{initiativeTitle}»</small> : null}
        </Link>
      ) : initiativeId ? (
        <Link className={styles.dossierLink} to={`/iniziative/${initiativeId}`}>
          Apri board iniziativa <Icon name="external-link" size={14} />
        </Link>
      ) : (
        <>
          <p className={styles.deepEmptyText} style={{ fontStyle: 'italic' }}>
            Associa la ricerca a un'iniziativa per collegarla al flusso di lavorazione.
          </p>
          <Link className={styles.dossierLink} to="/ricerche">
            Vai alle ricerche <Icon name="external-link" size={14} />
          </Link>
        </>
      )}
    </div>
  );
}
