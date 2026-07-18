import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, Skeleton, StatusBadge, useToast, type StatusBadgeVariant } from '@mrsmith/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type {
  CandidateMatchAnalysisResponse,
  MACompanyOverview,
  MACompanyOverviewAppearance,
  MACompanyOverviewCard,
  MACreateInitiativeCardResponse,
  MADeepAnalysis,
  MAInitiativeListResponse,
  MASessionThesisReading,
  MATarget,
} from '../../api/types';
import { IRLPanel } from '../../components/company/IRLPanel';
import { ShareholdersDetail, hasShareholdersDetail } from '../../components/company/ShareholdersDetail';
import { VendorFinancials, hasVendorFinancialsData, vendorFinancialSheetsCount } from '../../components/company/VendorFinancials';
import { WebVerificationDetail, hasWebVerificationDetail } from '../../components/company/WebVerificationDetail';
import { DeepAnalysisContent, formatDeepCompactEuro } from '../../components/deep/DeepComponents';
import { LabeledDisclosure } from '../../components/scheda/LabeledDisclosure';
import { resolveCohortPosition, readCohort } from '../../components/scheda/cohort';
import { LensBar, type LensKeyNumber, type LensOption } from '../../components/scheda/LensBar';
import { SpineNav, type SpineItem } from '../../components/scheda/SpineNav';
import { useSectionSpy } from '../../components/scheda/useSectionSpy';
import { ThesisReadingPanel } from '../../components/ThesisReadingPanel/ThesisReadingPanel';
import { CompanyRegistrySection } from '../iniziative/CompanyRegistrySection';
import { bucketLabelWithSuppressionHistory, dateLabel, errorLabel, sessionStatusLabel } from '../ricerche/helpers';
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

type OwnershipSummary =
  | { kind: 'majority'; label: 'Socio di maggioranza'; shareholders: Shareholder[]; residualNote: string }
  | { kind: 'tie'; label: 'Soci principali' | 'Soci'; shareholders: Shareholder[]; residualNote?: string }
  | { kind: 'singleTop'; label: 'Socio principale'; shareholders: Shareholder[]; residualNote?: string };

const SPINE_IDS = ['scheda-identita-title', 'scheda-deep-title', 'scheda-controllo-title', 'scheda-storia-title'] as const;

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

function appearanceBucketLabel(appearance: MACompanyOverviewAppearance): string {
  return bucketLabelWithSuppressionHistory(appearance.bucket, appearance.suppressedReason);
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

const ITALIAN_TAX_CODE_MONTHS = { A: 0, B: 1, C: 2, D: 3, E: 4, H: 5, L: 6, M: 7, P: 8, R: 9, S: 10, T: 11 } as const;

function calcAgeFromDate(value?: string): number | undefined {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return undefined;
  return calcAgeFromBirthDate(date);
}

function calcAgeFromBirthDate(date: Date): number | undefined {
  const now = new Date();
  let age = now.getFullYear() - date.getFullYear();
  const monthDelta = now.getMonth() - date.getMonth();
  if (monthDelta < 0 || (monthDelta === 0 && now.getDate() < date.getDate())) age -= 1;
  return age > 0 && age <= 120 ? age : undefined;
}

function ageFromItalianTaxCode(taxCode?: string): number | undefined {
  const code = taxCode?.trim().toUpperCase();
  if (!code || code.length !== 16) return undefined;
  const yearPart = Number.parseInt(code.slice(6, 8), 10);
  const rawDay = Number.parseInt(code.slice(9, 11), 10);
  const month = ITALIAN_TAX_CODE_MONTHS[code[8] as keyof typeof ITALIAN_TAX_CODE_MONTHS];
  if (!Number.isFinite(yearPart) || !Number.isFinite(rawDay) || month == null) return undefined;
  const day = rawDay > 40 ? rawDay - 40 : rawDay;
  if (day < 1 || day > 31) return undefined;
  const currentYear = new Date().getFullYear();
  let year = Math.floor(currentYear / 100) * 100 + yearPart;
  if (year > currentYear) year -= 100;
  return calcAgeFromBirthDate(new Date(year, month, day));
}

function shareholderAge(item: any): number | undefined {
  return item.age ?? item.personAge ?? calcAgeFromDate(item.birthDate) ?? ageFromItalianTaxCode(item.taxCode);
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
      age: shareholderAge(item),
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
          age: shareholderAge(sub),
        });
      }
    } else {
      list.push({
        name: [item.name, item.surname].filter(Boolean).join(' ') || item.companyName || 'Socio non nominato',
        taxCode: item.taxCode,
        percentShare: percent,
        age: shareholderAge(item),
      });
    }
  }
  return list;
}

