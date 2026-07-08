import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Button, Icon, Skeleton } from '@mrsmith/ui';
import { useApiClient } from '../../api/client';
import { DeepAnalysisContent } from '../../components/deep/DeepComponents';
import { IRLPanel } from '../../components/company/IRLPanel';
import { ShareholdersDetail } from '../../components/company/ShareholdersDetail';
import { ThesisReadingPanel } from '../../components/ThesisReadingPanel/ThesisReadingPanel';
import { VendorFinancials } from '../../components/company/VendorFinancials';
import { WebVerificationDetail } from '../../components/company/WebVerificationDetail';
import { writeCohort } from '../../components/scheda/cohort';
import type {
  MACardDossier,
  MACardProvenance,
  MACardThesisReading,
  MADeepAnalysis,
  MATarget,
  MATargetAdjustment,
  MATargetEvidence,
  MATargetFlag,
} from '../../api/types';
import { CompanyRegistrySection } from './CompanyRegistrySection';
import styles from './IniziativaCardDossierPage.module.css';

const CARD_STATE_LABELS: Record<string, string> = {
  da_contattare: 'Da contattare',
  contattata: 'Contattata',
  in_dialogo: 'In dialogo',
  approfondimento: 'Approfondimento',
  offerta: 'Offerta',
  chiusa: 'Chiusa',
  rimossa: 'Rimossa',
};

function schedaAziendaHref(companyKey: string, initiativeId: string): string {
  return `/aziende/${encodeURIComponent(companyKey)}?iniziativa=${encodeURIComponent(initiativeId)}`;
}

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 404) return 'Nessun dettaglio disponibile per questa card.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    if (error.status === 503) return 'Servizio non configurato in questo ambiente.';
    return `Richiesta non riuscita (${error.status}).`;
  }
  return 'Richiesta non riuscita.';
}

function dateLabel(value?: string): string {
  if (!value) return '—';
  return new Intl.DateTimeFormat('it-IT').format(new Date(value));
}

function ratingStars(rating: number): string {
  return '★'.repeat(Math.max(0, Math.min(3, rating)));
}

function latestProvenance(provenances: MACardProvenance[] | undefined): MACardProvenance | undefined {
  return provenances?.[0];
}

type TabKey = 'overview' | 'deep' | 'tesi' | 'irl' | 'financials' | 'shareholders' | 'registry' | 'web';

const TABS: { key: TabKey; label: string; icon: string }[] = [
  { key: 'overview', label: 'Panoramica', icon: 'external-link' },
  { key: 'deep', label: 'Analisi', icon: 'bar-chart-2' },
  { key: 'tesi', label: 'Lettura di tesi', icon: 'target' },
  { key: 'irl', label: 'IRL', icon: 'clipboard-check' },
  { key: 'financials', label: 'Finanziari', icon: 'circle-dollar-sign' },
  { key: 'shareholders', label: 'Soci', icon: 'user' },
  { key: 'registry', label: 'Registro camerale', icon: 'file-text' },
  { key: 'web', label: 'Web', icon: 'network' },
];

