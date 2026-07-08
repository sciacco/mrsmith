import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type {
  CandidateMatchAnalysisResponse,
  MACompanyOverview,
  MACompanyOverviewAppearance,
  MACompanyOverviewCard,
  MADeepAnalysis,
  MASessionThesisReading,
  MATarget,
} from '../../api/types';
import { DeepAnalysisContent } from '../../components/deep/DeepComponents';
import { RatingStars } from '../../components/RatingStars';
import { ThesisReadingPanel } from '../../components/ThesisReadingPanel/ThesisReadingPanel';
import { CompanyRegistrySection } from '../iniziative/CompanyRegistrySection';
import { bucketLabel, dateLabel, errorLabel, sessionStatusLabel } from '../ricerche/helpers';
import styles from './SchedaAziendaPage.module.css';

type Lens =
  | { type: 'ricerca'; id: string; ambiguous: boolean }
  | { type: 'iniziativa'; id: string; ambiguous: boolean }
  | { type: 'globale'; ambiguous: boolean };

type Shareholder = {
  name: string;
  taxCode?: string;
  percentShare?: number;
  age?: number;
};

const CARD_STATE_LABELS: Record<string, string> = {
  da_contattare: 'Da contattare',
  contattata: 'Contattata',
  in_dialogo: 'In dialogo',
  approfondimento: 'Approfondimento',
  offerta: 'Offerta',
  chiusa: 'Chiusa',
  rimossa: 'Rimossa',
};

const OUTCOME_LABELS: Record<string, string> = {
  contattato: 'Contattata',
  buon_lead: 'Buon lead',
  no_go: 'No-go',
  conclusa: 'Conclusa',
  non_idonea: 'Non idonea',
  sfumata: 'Sfumata',
  rimandata: 'Rimandata',
};

function resolveLens(searchParams: URLSearchParams): Lens {
  const ricerca = searchParams.get('ricerca')?.trim();
  const iniziativa = searchParams.get('iniziativa')?.trim();
  if (ricerca) return { type: 'ricerca', id: ricerca, ambiguous: Boolean(iniziativa) };
  if (iniziativa) return { type: 'iniziativa', id: iniziativa, ambiguous: false };
  return { type: 'globale', ambiguous: false };
}