function buildOwnershipSummary(shareholders: Shareholder[]): OwnershipSummary | undefined {
  if (shareholders.length === 0) return undefined;

  const sorted = [...shareholders].sort((a, b) => (b.percentShare ?? 0) - (a.percentShare ?? 0));
  const topShare = sorted[0]?.percentShare ?? 0;
  const topShareholders = sorted.filter((item) => (item.percentShare ?? 0) === topShare);
  const knownTotal = shareholders.reduce((sum, item) => sum + (item.percentShare ?? 0), 0);

  const formatPercent = (value: number) => `${value.toLocaleString('it-IT')}%`;
  const residual = sorted.filter((item) => !topShareholders.includes(item));
  const residualKnownTotal = residual.reduce((sum, item) => sum + (item.percentShare ?? 0), 0);
  const residualParts = residual
    .filter((item) => item.percentShare != null && item.percentShare > 0)
    .map((item) => `${item.name} ${formatPercent(item.percentShare ?? 0)}`);
  const residualNote = residualKnownTotal > 0
    ? `Quote residue note: ${residualParts.join(' · ')}.`
    : knownTotal < 100
      ? `Quote residue non dettagliate: ${formatPercent(Math.max(0, 100 - knownTotal))}.`
      : '';

  if (topShareholders.length === 1 && topShare > 50) {
    return { kind: 'majority', label: 'Socio di maggioranza', shareholders: topShareholders, residualNote };
  }

  if (topShareholders.length > 1) {
    const tieTotal = topShareholders.reduce((sum, item) => sum + (item.percentShare ?? 0), 0);
    return {
      kind: 'tie',
      label: tieTotal < 100 ? 'Soci principali' : 'Soci',
      shareholders: topShareholders,
      residualNote: residualNote || undefined,
    };
  }

  return { kind: 'singleTop', label: 'Socio principale', shareholders: topShareholders, residualNote: residualNote || undefined };
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

function EmptyPanel({ icon, title, text, action }: { icon: 'file-text' | 'target' | 'bar-chart-2'; title: string; text: string; action?: ReactNode }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyIcon} aria-hidden="true">
        <Icon name={icon} size={28} />
      </span>
      <h3>{title}</h3>
      <p>{text}</p>
      {action}
    </div>
  );
}