export function IniziativaCardDossierPage() {
  const { id, companyKey } = useParams<{ id: string; companyKey: string }>();
  const navigate = useNavigate();
  const api = useApiClient();
  const [activeTab, setActiveTab] = useState<TabKey>('overview');

  const query = useQuery({
    queryKey: ['ma-card-dossier', id, companyKey],
    enabled: Boolean(id && companyKey),
    queryFn: () =>
      api.get<MACardDossier>(
        `/binocolo/v1/ma/initiatives/${id}/cards/${encodeURIComponent(companyKey ?? '')}/dossier`,
      ),
    // Il 404 (card assente o sessioni sganciate) è uno stato, non un guasto:
    // niente retry, l'empty state deve comparire subito.
    retry: (failureCount, error) =>
      !(error instanceof ApiError && error.status === 404) && failureCount < 2,
  });

  if (query.isLoading) {
    return (
      <main className={styles.page}>
        <Skeleton rows={8} />
      </main>
    );
  }

  if (query.isError) {
    return (
      <main className={styles.page}>
        <button type="button" className={styles.backLink} onClick={() => navigate(`/iniziative/${id}`)}>
          ← Torna al board
        </button>
        <div className={styles.statePanel} role="alert">
          <Icon name="triangle-alert" size={22} />
          <p>{errorLabel(query.error)}</p>
        </div>
      </main>
    );
  }

  const dossier = query.data;
  if (!dossier) return null;

  const target = dossier.target;
  const card = dossier.card;
  const latestJudgement = latestProvenance(dossier.provenances);

  return (
    <main className={styles.page}>
      <button type="button" className={styles.backLink} onClick={() => navigate(`/iniziative/${id}`)}>
        ← Torna al board
      </button>

      <header className={styles.header}>
        <div className={styles.headTop}>
          <h1>{target.companyName}</h1>
          {card ? (
            <span className={styles.statePill}>{CARD_STATE_LABELS[card.state] ?? card.state}</span>
          ) : null}
          <a
            className={styles.actionLink}
            href={schedaAziendaHref(target.companyKey ?? companyKey ?? '', id ?? '')}
            target="_blank"
            rel="noopener noreferrer"
            onClick={() =>
              writeCohort({
                lensType: 'iniziativa',
                lensId: id ?? '',
                companyKeys: [target.companyKey ?? companyKey ?? ''],
              })
            }
            onAuxClick={() =>
              writeCohort({
                lensType: 'iniziativa',
                lensId: id ?? '',
                companyKeys: [target.companyKey ?? companyKey ?? ''],
              })
            }
          >
            Apri scheda ↗
          </a>
        </div>
        <div className={styles.meta}>
          {target.vatCode ? <span className={styles.mono}>P.IVA {target.vatCode}</span> : null}
          {target.taxCode ? <span className={styles.mono}>CF {target.taxCode}</span> : null}
          {target.town ? <span>{[target.town, target.province].filter(Boolean).join(' · ')}</span> : null}
          {dossier.sessionTitle ? <span>Ricerca: {dossier.sessionTitle}</span> : null}
        </div>
        {latestJudgement ? (
          <div className={styles.judgementSummary}>
            <span className={styles.judgementStars}>{ratingStars(latestJudgement.rating)}</span>
            {latestJudgement.sessionTitle ? <span>Ricerca: {latestJudgement.sessionTitle}</span> : null}
            {latestJudgement.scoreAtRating != null ? <span>Score al giudizio: {latestJudgement.scoreAtRating}</span> : null}
            {latestJudgement.confidenceAtRating ? <span>Confidenza: {latestJudgement.confidenceAtRating}</span> : null}
            <span>{dateLabel(latestJudgement.ratedAt)}</span>
          </div>
        ) : null}
      </header>

      <div className={styles.section}>
        <div className={styles.tabNav}>
          {TABS.map((tab) => (
            <button
              key={tab.key}
              type="button"
              className={`${styles.tabLink ?? ''} ${activeTab === tab.key ? styles.tabLinkActive ?? '' : ''}`}
              onClick={() => setActiveTab(tab.key)}
            >
              <Icon name={tab.icon as any} size={16} />
              <span>{tab.label}</span>
            </button>
          ))}
        </div>

        {activeTab === 'overview' && <OverviewTab target={target} />}
        {activeTab === 'deep' && (
          <DeepAnalysisTab
            deep={target.deep}
            companyKey={target.companyKey ?? companyKey ?? ''}
            onStarted={() => void query.refetch()}
          />
        )}
        {activeTab === 'tesi' && (
          <ThesisReadingTab
            initiativeId={id ?? ''}
            companyKey={target.companyKey ?? companyKey ?? ''}
            deepReady={target.deep?.status === 'ready'}
          />
        )}
        {activeTab === 'irl' && (
          <IRLPanel initiativeId={id ?? ''} companyKey={target.companyKey ?? companyKey ?? ''} companyName={target.companyName} />
        )}
        {activeTab === 'financials' && <VendorFinancials target={target} />}
        {activeTab === 'shareholders' && <ShareholdersDetail target={target} />}
        {activeTab === 'registry' && <RegistryTab target={target} />}
        {activeTab === 'web' && <WebVerificationDetail target={target} />}
      </div>

      <CompanyRegistrySection companyKey={target.companyKey ?? companyKey ?? ''} vatCode={target.vatCode} companyName={target.companyName} />
    </main>
  );
}