function normalizeDomainHref(domain: string): string {
  if (/^https?:\/\//i.test(domain)) return domain;
  return `https://${domain}`;
}

function formatScore(value?: number): string {
  if (value == null) return 'n.d.';
  return `${value.toLocaleString('it-IT')}/100`;
}

function ratingLabel(rating?: number): string {
  if (rating === -1) return 'Esclusa';
  if (!rating) return 'Non valutata';
  return `${'★'.repeat(rating)}${'☆'.repeat(Math.max(0, 3 - rating))}`;
}

function verdictLabel(verdict?: string): string {
  switch (verdict) {
    case 'strong_match':
      return 'Molto in tesi';
    case 'match':
      return 'In tesi';
    case 'weak_match':
      return 'Debole';
    case 'no_match':
      return 'Fuori tesi';
    case 'unclear':
      return 'Da verificare';
    default:
      return verdict ?? 'Da verificare';
  }
}

function deepStatusLabel(status?: MADeepAnalysis['status']): string {
  if (status === 'queued') return 'In coda';
  if (status === 'running') return 'In corso';
  if (status === 'ready') return 'Pronta';
  if (status === 'failed') return 'Non riuscita';
  return 'Assente';
}

function getLegalForm(target?: MATarget): string | undefined {
  const raw = target?.vendorPayload;
  return (
    raw?.detailedLegalForm?.description ||
    raw?.legalForm?.description ||
    raw?.legalForm?.legalForm?.description ||
    raw?.legalForm?.detailedLegalForm?.description ||
    raw?.legalFormDescription
  );
}

function calcAgeFromDate(value?: string): number | undefined {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return undefined;
  const now = new Date();
  let age = now.getFullYear() - date.getFullYear();
  const monthDelta = now.getMonth() - date.getMonth();
  if (monthDelta < 0 || (monthDelta === 0 && now.getDate() < date.getDate())) age -= 1;
  return age > 0 ? age : undefined;
}

function extractShareholders(target?: MATarget): Shareholder[] {
  const raw = target?.vendorPayload;
  if (!raw) return [];
  const direct = Array.isArray(raw.shareHolders) ? raw.shareHolders : undefined;
  if (direct) {
    return direct.map((item: any) => ({
      name: [item.name, item.surname].filter(Boolean).join(' ') || item.companyName || 'Socio non nominato',
      taxCode: item.taxCode,
      percentShare: item.percentShare,
      age: item.age ?? item.personAge ?? calcAgeFromDate(item.birthDate),
    }));
  }
  if (!Array.isArray(raw.shareholders)) return [];
  const list: Shareholder[] = [];
  for (const item of raw.shareholders) {
    const percent = item.percentShare ?? 0;
    const info = item.shareholdersInformation;
    if (Array.isArray(info) && info.length > 0) {
      for (const sub of info) {
        list.push({
          name: [sub.name, sub.surname].filter(Boolean).join(' ') || sub.companyName || 'Socio non nominato',
          taxCode: sub.taxCode,
          percentShare: sub.percentShare ?? percent,
          age: sub.age ?? sub.personAge ?? calcAgeFromDate(sub.birthDate),
        });
      }
    } else {
      list.push({
        name: [item.name, item.surname].filter(Boolean).join(' ') || item.companyName || 'Socio non nominato',
        taxCode: item.taxCode,
        percentShare: percent,
        age: item.age ?? item.personAge ?? calcAgeFromDate(item.birthDate),
      });
    }
  }
  return list;
}

function companySeniority(target?: MATarget): string | undefined {
  const raw = target?.vendorPayload;
  const date = raw?.companyDates?.startDate || raw?.companyDates?.incorporationDate || raw?.startDate || raw?.registrationDate;
  if (!date) return undefined;
  const year = new Date(date).getFullYear();
  if (!Number.isFinite(year)) return undefined;
  const age = new Date().getFullYear() - year;
  return age > 0 ? `${age.toLocaleString('it-IT')} anni (dal ${year})` : `Dal ${year}`;
}

function groupLabel(target?: MATarget): string | undefined {
  const group = target?.vendorPayload?.corporateGroups;
  if (!group) return undefined;
  const parts = [group.groupName ? `Gruppo ${group.groupName}` : undefined, group.holdingCompanyName ? `holding ${group.holdingCompanyName}` : undefined].filter(Boolean);
  if (parts.length > 0) return parts.join(' · ');
  if (group.belongsToGroup) return 'Appartiene a un gruppo';
  return undefined;
}

function targetDomain(target?: MATarget): string | undefined {
  return target?.webValidation?.selectedDomain || target?.vendorPayload?.webAndSocial?.website || target?.vendorPayload?.website;
}

function evidenceFromTarget(target?: MATarget): CandidateMatchAnalysisResponse | undefined {
  return target?.webValidation?.candidateMatchAnalysis;
}

function EmptyPanel({ icon, title, text }: { icon: 'file-text' | 'target' | 'bar-chart-2'; title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyIcon} aria-hidden="true">
        <Icon name={icon} size={28} />
      </span>
      <h3>{title}</h3>
      <p>{text}</p>
    </div>
  );
}

