import { ApiError } from '@mrsmith/api-client';
import { Icon } from '@mrsmith/ui';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { useApiClient } from '../../../../api/client';
import { ThesisReadingPanel } from '../../../../components/ThesisReadingPanel/ThesisReadingPanel';
import type { MASessionThesisReading, MATarget, MATargetAdjustment } from '../../../../api/types';
import { HiddenField } from '../HiddenField';
import styles from '../Inspector.module.css';
import { dossierHref } from './format';

export function TesiTab({
  target,
  sessionId,
  targetId,
  initiativeId,
  initiativeTitle,
}: {
  target: MATarget;
  sessionId?: string;
  targetId?: string;
  initiativeId?: string;
  initiativeTitle?: string;
}) {
  const api = useApiClient();
  const thesisFit = (target.adjustments ?? []).find((a: MATargetAdjustment) => a.code === 'thesis-fit');
  const legacyReading = target.deep?.brief?.thesisReading?.trim();
  const dossier = dossierHref({ initiativeId, companyKey: target.companyKey });
  const effectiveSessionId = sessionId || target.sessionId;
  const effectiveTargetId = targetId || target.id;
  const deepReady = target.deep?.status === 'ready';
  const canGenerateThesis = deepReady && Boolean(effectiveSessionId && effectiveTargetId);

  const thesisQuery = useQuery({
    queryKey: ['ma-session-thesis-reading', effectiveSessionId, effectiveTargetId],
    enabled: Boolean(effectiveSessionId && effectiveTargetId),
    queryFn: () => {
      if (!effectiveSessionId || !effectiveTargetId) throw new Error('Target non disponibile.');
      return api.get<MASessionThesisReading>(
        `/binocolo/v1/ma/sessions/${effectiveSessionId}/targets/${effectiveTargetId}/thesis-reading`,
      );
    },
    retry: (failureCount, error) => !(error instanceof ApiError && error.status === 404) && failureCount < 2,
  });
  const generate = useMutation({
    mutationFn: () => {
      if (!effectiveSessionId || !effectiveTargetId) throw new Error('Target non disponibile.');
      return api.post<MASessionThesisReading>(
        `/binocolo/v1/ma/sessions/${effectiveSessionId}/targets/${effectiveTargetId}/thesis-reading`,
        {},
      );
    },
    onSuccess: () => void thesisQuery.refetch(),
  });
  const notGenerated = thesisQuery.isError && thesisQuery.error instanceof ApiError && thesisQuery.error.status === 404;
  const queryError = thesisQuery.isError && !notGenerated
    ? thesisQuery.error instanceof ApiError ? `Lettura di tesi non caricata (${thesisQuery.error.status}).` : 'Lettura di tesi non caricata.'
    : null;
  const generationError = generate.isError
    ? generate.error instanceof ApiError ? `Generazione non riuscita (${generate.error.status}).` : 'Generazione non riuscita: riprova.'
    : null;

  return (
    <div className={styles.tabBody}>
      <ThesisReadingPanel
        record={thesisQuery.data}
        loading={thesisQuery.isLoading}
        notGenerated={notGenerated}
        queryError={queryError}
        deepReady={canGenerateThesis}
        onGenerate={() => generate.mutate()}
        generating={generate.isPending}
        generationError={generationError}
        showBlockedAction={false}
        readyText="Genera il memo che applica la tesi della ricerca corrente ai fatti dell'analisi azienda."
        blockedText="Serve prima l'analisi approfondita pronta: la lettura di tesi si appoggia ai fatti del dossier."
      />

      <div className={styles.caveat}>
        <span className={styles.tcIco}>i</span>
        <div>
          <b>La lettura di tesi segue la ricerca corrente.</b> Sotto resta visibile il fattore tecnico
          thesis-fit applicato allo score in questa sessione.
        </div>
      </div>

      <div className={styles.card}>
        <p className={styles.lab}>
          Fattore thesis-fit (in questa sessione) <HiddenField label="adjustments[thesis-fit]" />
        </p>
        {thesisFit ? (
          <div className={styles.rowsList}>
            <div className={`${styles.rowline} ${styles.rowlineNeutral}`}>
              <div>
                <div className={styles.rlMain}>{thesisFit.label || thesisFit.code}</div>
                <div className={styles.rlSub}>Fattore applicato allo score in questa sessione.</div>
              </div>
              <span className={styles.rlVal}>× {thesisFit.factor.toLocaleString('it-IT', { maximumFractionDigits: 2 })}</span>
            </div>
          </div>
        ) : (
          <p className={styles.cardSub} style={{ margin: 0 }}>
            Nessun fattore thesis-fit registrato per questo target.
          </p>
        )}
      </div>

      <div className={styles.card}>
        <p className={styles.lab}>
          Lettura legacy (pre-v3) <HiddenField label="deep.brief.thesisReading" />
        </p>
        {legacyReading ? (
          <p className={styles.rlMain} style={{ fontWeight: 400, color: 'var(--color-text-secondary)' }}>{legacyReading}</p>
        ) : (
          <p className={styles.muted} style={{ fontStyle: 'italic', margin: 0 }}>— vuota —</p>
        )}
        <p className={styles.cardSub} style={{ margin: '6px 0 0' }}>
          Le run v3 non popolano questo campo; il memo operativo è la lettura di tesi sopra.
        </p>
      </div>

      {dossier ? (
        <Link className={styles.dossierLink} to={dossier}>
          Apri dossier <Icon name="external-link" size={14} />
          <small>dossier iniziativa{initiativeTitle ? ` · «${initiativeTitle}»` : ''}</small>
        </Link>
      ) : null}
    </div>
  );
}