// ThesisReadingTab — lettura di tesi context-scoped (Fase 5): il memo che applica
// la tesi della sessione di provenienza al dossier neutro. Generazione on-demand,
// rigenerazione esplicita quando la tesi cambia (staleThesis).
function ThesisReadingTab({
  initiativeId,
  companyKey,
  deepReady,
}: {
  initiativeId: string;
  companyKey: string;
  deepReady: boolean;
}) {
  const api = useApiClient();
  const query = useQuery({
    queryKey: ['ma-thesis-reading', initiativeId, companyKey],
    enabled: Boolean(initiativeId && companyKey),
    queryFn: () =>
      api.get<MACardThesisReading>(
        `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(companyKey)}/thesis-reading`,
      ),
    retry: (failureCount, error) =>
      !(error instanceof ApiError && error.status === 404) && failureCount < 2,
  });
  const generate = useMutation({
    mutationFn: () =>
      api.post<MACardThesisReading>(
        `/binocolo/v1/ma/initiatives/${initiativeId}/cards/${encodeURIComponent(companyKey)}/thesis-reading`,
        {},
      ),
    onSuccess: () => void query.refetch(),
  });

  if (query.isLoading) return <Skeleton rows={5} />;

  const record = query.data;
  const notGenerated = !record && query.isError && query.error instanceof ApiError && query.error.status === 404;

  return (
    <ThesisReadingPanel
      record={record}
      notGenerated={notGenerated}
      queryError={query.isError && !notGenerated ? errorLabel(query.error) : null}
      deepReady={deepReady}
      onGenerate={() => generate.mutate()}
      generating={generate.isPending}
      generationError={generate.isError ? 'Generazione non riuscita: riprova.' : null}
      regenerateRequiresDeepReady={false}
    />
  );
}