export function SchedaAziendaPage() {
  const { companyKey } = useParams<{ companyKey: string }>();
  const [searchParams] = useSearchParams();
  const lens = useMemo(() => resolveLens(searchParams), [searchParams]);
  const api = useApiClient();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [excludeReason, setExcludeReason] = useState('');
  const [excludeOpen, setExcludeOpen] = useState(false);

  const encodedCompanyKey = encodeURIComponent(companyKey ?? '');
  const overviewKey = ['ma-company-overview', companyKey];

  const overviewQuery = useQuery({
    queryKey: overviewKey,
    enabled: Boolean(companyKey),
    queryFn: () => api.get<MACompanyOverview>(`/binocolo/v1/ma/companies/${encodedCompanyKey}/overview`),
  });

  const overview = overviewQuery.data;
  const identity = overview?.identity;
  const ricercaAppearance = useMemo(() => {
    if (!overview || lens.type !== 'ricerca') return undefined;
    return overview.appearances.find((item) => item.sessionId === lens.id);
  }, [lens, overview]);
  const initiativeCard = useMemo(() => {
    if (!overview || lens.type !== 'iniziativa') return undefined;
    return overview.cards.find((item) => item.initiativeId === lens.id);
  }, [lens, overview]);
  const sourceAppearance = useMemo(() => {
    if (!overview) return undefined;
    if (ricercaAppearance) return ricercaAppearance;
    if (lens.type === 'iniziativa') {
      return overview.appearances.find((item) => item.initiativeId === lens.id || item.sessionId === initiativeCard?.createdFromSession) ?? overview.appearances[0];
    }
    return overview.appearances[0];
  }, [initiativeCard, lens, overview, ricercaAppearance]);

  const targetKey = ['ma-company-scheda-target', sourceAppearance?.sessionId, sourceAppearance?.targetId];
  const targetQuery = useQuery({
    queryKey: targetKey,
    enabled: Boolean(sourceAppearance?.sessionId && sourceAppearance?.targetId),
    queryFn: () =>
      api.get<MATarget>(
        `/binocolo/v1/ma/sessions/${encodeURIComponent(sourceAppearance?.sessionId ?? '')}/targets/${encodeURIComponent(sourceAppearance?.targetId ?? '')}`,
      ),
    retry: (failureCount, error) => !(error instanceof ApiError && error.status === 404) && failureCount < 2,
  });

  const target = targetQuery.data;
  const deep = target?.deep ?? overview?.deep;
  const deepBusy = deep?.status === 'queued' || deep?.status === 'running';
  const deepReady = deep?.status === 'ready';

  useEffect(() => {
    if (!deepBusy) return;
    const handle = window.setInterval(() => {
      void overviewQuery.refetch();
      if (targetQuery.data) void targetQuery.refetch();
    }, 5000);
    return () => window.clearInterval(handle);
  }, [deepBusy, overviewQuery, targetQuery]);

  const thesisQuery = useQuery({
    queryKey: ['ma-company-scheda-thesis', lens.type === 'ricerca' ? lens.id : undefined, ricercaAppearance?.targetId],
    enabled: Boolean(lens.type === 'ricerca' && ricercaAppearance?.targetId && deepReady),
    queryFn: () =>
      api.get<MASessionThesisReading>(
        `/binocolo/v1/ma/sessions/${encodeURIComponent(lens.type === 'ricerca' ? lens.id : '')}/targets/${encodeURIComponent(ricercaAppearance?.targetId ?? '')}/thesis-reading`,
      ),
    retry: (failureCount, error) => !(error instanceof ApiError && error.status === 404) && failureCount < 2,
  });

  const generateThesis = useMutation({
    mutationFn: () =>
      api.post<MASessionThesisReading>(
        `/binocolo/v1/ma/sessions/${encodeURIComponent(lens.type === 'ricerca' ? lens.id : '')}/targets/${encodeURIComponent(ricercaAppearance?.targetId ?? '')}/thesis-reading`,
        {},
      ),
    onSuccess: () => void thesisQuery.refetch(),
  });

  const launchDeep = useMutation({
    mutationFn: () => api.post<{ status: MADeepAnalysis['status'] }>(`/binocolo/v1/ma/companies/${encodedCompanyKey}/deep-dive`, {}),
    onSuccess: (response) => {
      toast(response.status === 'ready' ? 'Analisi già disponibile.' : 'Analisi avviata.', 'success');
      void overviewQuery.refetch();
      void targetQuery.refetch();
    },
    onError: (error) => toast(errorLabel(error), 'error'),
  });

  async function submitRating(rating: number, reason: string) {
    if (!overview || !identity || lens.type !== 'ricerca' || !ricercaAppearance) return;
    const previousOverview = queryClient.getQueryData<MACompanyOverview>(overviewKey);
    const previousTarget = queryClient.getQueryData<MATarget>(targetKey);
    const nextRating = rating === 0 ? undefined : rating;

    queryClient.setQueryData<MACompanyOverview>(overviewKey, (current) => {
      if (!current) return current;
      return {
        ...current,
        appearances: current.appearances.map((item) =>
          item.sessionId === ricercaAppearance.sessionId
            ? { ...item, rating: nextRating, exclusionReason: rating === -1 ? reason : undefined }
            : item,
        ),
      };
    });
    queryClient.setQueryData<MATarget>(targetKey, (current) => (current ? { ...current, rating: nextRating } : current));

    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${encodeURIComponent(lens.id)}/rating`, {
        companyKey: identity.companyKey,
        rating,
        ...(rating > 0 ? { scoreAtRating: ricercaAppearance.score, confidenceAtRating: target?.confidence } : {}),
        ...(reason ? { reason } : {}),
      });
      void overviewQuery.refetch();
      void targetQuery.refetch();
    } catch (error) {
      toast(errorLabel(error), 'error');
      queryClient.setQueryData(overviewKey, previousOverview);
      queryClient.setQueryData(targetKey, previousTarget);
    }
  }

  function rateCurrent(rating: number) {
    if (!ricercaAppearance) return;
    if (rating === -1) {
      setExcludeReason('');
      setExcludeOpen(true);
      return;
    }
    const nextRating = ricercaAppearance.rating === rating ? 0 : rating;
    void submitRating(nextRating, '');
  }

  if (overviewQuery.isLoading) {
    return (
      <main className={styles.page}>
        <Skeleton rows={10} />
      </main>
    );
  }

  if (overviewQuery.isError) {
    return (
      <main className={styles.page}>
        <div className={styles.statePanel} role="alert">
          <Icon name="triangle-alert" size={24} />
          <h1>Scheda non disponibile</h1>
          <p>{errorLabel(overviewQuery.error)}</p>
        </div>
      </main>
    );
  }

  if (!overview || !identity) return null;

  const analysis = evidenceFromTarget(target);
  const legalForm = getLegalForm(target);
  const domain = identity.domain || targetDomain(target);
  const thesisNotGenerated = thesisQuery.isError && thesisQuery.error instanceof ApiError && thesisQuery.error.status === 404;
  const thesisQueryError = thesisQuery.isError && !thesisNotGenerated ? errorLabel(thesisQuery.error) : null;
  const shareholders = extractShareholders(target);
  const controllingShareholder = shareholders.reduce<Shareholder | undefined>((best, item) => {
    if (!best) return item;
    return (item.percentShare ?? 0) > (best.percentShare ?? 0) ? item : best;
  }, undefined);
  const successionFlags = [
    ...(target?.flags ?? []).map((flag) => flag.label),
    ...(target?.evidence ?? [])
      .filter((item) => /successione|titolare|propriet|soci/i.test(`${item.label} ${item.criterion}`))
      .map((item) => `${item.label}${item.value ? `: ${item.value}` : ''}`),
  ];

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Scheda azienda</p>
          <h1>{identity.companyName || identity.companyKey}</h1>
          <div className={styles.headerMeta}>
            {identity.vatCode ? <span>P.IVA {identity.vatCode}</span> : null}
            {identity.taxCode ? <span>CF {identity.taxCode}</span> : null}
            {[identity.town, identity.province].filter(Boolean).length > 0 ? <span>{[identity.town, identity.province].filter(Boolean).join(' · ')}</span> : null}
            {lens.type === 'ricerca' ? <span>Lente ricerca</span> : null}
            {lens.type === 'iniziativa' ? <span>Lente iniziativa</span> : null}
          </div>
        </div>
        {lens.type === 'ricerca' && ricercaAppearance ? (
          <div className={styles.ratingBox}>
            <span>Giudizio nella ricerca</span>
            <RatingStars rating={ricercaAppearance.rating ?? 0} onRate={rateCurrent} />
          </div>
        ) : null}
      </header>

      {lens.ambiguous ? (
        <div className={styles.warning} role="status">
          <Icon name="info" size={16} />
          <span>Sono presenti sia ricerca sia iniziativa: la pagina usa la lente ricerca per evitare ambiguità.</span>
        </div>
      ) : null}

      {lens.type === 'ricerca' && !ricercaAppearance ? (
        <div className={styles.warning} role="status">
          <Icon name="info" size={16} />
          <span>Questa azienda non compare nella ricerca indicata. La scheda mostra i dati azienda senza giudizio di tesi.</span>
        </div>
      ) : null}

      {targetQuery.isError ? (
        <div className={styles.warning} role="status">
          <Icon name="triangle-alert" size={16} />
          <span>Dettaglio ricerca non disponibile: alcune sezioni mostrano solo i dati azienda.</span>
        </div>
      ) : null}

      <section className={styles.block} aria-labelledby="scheda-identita-title">
        <div className={styles.blockHeader}>
          <span className={styles.blockIndex}>1</span>
          <div>
            <h2 id="scheda-identita-title">Cosa fa ed è in tesi</h2>
            <p>Identità societaria e lettura rispetto alla ricerca selezionata.</p>
          </div>
        </div>
        <dl className={styles.identityGrid}>
          <div>
            <dt>Ragione sociale</dt>
            <dd>{identity.companyName || identity.companyKey}</dd>
          </div>
          <div>
            <dt>Forma</dt>
            <dd>{legalForm ?? 'n.d.'}</dd>
          </div>
          <div>
            <dt>Identificativi</dt>
            <dd>{[identity.vatCode ? `P.IVA ${identity.vatCode}` : undefined, identity.taxCode ? `CF ${identity.taxCode}` : undefined].filter(Boolean).join(' · ') || 'n.d.'}</dd>
          </div>
          <div>
            <dt>ATECO</dt>
            <dd>{[identity.atecoCode, identity.atecoDescription].filter(Boolean).join(' · ') || 'n.d.'}</dd>
          </div>
          <div>
            <dt>Sede</dt>
            <dd>{[identity.town, identity.province].filter(Boolean).join(' · ') || 'n.d.'}</dd>
          </div>
          <div>
            <dt>Dominio</dt>
            <dd>
              {domain ? (
                <a href={normalizeDomainHref(domain)} target="_blank" rel="noopener noreferrer" className={styles.externalLink}>
                  {domain} <Icon name="external-link" size={13} />
                </a>
              ) : (
                'n.d.'
              )}
            </dd>
          </div>
        </dl>

        {lens.type === 'ricerca' && ricercaAppearance ? (
          <div className={styles.contextGrid}>
            <div className={styles.contextCard}>
              <div className={styles.cardTopline}>
                <span className={styles.statusPill}>{verdictLabel(analysis?.verdict)}</span>
                <span>{bucketLabel(ricercaAppearance.bucket)}</span>
              </div>
              <h3>Verdetto nella ricerca</h3>
              <p>{analysis?.businessFit || target?.rationale || 'Dettaglio del giudizio non disponibile per questa ricerca.'}</p>
              <div className={styles.evidenceColumns}>
                <EvidenceList title="A favore" items={analysis?.evidenceFor} />
                <EvidenceList title="Da chiarire" items={analysis?.evidenceAgainst} />
              </div>
            </div>
            <ThesisReadingPanel
              compact
              record={thesisQuery.data}
              loading={deepReady && thesisQuery.isLoading}
              notGenerated={!deepReady || thesisNotGenerated}
              queryError={thesisQueryError}
              generationError={generateThesis.isError ? errorLabel(generateThesis.error) : null}
              generating={generateThesis.isPending}
              deepReady={deepReady}
              onGenerate={() => generateThesis.mutate()}
              generateLabel="Genera lettura"
              regenerateLabel="Rigenera lettura"
            />
          </div>
        ) : null}
      </section>

      <section className={styles.block} aria-labelledby="scheda-deep-title">
        <div className={styles.blockHeader}>
          <span className={styles.blockIndex}>2</span>
          <div>
            <h2 id="scheda-deep-title">È sana e quanto vale</h2>
            <p>Analisi finanziaria, qualità del dato, valutazione e brief.</p>
          </div>
          <span className={styles.statusPill}>{deepStatusLabel(deep?.status)}</span>
        </div>
        {!deep?.status ? (
          <div className={styles.actionState}>
            <p>Nessuna analisi approfondita disponibile per questa azienda.</p>
            <Button size="sm" onClick={() => launchDeep.mutate()} loading={launchDeep.isPending}>Avvia analisi</Button>
          </div>
        ) : deep.status === 'queued' || deep.status === 'running' ? (
          <div className={styles.actionState}>
            <p>{deep.status === 'queued' ? 'Analisi in coda.' : 'Analisi in corso.'} La scheda si aggiorna automaticamente.</p>
            {deep.updatedAt ? <span>Aggiornata {dateLabel(deep.updatedAt)}</span> : null}
          </div>
        ) : deep.status === 'failed' ? (
          <div className={styles.actionState}>
            <p>Analisi non riuscita. Puoi avviarla di nuovo.</p>
            <Button size="sm" onClick={() => launchDeep.mutate()} loading={launchDeep.isPending}>Avvia analisi</Button>
          </div>
        ) : deep.status === 'ready' && (deep.scorecard || deep.valuation || deep.brief) ? (
          <DeepAnalysisContent deep={deep} variant="full" />
        ) : (
          <div className={styles.actionState}>
            <p>Analisi pronta, dettaglio non disponibile in questa risposta.</p>
          </div>
        )}
      </section>

      <section className={styles.block} aria-labelledby="scheda-controllo-title">
        <div className={styles.blockHeader}>
          <span className={styles.blockIndex}>3</span>
          <div>
            <h2 id="scheda-controllo-title">Chi la controlla e qual è l’angolo</h2>
            <p>Compagine, controllo e segnali utili per impostare l’approccio.</p>
          </div>
        </div>
        {targetQuery.isLoading ? (
          <Skeleton rows={4} />
        ) : shareholders.length === 0 && !groupLabel(target) && !companySeniority(target) && successionFlags.length === 0 ? (
          <EmptyPanel icon="target" title="Dati di controllo non disponibili" text="La scheda non ha abbastanza elementi su soci, gruppo o successione." />
        ) : (
          <div className={styles.angleGrid}>
            <div className={styles.angleCard}>
              <h3>Controllo</h3>
              {controllingShareholder ? (
                <dl>
                  <div><dt>Socio principale</dt><dd>{controllingShareholder.name}</dd></div>
                  <div><dt>Quota</dt><dd>{controllingShareholder.percentShare != null && controllingShareholder.percentShare > 0 ? `${controllingShareholder.percentShare.toLocaleString('it-IT')}%` : 'n.d.'}</dd></div>
                  <div><dt>Età</dt><dd>{controllingShareholder.age ? `${controllingShareholder.age} anni` : 'n.d.'}</dd></div>
                </dl>
              ) : <p className={styles.muted}>Socio di controllo non identificato.</p>}
            </div>
            <div className={styles.angleCard}>
              <h3>Gruppo e anzianità</h3>
              <dl>
                <div><dt>Holding</dt><dd>{groupLabel(target) ?? 'n.d.'}</dd></div>
                <div><dt>Anzianità</dt><dd>{companySeniority(target) ?? 'n.d.'}</dd></div>
              </dl>
            </div>
            <div className={styles.angleCardWide}>
              <h3>Segnali di angolo</h3>
              {successionFlags.length > 0 ? (
                <ul className={styles.bulletList}>{successionFlags.slice(0, 6).map((item) => <li key={item}>{item}</li>)}</ul>
              ) : (
                <p className={styles.muted}>Nessun flag specifico disponibile.</p>
              )}
            </div>
          </div>
        )}
      </section>

      <section className={styles.block} aria-labelledby="scheda-storia-title">
        <div className={styles.blockHeader}>
          <span className={styles.blockIndex}>4</span>
          <div>
            <h2 id="scheda-storia-title">Cosa ne sappiamo e cosa ne abbiamo fatto</h2>
            <p>Registro azienda, apparizioni nelle ricerche e iniziative collegate.</p>
          </div>
        </div>
        <div className={styles.registryWrap}>
          <CompanyRegistrySection companyKey={identity.companyKey} vatCode={identity.vatCode} companyName={identity.companyName} readOnly={false} />
        </div>
        <HistorySection appearances={overview.appearances} />
        <CardsSection cards={overview.cards} activeInitiativeId={lens.type === 'iniziativa' ? lens.id : undefined} />
      </section>

      <Modal open={excludeOpen} onClose={() => setExcludeOpen(false)} title="Escludi target" size="sm">
        <div className={styles.excludeModalBody}>
          <p>&ldquo;{identity.companyName || identity.companyKey}&rdquo; esce dalla shortlist. Il motivo alimenta la calibrazione futura del punteggio.</p>
          <div className={styles.excludeReasons}>
            {['Fuori settore', 'Troppo piccola', 'Distress', 'Non in vendita'].map((preset) => (
              <button
                key={preset}
                type="button"
                className={`${styles.excludeChip} ${excludeReason === preset ? styles.excludeChipActive : ''}`}
                onClick={() => setExcludeReason(preset)}
              >
                {preset}
              </button>
            ))}
          </div>
          <input
            type="text"
            className={styles.excludeReasonInput}
            placeholder="Motivo libero (opzionale)"
            value={excludeReason}
            onChange={(event) => setExcludeReason(event.target.value)}
            maxLength={300}
          />
          <div className={styles.modalActions}>
            <Button variant="secondary" onClick={() => setExcludeOpen(false)}>Annulla</Button>
            <Button
              variant="danger"
              onClick={() => {
                setExcludeOpen(false);
                void submitRating(-1, excludeReason.trim());
              }}
              leftIcon={<Icon name="x-circle" size={16} />}
            >
              Escludi
            </Button>
          </div>
        </div>
      </Modal>
    </main>
  );
}

function EvidenceList({ title, items }: { title: string; items?: string[] }) {
  if (!items || items.length === 0) return null;
  return (
    <div>
      <h4>{title}</h4>
      <ul className={styles.bulletList}>
        {items.map((item) => <li key={item}>{item}</li>)}
      </ul>
    </div>
  );
}

function HistorySection({ appearances }: { appearances: MACompanyOverviewAppearance[] }) {
  return (
    <div className={styles.subSection}>
      <h3>Storia nelle ricerche</h3>
      {appearances.length === 0 ? (
        <EmptyPanel icon="file-text" title="Nessuna apparizione" text="L’azienda non risulta ancora vista da una ricerca." />
      ) : (
        <div className={styles.timeline}>
          {appearances.map((appearance) => (
            <article key={`${appearance.sessionId}-${appearance.targetId}`} className={styles.timelineItem}>
              <div>
                <Link to={`/ricerche/${appearance.sessionId}`} className={styles.rowTitle}>{appearance.sessionTitle || appearance.sessionId}</Link>
                <p>{sessionStatusLabel(appearance.sessionStatus as any)} · {bucketLabel(appearance.bucket)} · punteggio {formatScore(appearance.score)}</p>
                {appearance.initiativeTitle ? <p>Iniziativa: {appearance.initiativeTitle}</p> : null}
              </div>
              <div className={styles.historyFacts}>
                <span>{ratingLabel(appearance.rating)}</span>
                <span>Vista il {dateLabel(appearance.createdAt)}</span>
                {appearance.scoreAtRating != null ? <span>Score al giudizio {formatScore(appearance.scoreAtRating)}</span> : null}
                {appearance.ratedAt ? <span>Giudizio il {dateLabel(appearance.ratedAt)}</span> : null}
                {appearance.exclusionReason ? <span>Motivo: {appearance.exclusionReason}</span> : null}
                {(appearance.outcomes ?? []).map((outcome) => (
                  <span key={outcome.id}>{OUTCOME_LABELS[outcome.event] ?? outcome.event}{outcome.note ? ` · ${outcome.note}` : ''}</span>
                ))}
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}

function CardsSection({ cards, activeInitiativeId }: { cards: MACompanyOverviewCard[]; activeInitiativeId?: string }) {
  const sorted = [...cards].sort((a, b) => Number(b.initiativeId === activeInitiativeId) - Number(a.initiativeId === activeInitiativeId));
  return (
    <div className={styles.subSection}>
      <h3>Iniziative collegate</h3>
      {sorted.length === 0 ? (
        <EmptyPanel icon="file-text" title="Nessuna iniziativa" text="Non risultano card aperte o storiche per questa azienda." />
      ) : (
        <div className={styles.cardGrid}>
          {sorted.map((card) => (
            <article key={`${card.initiativeId}-${card.companyKey}`} className={`${styles.linkedCard} ${card.initiativeId === activeInitiativeId ? styles.linkedCardActive : ''}`}>
              <div>
                <Link to={`/iniziative/${card.initiativeId}`} className={styles.rowTitle}>{card.initiativeTitle || card.initiativeId}</Link>
                {card.initiativeId === activeInitiativeId ? <span className={styles.statusPill}>Lente attiva</span> : null}
              </div>
              <dl>
                <div><dt>Stato</dt><dd>{CARD_STATE_LABELS[card.state] ?? card.state}</dd></div>
                <div><dt>Esito</dt><dd>{card.esito ? OUTCOME_LABELS[card.esito] ?? card.esito : 'n.d.'}</dd></div>
                {card.lastEvent ? <div><dt>Ultimo evento</dt><dd>{card.lastEvent}</dd></div> : null}
                <div><dt>Aggiornata</dt><dd>{dateLabel(card.updatedAt)}</dd></div>
              </dl>
              <Link to={`/iniziative/${card.initiativeId}/dossier/${encodeURIComponent(card.companyKey)}`} className={styles.externalLink}>Dossier card <Icon name="external-link" size={13} /></Link>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