export function SchedaAziendaPage() {
  const { companyKey } = useParams<{ companyKey: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const lens = useMemo(() => resolveLens(searchParams), [searchParams]);
  const api = useApiClient();
  const navigate = useNavigate();
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
  const deepStatus = deep?.status;
  const deepBusy = deepStatus === 'queued' || deepStatus === 'running';
  const deepReady = deepStatus === 'ready';

  useEffect(() => {
    if (!deepBusy) return;
    const handle = window.setInterval(() => {
      void overviewQuery.refetch();
      if (targetQuery.data) void targetQuery.refetch();
    }, 5000);
    return () => window.clearInterval(handle);
  }, [deepBusy, overviewQuery, targetQuery]);

  // S3 — annuncio del deep che arriva. Reagisce SOLO alla transizione di stato
  // (busy → ready|failed), mai a ogni poll. Il valore precedente è tenuto in un
  // ref insieme al companyKey: alla prima resa di una scheda (o dopo un cambio
  // azienda con la staffetta) la baseline è "fresca" e nessun toast scatta —
  // così una scheda aperta con il deep già pronto resta silenziosa.
  const prevDeepRef = useRef<{ companyKey?: string; status?: MADeepAnalysis['status'] }>({});
  const [deepArrivedPulse, setDeepArrivedPulse] = useState(false);

  useEffect(() => {
    const prev = prevDeepRef.current;
    const sameCompany = prev.companyKey === companyKey;
    const prevStatus = sameCompany ? prev.status : undefined;
    prevDeepRef.current = { companyKey, status: deepStatus };
    if (!sameCompany) return; // baseline per una nuova azienda: mai annunciare
    const wasBusy = prevStatus === 'queued' || prevStatus === 'running';
    if (!wasBusy) return;
    if (deepStatus === 'ready') {
      toast('Analisi pronta', 'success');
      setDeepArrivedPulse(true);
    } else if (deepStatus === 'failed') {
      toast('Analisi non riuscita', 'error');
    }
  }, [companyKey, deepStatus, toast]);

  // Pulse one-shot (§8.3): il flag si spegne da solo, così la classe di
  // animazione non resta appesa e non si ri-triggera sui poll successivi.
  // Il timeout copre anche prefers-reduced-motion (nessun onAnimationEnd).
  useEffect(() => {
    if (!deepArrivedPulse) return;
    const handle = window.setTimeout(() => setDeepArrivedPulse(false), 1500);
    return () => window.clearTimeout(handle);
  }, [deepArrivedPulse]);

  const contentReady = Boolean(overview && identity);
  const { activeId: activeSpineId, scrollTo: scrollToSection } = useSectionSpy(SPINE_IDS, contentReady);

  // Staffetta: la coorte è usata solo se coerente con la lente attiva e con il
  // companyKey corrente; altrimenti degradazione elegante (niente frecce, niente
  // «N di M»). Ri-letta a ogni cambio di lente/azienda (la storage è stabile).
  const cohortPosition = useMemo(
    () => resolveCohortPosition(readCohort(), lens.type, lens.type === 'globale' ? undefined : lens.id, companyKey),
    [lens, companyKey],
  );

  const goToCohortKey = useCallback(
    (key: string | undefined) => {
      if (!key || !cohortPosition) return;
      navigate(`/aziende/${encodeURIComponent(key)}?${cohortPosition.lensType}=${encodeURIComponent(cohortPosition.lensId)}`);
    },
    [navigate, cohortPosition],
  );

  // Prefetch della vicina successiva (stessa queryKey/fetch dell'overview della
  // scheda) al mount e a ogni navigazione: la transizione con `j` è istantanea.
  useEffect(() => {
    const key = cohortPosition?.nextKey;
    if (!key) return;
    void queryClient.prefetchQuery({
      queryKey: ['ma-company-overview', key],
      queryFn: () => api.get<MACompanyOverview>(`/binocolo/v1/ma/companies/${encodeURIComponent(key)}/overview`),
    });
  }, [cohortPosition?.nextKey, api, queryClient]);

  useEffect(() => {
    function onKey(event: KeyboardEvent) {
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      if (excludeOpen) return;
      const node = event.target as HTMLElement | null;
      if (node) {
        const tag = node.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || node.isContentEditable) return;
      }
      if (event.key === 'j') {
        if (!cohortPosition?.nextKey) return;
        event.preventDefault();
        goToCohortKey(cohortPosition.nextKey);
        return;
      }
      if (event.key === 'k') {
        if (!cohortPosition?.prevKey) return;
        event.preventDefault();
        goToCohortKey(cohortPosition.prevKey);
        return;
      }
      const index = ['1', '2', '3', '4'].indexOf(event.key);
      const targetId = SPINE_IDS[index];
      if (!targetId) return;
      event.preventDefault();
      scrollToSection(targetId);
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [excludeOpen, scrollToSection, cohortPosition, goToCohortKey]);

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
  const ownershipSummary = buildOwnershipSummary(shareholders);
  const successionFlags = [
    ...(target?.flags ?? []).map((flag) => flag.label),
    ...(target?.evidence ?? [])
      .filter((item) => /successione|titolare|propriet|soci/i.test(`${item.label} ${item.criterion}`))
      .map((item) => `${item.label}${item.value ? `: ${item.value}` : ''}`),
  ];
  const showWebVerification = hasWebVerificationDetail(target);
  const showVendorFinancials = hasVendorFinancialsData(target);
  const showShareholdersDetail = hasShareholdersDetail(target);
  const verificationDensity = target?.webValidation?.selectedDomain ? 'dominio confermato' : undefined;
  const financialSheetsCount = vendorFinancialSheetsCount(target);
  const showIRL = lens.type === 'iniziativa' && Boolean(initiativeCard && identity.companyKey);
  const companyName = identity.companyName || identity.companyKey;

  const lensOptions: LensOption[] = [];
  const seenSessions = new Set<string>();
  for (const appearance of overview.appearances) {
    if (seenSessions.has(appearance.sessionId)) continue;
    seenSessions.add(appearance.sessionId);
    lensOptions.push({ value: `ricerca:${appearance.sessionId}`, label: appearance.sessionTitle || appearance.sessionId });
  }
  const seenInitiatives = new Set<string>();
  for (const card of overview.cards) {
    if (seenInitiatives.has(card.initiativeId)) continue;
    seenInitiatives.add(card.initiativeId);
    lensOptions.push({ value: `iniziativa:${card.initiativeId}`, label: card.initiativeTitle || card.initiativeId });
  }
  lensOptions.push({ value: 'globale', label: 'Vista globale' });

  const selectedLens = lens.type === 'ricerca' ? `ricerca:${lens.id}` : lens.type === 'iniziativa' ? `iniziativa:${lens.id}` : 'globale';
  const notInLens = lens.type === 'ricerca' && !ricercaAppearance;
  if (!lensOptions.some((option) => option.value === selectedLens)) {
    lensOptions.unshift({ value: selectedLens, label: lens.type === 'ricerca' ? 'Ricerca selezionata' : 'Iniziativa selezionata' });
  }

  function changeLens(value: string) {
    if (value === 'globale') {
      setSearchParams({});
      return;
    }
    const idx = value.indexOf(':');
    const type = value.slice(0, idx);
    const id = value.slice(idx + 1);
    setSearchParams(type === 'ricerca' ? { ricerca: id } : { iniziativa: id });
  }

  let originLabel: string;
  let originTo: string | undefined;
  if (lens.type === 'ricerca') {
    const title = ricercaAppearance?.sessionTitle || overview.appearances.find((item) => item.sessionId === lens.id)?.sessionTitle || lens.id;
    originLabel = `Ricerca «${title}»`;
    originTo = `/ricerche/${lens.id}`;
  } else if (lens.type === 'iniziativa') {
    const title = initiativeCard?.initiativeTitle || overview.cards.find((item) => item.initiativeId === lens.id)?.initiativeTitle || lens.id;
    originLabel = `Iniziativa «${title}»`;
    originTo = `/iniziative/${lens.id}`;
  } else {
    originLabel = 'Vista globale';
    originTo = undefined;
  }

  const lensVerdict =
    lens.type === 'ricerca' && ricercaAppearance
      ? { label: verdictLabel(analysis?.verdict), bucket: appearanceBucketLabel(ricercaAppearance) }
      : undefined;
  const lensRating =
    lens.type === 'ricerca' && ricercaAppearance ? { value: ricercaAppearance.rating ?? 0, onRate: rateCurrent } : undefined;

  const scorecard = deep?.scorecard;
  const valuation = deep?.valuation;
  const turnoverValue = scorecard?.turnover ?? target?.turnover;
  const ebitdaValue = scorecard?.ebitda;
  const equityLow = valuation?.equityLow ?? valuation?.bridge?.equityLow;
  const equityHigh = valuation?.equityHigh ?? valuation?.bridge?.equityHigh;
  const redFlagsCount = deep?.brief?.redFlags?.length;
  const equityValue =
    equityLow != null && equityHigh != null
      ? `${formatDeepCompactEuro(equityLow)}–${formatDeepCompactEuro(equityHigh)}`
      : equityLow != null || equityHigh != null
        ? formatDeepCompactEuro(equityLow ?? equityHigh)
        : '—';
  const showKeyStrip = Boolean(deep?.status) || turnoverValue != null;
  const keyNumbers: LensKeyNumber[] | undefined = showKeyStrip
    ? [
        { key: 'turnover', label: 'Fatturato', value: formatDeepCompactEuro(turnoverValue ?? undefined), targetId: 'scheda-deep-title' },
        { key: 'ebitda', label: 'EBITDA', value: formatDeepCompactEuro(ebitdaValue ?? undefined), targetId: 'scheda-deep-title' },
        { key: 'equity', label: 'Equity', value: equityValue, targetId: 'scheda-deep-title' },
        { key: 'redflags', label: 'Red flag', value: redFlagsCount != null ? String(redFlagsCount) : '—', targetId: 'scheda-deep-title' },
      ]
    : undefined;

  const deepBadgeVariant: StatusBadgeVariant =
    deepStatus === 'ready' ? 'success' : deepStatus === 'running' ? 'accent' : deepStatus === 'failed' ? 'danger' : 'neutral';
  const controlAbsent =
    !targetQuery.isLoading &&
    shareholders.length === 0 &&
    !groupLabel(target) &&
    !companySeniority(target) &&
    successionFlags.length === 0;
  const spineItems: SpineItem[] = [
    { id: SPINE_IDS[0], index: 1, title: 'Cosa fa ed è in tesi' },
    {
      id: SPINE_IDS[1],
      index: 2,
      title: 'È sana e quanto vale',
      meta: <StatusBadge value={deepStatusLabel(deepStatus)} variant={deepBadgeVariant} dot={false} />,
    },
    { id: SPINE_IDS[2], index: 3, title: 'Chi la controlla', meta: controlAbsent ? '—' : undefined },
    {
      id: SPINE_IDS[3],
      index: 4,
      title: 'Cosa ne sappiamo',
      meta: `${overview.appearances.length} ricerche · ${overview.cards.length} iniziative`,
    },
  ];

  return (
    <main className={styles.page}>
      <LensBar
        companyName={companyName}
        originLabel={originLabel}
        originTo={originTo}
        lensOptions={lensOptions}
        selectedLens={selectedLens}
        onLensChange={changeLens}
        notInLensNote={notInLens ? 'Azienda non presente in questa ricerca' : undefined}
        verdict={lensVerdict}
        rating={lensRating}
        keyNumbers={keyNumbers}
        onKeyNumberClick={scrollToSection}
        staffetta={
          cohortPosition
            ? {
                position: cohortPosition.position,
                total: cohortPosition.total,
                onPrev: cohortPosition.prevKey ? () => goToCohortKey(cohortPosition.prevKey) : undefined,
                onNext: cohortPosition.nextKey ? () => goToCohortKey(cohortPosition.nextKey) : undefined,
              }
            : undefined
        }
      />

      <div className={styles.layout}>
        <SpineNav items={spineItems} activeId={activeSpineId} onNavigate={scrollToSection} />

        <div className={styles.content}>
      <header className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Scheda azienda</p>
          <h1>{companyName}</h1>
          <div className={styles.headerMeta}>
            {identity.vatCode ? <span>P.IVA {identity.vatCode}</span> : null}
            {identity.taxCode ? <span>CF {identity.taxCode}</span> : null}
            {[identity.town, identity.province].filter(Boolean).length > 0 ? <span>{[identity.town, identity.province].filter(Boolean).join(' · ')}</span> : null}
          </div>
        </div>
      </header>

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
            <dt>Forma</dt>
            <dd>{legalForm ?? 'n.d.'}</dd>
          </div>
          <div>
            <dt>ATECO</dt>
            <dd>{[identity.atecoCode, identity.atecoDescription].filter(Boolean).join(' · ') || 'n.d.'}</dd>
          </div>
          <div>
            <dt>Dominio</dt>
            <dd>
              {domain ? (
                <span className={styles.domainValue}>
                  <a href={normalizeDomainHref(domain)} target="_blank" rel="noopener noreferrer" className={styles.externalLink}>
                    {domain} <Icon name="external-link" size={13} />
                  </a>
                  {identity.domainMethod === 'manual' && identity.identityState !== 'verified' ? (
                    <StatusBadge value="Dominio non confermato" variant="warning" dot={false} />
                  ) : null}
                </span>
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
                <span>{appearanceBucketLabel(ricercaAppearance)}</span>
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

        {target && showWebVerification ? (
          <LabeledDisclosure title="Dettaglio della verifica" density={verificationDensity}>
            <WebVerificationDetail target={target} />
          </LabeledDisclosure>
        ) : null}
      </section>

      <section
        className={`${styles.block} ${deepArrivedPulse ? styles.blockPulse : ''}`}
        aria-labelledby="scheda-deep-title"
        onAnimationEnd={deepArrivedPulse ? () => setDeepArrivedPulse(false) : undefined}
      >
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

        {target && showVendorFinancials ? (
          <LabeledDisclosure
            title="Bilanci (fonte camerale)"
            density={financialSheetsCount > 0 ? `${financialSheetsCount} ${financialSheetsCount === 1 ? 'esercizio' : 'esercizi'}` : undefined}
          >
            <VendorFinancials target={target} />
          </LabeledDisclosure>
        ) : null}
      </section>

      <section className={styles.block} aria-labelledby="scheda-controllo-title">
        <div className={styles.blockHeader}>
          <span className={styles.blockIndex}>3</span>
          <div>
            <h2 id="scheda-controllo-title">Chi la controlla e qual è la leva di approccio</h2>
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
              {ownershipSummary ? (
                <>
                  <dl>
                    <div>
                      <dt>{ownershipSummary.label}</dt>
                      <dd>
                        <ul className={styles.shareholderList}>
                          {ownershipSummary.shareholders.map((shareholder) => (
                            <li key={`${shareholder.taxCode ?? shareholder.name}-${shareholder.percentShare ?? 'nd'}`}>
                              <span>{shareholder.name}</span>
                              <span>{shareholder.percentShare != null && shareholder.percentShare > 0 ? `${shareholder.percentShare.toLocaleString('it-IT')}%` : 'n.d.'}</span>
                              {shareholder.age ? <span>{shareholder.age} anni</span> : null}
                            </li>
                          ))}
                        </ul>
                      </dd>
                    </div>
                  </dl>
                  {ownershipSummary.residualNote ? <p className={styles.shareholderNote}>{ownershipSummary.residualNote}</p> : null}
                </>
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
              <h3>Leve di approccio</h3>
              {successionFlags.length > 0 ? (
                <ul className={styles.bulletList}>{successionFlags.slice(0, 6).map((item) => <li key={item}>{item}</li>)}</ul>
              ) : (
                <p className={styles.muted}>Nessun flag specifico disponibile.</p>
              )}
            </div>
          </div>
        )}

        {target && showShareholdersDetail ? (
          <LabeledDisclosure title="Tutti i soci" density={String(shareholders.length)}>
            <ShareholdersDetail target={target} />
          </LabeledDisclosure>
        ) : null}
      </section>

      <section className={styles.block} aria-labelledby="scheda-storia-title">
        <div className={styles.blockHeader}>
          <span className={styles.blockIndex}>4</span>
          <div>
            <h2 id="scheda-storia-title">Cosa ne sappiamo e cosa ne abbiamo fatto</h2>
            <p>Registro azienda, ricerche e iniziative collegate.</p>
          </div>
        </div>
        <div className={styles.registryWrap}>
          <CompanyRegistrySection companyKey={identity.companyKey} vatCode={identity.vatCode} companyName={identity.companyName} readOnly={false} />
        </div>
        {showIRL ? (
          <div className={styles.subSection}>
            <h3>IRL</h3>
            <IRLPanel initiativeId={lens.type === 'iniziativa' ? lens.id : ''} companyKey={identity.companyKey} companyName={identity.companyName || identity.companyKey} />
          </div>
        ) : null}
        <HistorySection appearances={overview.appearances} companyKey={identity.companyKey} />
        <CardsSection cards={overview.cards} companyKey={identity.companyKey} activeInitiativeId={lens.type === 'iniziativa' ? lens.id : undefined} />
      </section>
        </div>
      </div>

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

function HistorySection({ appearances, companyKey }: { appearances: MACompanyOverviewAppearance[]; companyKey: string }) {
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
                <Link
                  to={`/aziende/${encodeURIComponent(companyKey)}?ricerca=${encodeURIComponent(appearance.sessionId)}`}
                  className={styles.rowTitle}
                >
                  {appearance.sessionTitle || appearance.sessionId}
                </Link>
                <p>{sessionStatusLabel(appearance.sessionStatus as any)} · {appearanceBucketLabel(appearance)} · punteggio {formatScore(appearance.score)}</p>
                {appearance.initiativeTitle ? <p>Iniziativa: {appearance.initiativeTitle}</p> : null}
                <Link to={`/ricerche/${appearance.sessionId}`} className={styles.externalLink}>
                  Apri ricerca <Icon name="external-link" size={13} />
                </Link>
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

function CardsSection({ cards, companyKey, activeInitiativeId }: { cards: MACompanyOverviewCard[]; companyKey: string; activeInitiativeId?: string }) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [pickerOpen, setPickerOpen] = useState(false);
  const [creatingFor, setCreatingFor] = useState<string | null>(null);
  const initiatives = useQuery({
    queryKey: ['ma-initiatives', 'active'],
    enabled: pickerOpen,
    queryFn: () => api.get<MAInitiativeListResponse>('/binocolo/v1/ma/initiatives'),
  });
  const existingIds = new Set(cards.filter((card) => card.state !== 'chiusa' && card.state !== 'rimossa').map((card) => card.initiativeId));
  const available = (initiatives.data?.items ?? []).filter((initiative) => !existingIds.has(initiative.id));
  const sorted = [...cards].sort((a, b) => Number(b.initiativeId === activeInitiativeId) - Number(a.initiativeId === activeInitiativeId));

  async function addToInitiative(initiativeId: string) {
    setCreatingFor(initiativeId);
    try {
      await api.post<MACreateInitiativeCardResponse>(`/binocolo/v1/ma/initiatives/${initiativeId}/cards`, { companyKey });
      await queryClient.invalidateQueries({ queryKey: ['ma-company-overview', companyKey] });
      setPickerOpen(false);
      toast('Azienda aggiunta all’iniziativa.', 'success');
    } catch (error) {
      toast(error instanceof ApiError && error.status === 409 ? 'Azienda già presente in questa iniziativa.' : errorLabel(error), 'error');
    } finally {
      setCreatingFor(null);
    }
  }

  const addAction = <Button size="sm" onClick={() => setPickerOpen(true)} leftIcon={<Icon name="plus" size={14} />}>Aggiungi a iniziativa</Button>;
  return (
    <div className={styles.subSection}>
      <div className={styles.subSectionHeader}><h3>Iniziative collegate</h3>{sorted.length > 0 ? addAction : null}</div>
      {sorted.length === 0 ? (
        <EmptyPanel icon="file-text" title="Nessuna iniziativa" text="Non risultano card aperte o storiche per questa azienda." action={addAction} />
      ) : (
        <div className={styles.cardGrid}>
          {sorted.map((card) => (
            <article key={`${card.initiativeId}-${card.companyKey}`} className={`${styles.linkedCard} ${card.initiativeId === activeInitiativeId ? styles.linkedCardActive : ''}`}>
              <div>
                <Link
                  to={`/aziende/${encodeURIComponent(companyKey)}?iniziativa=${encodeURIComponent(card.initiativeId)}`}
                  className={styles.rowTitle}
                >
                  {card.initiativeTitle || card.initiativeId}
                </Link>
                {card.initiativeId === activeInitiativeId ? <span className={styles.statusPill}>Lente attiva</span> : null}
                {card.origin === 'direct' ? <span className={styles.statusPill}>Diretta</span> : null}
              </div>
              <dl>
                <div><dt>Stato</dt><dd>{CARD_STATE_LABELS[card.state] ?? card.state}</dd></div>
                <div><dt>Esito</dt><dd>{card.esito ? OUTCOME_LABELS[card.esito] ?? card.esito : 'n.d.'}</dd></div>
                {card.lastEvent ? <div><dt>Ultimo evento</dt><dd>{card.lastEvent}</dd></div> : null}
                <div><dt>Aggiornata</dt><dd>{dateLabel(card.updatedAt)}</dd></div>
              </dl>
              <Link to={`/iniziative/${encodeURIComponent(card.initiativeId)}`} className={styles.externalLink}>Apri iniziativa <Icon name="external-link" size={13} /></Link>
            </article>
          ))}
        </div>
      )}
      <Modal open={pickerOpen} onClose={() => setPickerOpen(false)} title="Aggiungi a iniziativa" size="md" dismissible={!creatingFor}>
        <div className={styles.initiativePicker}>
          <p>Seleziona l’iniziativa in cui aprire la card di lavorazione.</p>
          {initiatives.isLoading ? <Skeleton rows={3} /> : initiatives.isError ? (
            <div className={styles.warning} role="alert"><Icon name="triangle-alert" size={16} /><span>{errorLabel(initiatives.error)}</span></div>
          ) : available.length === 0 ? (
            <p className={styles.muted}>Nessuna iniziativa attiva disponibile.</p>
          ) : available.map((initiative) => (
            <div key={initiative.id} className={styles.initiativeOption}>
              <div><strong>{initiative.title}</strong>{initiative.description ? <p>{initiative.description}</p> : null}</div>
              <Button variant="secondary" size="sm" loading={creatingFor === initiative.id} disabled={Boolean(creatingFor && creatingFor !== initiative.id)} onClick={() => void addToInitiative(initiative.id)}>Aggiungi</Button>
            </div>
          ))}
        </div>
      </Modal>
    </div>
  );
}