function EmptyState({ icon, title, text }: { icon: string; title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <Icon name={icon as any} size={28} />
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

function OverviewTab({ target }: { target: MATarget }) {
  const evidenceFamilies: { key: string; label: string }[] = [
    { key: 'aderenza', label: 'Aderenza strategia' },
    { key: 'opportunita', label: 'Opportunità deal' },
    { key: 'economico', label: 'Profilo economico' },
  ];
  return (
    <div>
      <div>
        <h3 className={styles.sectionTitle}>Razionale aderenza Strategia</h3>
        <p>{target.rationale || 'Nessun razionale disponibile.'}</p>
      </div>

      {target.missingCriteria.length > 0 ? (
        <p>
          <strong>Criteri mancanti o parziali:</strong> {target.missingCriteria.join(', ')}
        </p>
      ) : null}

      {evidenceFamilies.map((family) => {
        const items = target.evidence.filter((item) => item.family === family.key);
        if (items.length === 0) return null;
        return (
          <div key={family.key}>
            <h4>{family.label}</h4>
            {items.map((item) => (
              <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
            ))}
          </div>
        );
      })}
      {target.evidence.filter((item) => !item.family).length > 0 && (
        <div>
          <h4>Altre evidenze</h4>
          {target.evidence
            .filter((item) => !item.family)
            .map((item) => (
              <EvidenceRow key={`${item.criterion}-${item.label}`} evidence={item} />
            ))}
        </div>
      )}
      <ScoreAdjustments adjustments={target.adjustments} />
      {target.flags && target.flags.length > 0 ? <FlagChips flags={target.flags} /> : null}
    </div>
  );
}

function EvidenceRow({ evidence }: { evidence: MATargetEvidence }) {
  const weight = evidence.weight ?? 0;
  return (
    <div>
      <div>
        <strong>{evidence.label}</strong>
        {weight > 0 ? (
          <small>
            {' '}
            {Math.round(evidence.points ?? 0)} / {Math.round(weight)}
          </small>
        ) : null}
      </div>
      <p>{evidence.value || evidenceStatusLabel(evidence.status)}</p>
    </div>
  );
}

function evidenceStatusLabel(value: string): string {
  if (value === 'match') return 'criterio soddisfatto';
  if (value === 'fuori_criterio') return 'fuori criterio';
  if (value === 'criterio_mancante') return 'criterio mancante';
  return 'match parziale';
}

function ScoreAdjustments({ adjustments }: { adjustments?: MATargetAdjustment[] }) {
  const items = (adjustments ?? []).filter((adj) => adj.factor < 1);
  if (items.length === 0) return null;
  return (
    <div>
      <span>Rettifiche al punteggio</span>
      {items.map((adj) => (
        <div key={adj.code}>
          <span>{adj.label}</span>
          <span>−{Math.round((1 - adj.factor) * 100)}%</span>
        </div>
      ))}
    </div>
  );
}

function FlagChips({ flags }: { flags?: MATargetFlag[] }) {
  if (!flags || flags.length === 0) return null;
  return (
    <div>
      {flags.map((flag) => (
        <span key={flag.code}>{flag.label}</span>
      ))}
    </div>
  );
}

function DeepAnalysisTab({
  deep,
  companyKey,
  onStarted,
}: {
  deep?: MADeepAnalysis;
  companyKey: string;
  onStarted: () => void;
}) {
  const api = useApiClient();
  const launch = useMutation({
    mutationFn: () => api.post(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/deep-dive`, {}),
    onSuccess: onStarted,
  });

  const launchButton = (
    <>
      <Button onClick={() => launch.mutate()} loading={launch.isPending} disabled={!companyKey}>
        Avvia analisi
      </Button>
      {launch.isError ? <p>Avvio non riuscito: riprova.</p> : null}
    </>
  );

  if (!deep) {
    return (
      <div>
        <EmptyState
          icon="search"
          title="Analisi non ancora eseguita"
          text="Avvia l’analisi approfondita per generare il brief analista."
        />
        {launchButton}
      </div>
    );
  }
  if (deep.status === 'queued' || deep.status === 'running') {
    return (
      <EmptyState
        icon="clipboard-check"
        title={deep.status === 'queued' ? 'In coda' : 'Analisi in corso'}
        text="L'analisi approfondita è in elaborazione."
      />
    );
  }
  if (deep.status === 'failed') {
    return (
      <div>
        <EmptyState
          icon="file-text"
          title="Analisi non riuscita"
          text={`Si è verificato un problema (${deep.errorCode || 'errore'}).`}
        />
        {launchButton}
      </div>
    );
  }
  return <DeepAnalysisContent deep={deep} variant="full" />;
}

function RegistryTab({ target }: { target: MATarget }) {
  const rawPayload = target.vendorPayload;
  const detailedForm = rawPayload?.detailedLegalForm?.description || '-';
  const startDate = rawPayload?.startDate || '-';
  const regDate = rawPayload?.registrationDate || '-';
  const pec = rawPayload?.pec || rawPayload?.PEC || '-';

  return (
    <dl className={styles.dl}>
      <div>
        <dt>Ragione Sociale</dt>
        <dd>{target.companyName}</dd>
      </div>
      <div>
        <dt>Forma Giuridica</dt>
        <dd>{detailedForm}</dd>
      </div>
      <div>
        <dt>Partita IVA</dt>
        <dd>{target.vatCode || '-'}</dd>
      </div>
      <div>
        <dt>Codice Fiscale</dt>
        <dd>{target.taxCode || '-'}</dd>
      </div>
      <div>
        <dt>Inizio Attività</dt>
        <dd>{startDate}</dd>
      </div>
      <div>
        <dt>Data Registrazione</dt>
        <dd>{regDate}</dd>
      </div>
      <div>
        <dt>Stato Attività</dt>
        <dd>{target.activityStatus || '-'}</dd>
      </div>
      <div>
        <dt>PEC</dt>
        <dd>{pec}</dd>
      </div>
      <div className={styles.wide}>
        <dt>Sede Legale</dt>
        <dd>{[target.town, target.province].filter(Boolean).join(' · ') || '-'}</dd>
      </div>
    </dl>
  );
}

