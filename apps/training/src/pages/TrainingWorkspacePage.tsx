import { useMemo, useRef, useState, type ReactNode } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import {
  useCreateEnrollment,
  useCreateAward,
  useCreateCourse,
  useCreateTrainingMasterData,
  useCreateTrainingRequest,
  useDownloadDocument,
  useEnrollmentTransition,
  useRunTrainingJobs,
  useTrainingExport,
  useTrainingLookups,
  useTrainingRequestAction,
  useTrainingWorkspace,
  useUpdateEnrollment,
  useUpdateCourse,
  useUpdateTrainingMasterData,
  useUpdateAward,
  useUploadAwardDocument,
  useUploadEnrollmentDocument,
  useValidateDocument,
  type TrainingCoursePayload,
  type TrainingMasterDataKind,
} from '../api/queries';
import type {
  CatalogCourse,
  CatalogMasterData,
  CertificationRow,
  ComplianceGapRow,
  ExpiringCertificationRow,
  PlanBudgetRow,
  PlanEnrollment,
  TrainingRequest,
} from '../api/types';
import styles from './TrainingWorkspacePage.module.css';

export type TrainingView = 'piano' | 'richieste' | 'catalogo' | 'certificazioni' | 'report';

interface TrainingWorkspacePageProps {
  view: TrainingView;
  isPeopleAdmin: boolean;
}

interface ReasonTransitionTarget {
  row: PlanEnrollment;
  transition: 'revert_to_proposed' | 'reopen';
}

const viewTitles: Record<TrainingView, string> = {
  piano: 'Piano',
  richieste: 'Richieste',
  catalogo: 'Catalogo',
  certificazioni: 'Certificazioni',
  report: 'Report',
};

const viewSubtitles: Record<TrainingView, string> = {
  piano: 'Iscrizioni pianificate, approvate e in corso.',
  richieste: 'Proposte dei dipendenti e coda di valutazione People.',
  catalogo: 'Corsi, fornitori, aree formative e certificazioni collegate.',
  certificazioni: 'Validita, scadenze e attestati collegati alle persone.',
  report: 'Budget, scadenze e formazione obbligatoria da seguire.',
};

const statusLabels: Record<string, string> = {
  proposed: 'Proposta',
  approved: 'Approvata',
  in_progress: 'In corso',
  completed: 'Completata',
  failed: 'Non superata',
  cancelled: 'Annullata',
  expired: 'Scaduta',
  submitted: 'Inviata',
  under_review: 'In valutazione',
  accepted: 'Accettata',
  rejected: 'Respinta',
  converted: 'Convertita',
  passed_exam: 'Esame superato',
  attendance_only: 'Frequenza',
  valid: 'Valida',
  valid_no_expiry: 'Valida senza scadenza',
  not_certified: 'Non certificata',
  missing_or_expired: 'Da pianificare',
  compliant: 'Coperta',
  no_cert_linked: 'Da verificare',
  candidate: 'Pronta',
  skipped: 'Saltata',
  classroom: 'Aula',
  online_live: 'Online live',
  online_self: 'Online autonoma',
  on_the_job: 'Affiancamento',
  mixed: 'Mista',
};

const validationSourceLabels: Record<string, string> = {
  document_verified: 'Attestato verificato',
  declared_survey: 'Dichiarazione survey',
  declared_verbal: 'Dichiarazione verbale',
  declared_cv: 'Dichiarazione CV',
  imported_legacy: 'Storico importato',
};

const catalogCreateLabels: Record<TrainingMasterDataKind, string> = {
  vendors: 'Nuovo fornitore',
  teams: 'Nuovo team',
  'skill-areas': 'Nuova area',
  certifications: 'Nuova certificazione',
  plans: 'Nuovo piano',
  'mandatory-rules': 'Nuova regola',
};

const catalogEditLabels: Record<TrainingMasterDataKind, string> = {
  vendors: 'Modifica fornitore',
  teams: 'Modifica team',
  'skill-areas': 'Modifica area',
  certifications: 'Modifica certificazione',
  plans: 'Modifica piano',
  'mandatory-rules': 'Modifica regola',
};

function label(value: string | undefined): string {
  if (!value) return '';
  return statusLabels[value] ?? value;
}

function formatMoney(value: number | undefined): string {
  if (value === undefined) return '';
  return new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR' }).format(value);
}

function formatDate(value: string | undefined): string {
  if (!value) return '';
  return value.slice(0, 10);
}

function getParam(params: URLSearchParams, key: string): string {
  return params.get(key) ?? '';
}

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

function numberDraft(value: number | undefined): string {
  return value === undefined ? '' : String(value);
}

function optionalNumber(value: string): number | undefined {
  const trimmed = value.trim();
  if (trimmed === '') return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function optionalText(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed === '' ? undefined : trimmed;
}

function uniqueOptions(values: Array<string | undefined>) {
  return [...new Set(values.filter((value): value is string => Boolean(value)))]
    .sort((a, b) => a.localeCompare(b))
    .map((value) => ({ value, label: value }));
}

function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') return body.message;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error;
  }
  return fallback;
}

export function TrainingWorkspacePage({ view, isPeopleAdmin }: TrainingWorkspacePageProps) {
  const query = useTrainingWorkspace(isPeopleAdmin);
  const [params, setParams] = useSearchParams();
  const [confirmTarget, setConfirmTarget] = useState<PlanEnrollment | null>(null);
  const [reasonTransitionTarget, setReasonTransitionTarget] = useState<ReasonTransitionTarget | null>(null);
  const [rejectTarget, setRejectTarget] = useState<TrainingRequest | null>(null);
  const [convertTarget, setConvertTarget] = useState<TrainingRequest | null>(null);
  const [enrollmentModalOpen, setEnrollmentModalOpen] = useState(false);
  const [requestModalOpen, setRequestModalOpen] = useState(false);
  const [courseModalOpen, setCourseModalOpen] = useState(false);
  const [catalogCreateKind, setCatalogCreateKind] = useState<TrainingMasterDataKind | null>(null);
  const [catalogEditTarget, setCatalogEditTarget] = useState<{ kind: TrainingMasterDataKind; id: string } | null>(null);
  const [awardModalOpen, setAwardModalOpen] = useState(false);
  const [runJobsConfirm, setRunJobsConfirm] = useState(false);
  const [correctTarget, setCorrectTarget] = useState<CertificationRow | null>(null);
  const [enrollmentDraft, setEnrollmentDraft] = useState({ employeeId: '', courseId: '', trainingPlanId: '' });
  const [editEnrollmentTarget, setEditEnrollmentTarget] = useState<PlanEnrollment | null>(null);
  const emptyEditEnrollmentDraft = {
    priority: '',
    levelAsIs: '',
    levelToBe: '',
    plannedStart: '',
    plannedEnd: '',
    hoursPlanned: '',
    costPlanned: '',
    motivation: '',
    objective: '',
    notes: '',
  };
  const [editEnrollmentDraft, setEditEnrollmentDraft] = useState(emptyEditEnrollmentDraft);
  const [transitionReason, setTransitionReason] = useState('');
  const [rejectReason, setRejectReason] = useState('');
  const [convertPlanId, setConvertPlanId] = useState('');
  const [requestDraft, setRequestDraft] = useState({ courseId: '', title: '', motivation: '', desiredYear: String(new Date().getFullYear()) });
  const emptyCourseDraft = {
    title: '',
    vendorId: '',
    skillAreaId: '',
    leadsToCertId: '',
    deliveryMode: 'mixed',
    providerKind: 'external',
    defaultHours: '',
    defaultCost: '',
    courseUrl: '',
    description: '',
    recurrenceMonths: '',
    complianceFramework: '',
    mandatory: false,
    active: true,
  };
  const [courseDraft, setCourseDraft] = useState(emptyCourseDraft);
  const [editCourseTarget, setEditCourseTarget] = useState<CatalogCourse | null>(null);
  const [vendorDraft, setVendorDraft] = useState({ name: '', website: '', notes: '', active: true });
  const [teamDraft, setTeamDraft] = useState({ code: '', name: '', description: '', active: true });
  const [skillAreaDraft, setSkillAreaDraft] = useState({ code: '', name: '', parentId: '', description: '', active: true });
  const [certificationDraft, setCertificationDraft] = useState({
    code: '',
    name: '',
    issuerVendorId: '',
    skillAreaId: '',
    typicalValidityMonths: '',
    description: '',
    active: true,
  });
  const [planDraft, setPlanDraft] = useState({ year: String(new Date().getFullYear()), status: 'draft', budgetTotal: '', notes: '' });
  const [mandatoryRuleDraft, setMandatoryRuleDraft] = useState({ courseId: '', teamId: '', roleFilter: '', notes: '', active: true });
  const [awardDraft, setAwardDraft] = useState({ employeeId: '', certificationId: '', outcome: 'passed_exam', awardedOn: todayISO(), expiresOn: '' });
  const [correctDraft, setCorrectDraft] = useState({
    outcome: 'passed_exam',
    awardedOn: todayISO(),
    expiresOn: '',
    validationSource: 'document_verified',
  });
  const [uploadAwardId, setUploadAwardId] = useState<string | null>(null);
  const [uploadEnrollmentId, setUploadEnrollmentId] = useState<string | null>(null);
  const uploadInputRef = useRef<HTMLInputElement>(null);
  const uploadEnrollmentInputRef = useRef<HTMLInputElement>(null);
  const { toast } = useToast();
  const lookups = useTrainingLookups(isPeopleAdmin || awardModalOpen || requestModalOpen);
  const enrollmentTransition = useEnrollmentTransition(isPeopleAdmin);
  const createEnrollment = useCreateEnrollment(isPeopleAdmin);
  const updateEnrollment = useUpdateEnrollment(isPeopleAdmin);
  const createAward = useCreateAward(isPeopleAdmin);
  const updateAward = useUpdateAward(isPeopleAdmin);
  const requestAction = useTrainingRequestAction(isPeopleAdmin);
  const createRequest = useCreateTrainingRequest(isPeopleAdmin);
  const createCourse = useCreateCourse(isPeopleAdmin);
  const updateCourse = useUpdateCourse(isPeopleAdmin);
  const createMasterData = useCreateTrainingMasterData(isPeopleAdmin);
  const updateMasterData = useUpdateTrainingMasterData(isPeopleAdmin);
  const uploadAwardDocument = useUploadAwardDocument(isPeopleAdmin);
  const uploadEnrollmentDocument = useUploadEnrollmentDocument(isPeopleAdmin);
  const validateDocument = useValidateDocument(isPeopleAdmin);
  const downloadDocument = useDownloadDocument();
  const runTrainingJobs = useRunTrainingJobs(isPeopleAdmin);
  const trainingExport = useTrainingExport();

  const workspace = query.data;
  const q = getParam(params, 'q');
  const team = getParam(params, 'team');
  const status = getParam(params, 'status');
  const year = getParam(params, 'year');

  function updateParam(key: string, value: string | null) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (value) next.set(key, value);
      else next.delete(key);
      return next;
    });
  }

  function exportKind(kind: string) {
    trainingExport.mutate(
      { kind, search: params },
      {
        onSuccess: () => toast('Export avviato'),
        onError: (error) => toast(apiErrorMessage(error, 'Export non riuscito'), 'error'),
      },
    );
  }

  function closeCourseModal() {
    setCourseModalOpen(false);
    setEditCourseTarget(null);
    setCourseDraft(emptyCourseDraft);
  }

  function coursePayloadFromDraft(): TrainingCoursePayload {
    return {
      title: courseDraft.title,
      vendorId: courseDraft.vendorId || undefined,
      skillAreaId: courseDraft.skillAreaId || undefined,
      leadsToCertId: courseDraft.leadsToCertId || undefined,
      deliveryMode: courseDraft.deliveryMode,
      providerKind: courseDraft.providerKind,
      defaultHours: optionalNumber(courseDraft.defaultHours),
      defaultCost: optionalNumber(courseDraft.defaultCost),
      courseUrl: optionalText(courseDraft.courseUrl),
      description: optionalText(courseDraft.description),
      mandatory: courseDraft.mandatory,
      recurrenceMonths: optionalNumber(courseDraft.recurrenceMonths),
      complianceFramework: optionalText(courseDraft.complianceFramework),
      active: courseDraft.active,
    };
  }

  function coursePayloadFromRow(row: CatalogCourse, active: boolean): TrainingCoursePayload {
    return {
      title: row.title,
      vendorId: row.vendorId || undefined,
      skillAreaId: row.skillAreaId || undefined,
      leadsToCertId: row.leadsToCertId || undefined,
      deliveryMode: row.deliveryMode || 'mixed',
      providerKind: row.providerKind || 'external',
      defaultHours: row.defaultHours,
      defaultCost: row.defaultCost,
      courseUrl: row.courseUrl || undefined,
      description: row.description || undefined,
      mandatory: row.mandatory,
      recurrenceMonths: row.recurrenceMonths,
      complianceFramework: row.complianceFramework || undefined,
      active,
    };
  }

  function editCourse(course: CatalogCourse) {
    setEditCourseTarget(course);
    setCourseDraft({
      title: course.title,
      vendorId: course.vendorId ?? '',
      skillAreaId: course.skillAreaId ?? '',
      leadsToCertId: course.leadsToCertId ?? '',
      deliveryMode: course.deliveryMode || 'mixed',
      providerKind: course.providerKind || 'external',
      defaultHours: numberDraft(course.defaultHours),
      defaultCost: numberDraft(course.defaultCost),
      courseUrl: course.courseUrl ?? '',
      description: course.description ?? '',
      recurrenceMonths: numberDraft(course.recurrenceMonths),
      complianceFramework: course.complianceFramework ?? '',
      mandatory: course.mandatory,
      active: course.active,
    });
    setCourseModalOpen(true);
  }

  function resetCatalogDraft(kind: TrainingMasterDataKind) {
    switch (kind) {
      case 'vendors':
        setVendorDraft({ name: '', website: '', notes: '', active: true });
        break;
      case 'teams':
        setTeamDraft({ code: '', name: '', description: '', active: true });
        break;
      case 'skill-areas':
        setSkillAreaDraft({ code: '', name: '', parentId: '', description: '', active: true });
        break;
      case 'certifications':
        setCertificationDraft({
          code: '',
          name: '',
          issuerVendorId: '',
          skillAreaId: '',
          typicalValidityMonths: '',
          description: '',
          active: true,
        });
        break;
      case 'plans':
        setPlanDraft({ year: String(new Date().getFullYear()), status: 'draft', budgetTotal: '', notes: '' });
        break;
      case 'mandatory-rules':
        setMandatoryRuleDraft({ courseId: '', teamId: '', roleFilter: '', notes: '', active: true });
        break;
    }
  }

  function closeCatalogCreate() {
    if (catalogCreateKind) resetCatalogDraft(catalogCreateKind);
    setCatalogEditTarget(null);
    setCatalogCreateKind(null);
  }

  function catalogBodyFromDraft(kind: TrainingMasterDataKind): Record<string, unknown> {
    switch (kind) {
      case 'vendors':
        return vendorDraft;
      case 'teams':
        return teamDraft;
      case 'skill-areas':
        return {
          code: skillAreaDraft.code,
          name: skillAreaDraft.name,
          parentId: skillAreaDraft.parentId || undefined,
          description: skillAreaDraft.description,
          active: skillAreaDraft.active,
        };
      case 'certifications':
        return {
          code: certificationDraft.code,
          name: certificationDraft.name,
          issuerVendorId: certificationDraft.issuerVendorId || undefined,
          skillAreaId: certificationDraft.skillAreaId || undefined,
          typicalValidityMonths: optionalNumber(certificationDraft.typicalValidityMonths),
          description: certificationDraft.description,
          active: certificationDraft.active,
        };
      case 'plans':
        return {
          year: Number(planDraft.year),
          status: planDraft.status,
          budgetTotal: optionalNumber(planDraft.budgetTotal),
          notes: planDraft.notes,
        };
      case 'mandatory-rules':
        return {
          courseId: mandatoryRuleDraft.courseId,
          teamId: mandatoryRuleDraft.teamId || undefined,
          roleFilter: mandatoryRuleDraft.roleFilter,
          notes: mandatoryRuleDraft.notes,
          active: mandatoryRuleDraft.active,
        };
    }
  }

  function submitCatalogCreate(kind: TrainingMasterDataKind) {
    const body = catalogBodyFromDraft(kind);
    if (catalogEditTarget?.kind === kind) {
      updateMasterData.mutate(
        { kind, id: catalogEditTarget.id, body },
        {
          onSuccess: () => {
            resetCatalogDraft(kind);
            setCatalogEditTarget(null);
            setCatalogCreateKind(null);
            toast('Voce aggiornata');
          },
          onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento voce non riuscito'), 'error'),
        },
      );
      return;
    }
    createMasterData.mutate(
      { kind, body },
      {
        onSuccess: () => {
          resetCatalogDraft(kind);
          setCatalogEditTarget(null);
          setCatalogCreateKind(null);
          toast('Voce creata');
        },
        onError: (error) => toast(apiErrorMessage(error, 'Creazione voce non riuscita'), 'error'),
      },
    );
  }

  function openCatalogCreate(kind: TrainingMasterDataKind) {
    resetCatalogDraft(kind);
    setCatalogEditTarget(null);
    setCatalogCreateKind(kind);
  }

  function editCatalogData(kind: TrainingMasterDataKind, id: string) {
    const masterData = workspace?.masterData;
    if (!masterData) return;
    switch (kind) {
      case 'vendors': {
        const row = masterData.vendors.find((item) => item.id === id);
        if (!row) return;
        setVendorDraft({ name: row.name, website: row.website ?? '', notes: row.notes ?? '', active: row.active });
        break;
      }
      case 'teams': {
        const row = masterData.teams.find((item) => item.id === id);
        if (!row) return;
        setTeamDraft({ code: row.code, name: row.name, description: row.description ?? '', active: row.active });
        break;
      }
      case 'skill-areas': {
        const row = masterData.skillAreas.find((item) => item.id === id);
        if (!row) return;
        setSkillAreaDraft({ code: row.code, name: row.name, parentId: row.parentId ?? '', description: row.description ?? '', active: row.active });
        break;
      }
      case 'certifications': {
        const row = masterData.certifications.find((item) => item.id === id);
        if (!row) return;
        setCertificationDraft({
          code: row.code,
          name: row.name,
          issuerVendorId: row.issuerVendorId ?? '',
          skillAreaId: row.skillAreaId ?? '',
          typicalValidityMonths: numberDraft(row.typicalValidityMonths),
          description: row.description ?? '',
          active: row.active,
        });
        break;
      }
      case 'plans': {
        const row = masterData.plans.find((item) => item.id === id);
        if (!row) return;
        setPlanDraft({ year: String(row.year), status: row.status, budgetTotal: numberDraft(row.budgetTotal), notes: row.notes ?? '' });
        break;
      }
      case 'mandatory-rules': {
        const row = masterData.mandatoryRules.find((item) => item.id === id);
        if (!row) return;
        setMandatoryRuleDraft({
          courseId: row.courseId,
          teamId: row.teamId ?? '',
          roleFilter: row.roleFilter ?? '',
          notes: row.notes ?? '',
          active: row.active,
        });
        break;
      }
    }
    setCatalogEditTarget({ kind, id });
    setCatalogCreateKind(kind);
  }

  function toggleCatalogData(kind: TrainingMasterDataKind, id: string, active: boolean) {
    const masterData = workspace?.masterData;
    if (!masterData || kind === 'plans') return;
    let body: Record<string, unknown> | null = null;
    switch (kind) {
      case 'vendors': {
        const row = masterData.vendors.find((item) => item.id === id);
        if (row) body = { name: row.name, website: row.website, notes: row.notes, active: !active };
        break;
      }
      case 'teams': {
        const row = masterData.teams.find((item) => item.id === id);
        if (row) body = { code: row.code, name: row.name, description: row.description, active: !active };
        break;
      }
      case 'skill-areas': {
        const row = masterData.skillAreas.find((item) => item.id === id);
        if (row) body = { code: row.code, name: row.name, parentId: row.parentId || undefined, description: row.description, active: !active };
        break;
      }
      case 'certifications': {
        const row = masterData.certifications.find((item) => item.id === id);
        if (row) {
          body = {
            code: row.code,
            name: row.name,
            issuerVendorId: row.issuerVendorId || undefined,
            skillAreaId: row.skillAreaId || undefined,
            typicalValidityMonths: row.typicalValidityMonths,
            description: row.description,
            active: !active,
          };
        }
        break;
      }
      case 'mandatory-rules': {
        const row = masterData.mandatoryRules.find((item) => item.id === id);
        if (row) {
          body = {
            courseId: row.courseId,
            teamId: row.teamId || undefined,
            roleFilter: row.roleFilter,
            notes: row.notes,
            active: !active,
          };
        }
        break;
      }
    }
    if (!body) return;
    updateMasterData.mutate(
      { kind, id, body },
      {
        onSuccess: () => toast(active ? 'Voce disattivata' : 'Voce riattivata'),
        onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento voce non riuscito'), 'error'),
      },
    );
  }

  const filteredPlan = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return (workspace?.plan ?? []).filter((item) => {
      if (team && item.teamCode !== team) return false;
      if (status && item.status !== status) return false;
      if (year && String(item.year) !== year) return false;
      if (!needle) return true;
      return [
        item.employeeName,
        item.employeeEmail,
        item.courseTitle,
        item.vendorName,
        item.skillAreaName,
      ]
        .filter((value): value is string => Boolean(value))
        .some((value) => value.toLowerCase().includes(needle));
    });
  }, [q, status, team, workspace?.plan, year]);

  const filteredRequests = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return (workspace?.requests ?? []).filter((item) => {
      if (status && item.status !== status) return false;
      if (!needle) return true;
      return [
        item.employeeName,
        item.courseTitle,
        item.freeTextTitle,
        item.skillAreaName,
        item.motivation,
      ]
        .filter((value): value is string => Boolean(value))
        .some((value) => value.toLowerCase().includes(needle));
    });
  }, [q, status, workspace?.requests]);

  const filteredCatalog = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return (workspace?.catalog ?? []).filter((item) => {
      if (status === 'mandatory' && !item.mandatory) return false;
      if (status === 'active' && !item.active) return false;
      if (!needle) return true;
      return [
        item.title,
        item.vendorName,
        item.skillAreaName,
        item.certificationName,
        item.complianceFramework,
      ]
        .filter((value): value is string => Boolean(value))
        .some((value) => value.toLowerCase().includes(needle));
    });
  }, [q, status, workspace?.catalog]);

  const filteredCertifications = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return (workspace?.certifications ?? []).filter((item) => {
      if (status && item.currentStatus !== status) return false;
      if (year && !item.awardedOn.startsWith(year)) return false;
      if (!needle) return true;
      return [
        item.employeeName,
        item.employeeEmail,
        item.certificationCode,
        item.certificationName,
        item.outcome,
        item.validationSource,
        item.documentFilename,
      ]
        .filter((value): value is string => Boolean(value))
        .some((value) => value.toLowerCase().includes(needle));
    });
  }, [q, status, workspace?.certifications, year]);

  if (query.isLoading) {
    return (
      <main className={styles.page}>
        <PageHeader view={view} isPeopleAdmin={isPeopleAdmin} />
        <Skeleton rows={8} />
      </main>
    );
  }

  if (query.error) {
    return (
      <main className={styles.page}>
        <PageHeader view={view} isPeopleAdmin={isPeopleAdmin} />
        <StateBlock
          tone="warning"
          title="Dati non disponibili"
          text="La console Formazione non puo caricare le informazioni in questo momento."
        />
      </main>
    );
  }

  if (workspace?.me.onboardingPending) {
    return (
      <main className={styles.page}>
        <PageHeader view={view} isPeopleAdmin={isPeopleAdmin} />
        <StateBlock
          tone="info"
          title="Profilo HR in preparazione"
          text="Il tuo profilo deve essere completato da People prima di mostrare piano, richieste e certificazioni."
        />
      </main>
    );
  }

  return (
    <main className={styles.page}>
      <PageHeader view={view} isPeopleAdmin={isPeopleAdmin} />

      {view === 'piano' && (
        <PlanView
          rows={filteredPlan}
          allRows={workspace?.plan ?? []}
          q={q}
          team={team}
          status={status}
          year={year}
          isPeopleAdmin={isPeopleAdmin}
          actionsEnabled={
            !enrollmentTransition.isPending &&
            !createEnrollment.isPending &&
            !updateEnrollment.isPending &&
            !uploadEnrollmentDocument.isPending &&
            !validateDocument.isPending &&
            !downloadDocument.isPending &&
            !trainingExport.isPending
          }
          onSearch={(value) => updateParam('q', value)}
          onTeam={(value) => updateParam('team', value)}
          onStatus={(value) => updateParam('status', value)}
          onYear={(value) => updateParam('year', value)}
          onConfirm={setConfirmTarget}
          onReasonTransition={(row, transition) => {
            setReasonTransitionTarget({ row, transition });
            setTransitionReason('');
          }}
          onDownloadDocument={(row) => {
            if (!row.documentId) return;
            downloadDocument.mutate(
              { documentId: row.documentId, filename: row.documentFilename || 'documento-formazione.pdf' },
              { onError: (error) => toast(apiErrorMessage(error, 'Download documento non riuscito'), 'error') },
            );
          }}
          onEdit={(row) => {
            setEditEnrollmentTarget(row);
            setEditEnrollmentDraft({
              priority: numberDraft(row.priority),
              levelAsIs: numberDraft(row.levelAsIs),
              levelToBe: numberDraft(row.levelToBe),
              plannedStart: formatDate(row.plannedStart),
              plannedEnd: formatDate(row.plannedEnd),
              hoursPlanned: numberDraft(row.hoursPlanned),
              costPlanned: numberDraft(row.costPlanned),
              motivation: row.motivation ?? '',
              objective: row.objective ?? '',
              notes: row.notes ?? '',
            });
          }}
          onValidateDocument={(documentId) => {
            validateDocument.mutate(
              documentId,
              {
                onSuccess: () => toast('Documento validato'),
                onError: (error) => toast(apiErrorMessage(error, 'Validazione documento non riuscita'), 'error'),
              },
            );
          }}
          onUploadDocument={(enrollmentId) => {
            setUploadEnrollmentId(enrollmentId);
            uploadEnrollmentInputRef.current?.click();
          }}
          onTransition={(id, transition) => {
            enrollmentTransition.mutate(
              { id, transition },
              {
                onSuccess: () => toast('Piano aggiornato'),
                onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento piano non riuscito'), 'error'),
              },
            );
          }}
          onExport={() => exportKind('plan')}
          onNew={() => setEnrollmentModalOpen(true)}
        />
      )}

      {view === 'richieste' && (
        <RequestsView
          rows={filteredRequests}
          allRows={workspace?.requests ?? []}
          q={q}
          status={status}
          isPeopleAdmin={isPeopleAdmin}
          actionsEnabled={!requestAction.isPending && !createRequest.isPending && !trainingExport.isPending}
          onSearch={(value) => updateParam('q', value)}
          onStatus={(value) => updateParam('status', value)}
          onNew={() => setRequestModalOpen(true)}
          onExport={() => exportKind('requests')}
          onAction={(id, transition) => {
            requestAction.mutate(
              { id, transition },
              {
                onSuccess: () => toast('Richiesta aggiornata'),
                onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento richiesta non riuscito'), 'error'),
              },
            );
          }}
          onReject={(row) => {
            setRejectTarget(row);
            setRejectReason('');
          }}
          onConvert={(row) => {
            setConvertTarget(row);
            setConvertPlanId('');
          }}
        />
      )}

      {view === 'catalogo' && (
        <CatalogView
          rows={filteredCatalog}
          q={q}
          status={status}
          isPeopleAdmin={isPeopleAdmin}
          actionsEnabled={!createCourse.isPending && !updateCourse.isPending && !createMasterData.isPending && !updateMasterData.isPending && !trainingExport.isPending}
          masterData={workspace?.masterData}
          onSearch={(value) => updateParam('q', value)}
          onStatus={(value) => updateParam('status', value)}
          onExport={() => exportKind('catalog')}
          onNew={() => {
            setEditCourseTarget(null);
            setCourseDraft(emptyCourseDraft);
            setCourseModalOpen(true);
          }}
          onEditCourse={editCourse}
          onToggleCourse={(course) => {
            updateCourse.mutate(
              { id: course.id, body: coursePayloadFromRow(course, !course.active) },
              {
                onSuccess: () => toast(course.active ? 'Corso disattivato' : 'Corso riattivato'),
                onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento corso non riuscito'), 'error'),
              },
            );
          }}
          onNewData={openCatalogCreate}
          onEditData={editCatalogData}
          onToggleData={toggleCatalogData}
        />
      )}

      {view === 'certificazioni' && (
        <CertificationsView
          rows={filteredCertifications}
          allRows={workspace?.certifications ?? []}
          q={q}
          status={status}
          year={year}
          isPeopleAdmin={isPeopleAdmin}
          actionsEnabled={
            !uploadAwardDocument.isPending &&
            !createAward.isPending &&
            !updateAward.isPending &&
            !trainingExport.isPending &&
            !validateDocument.isPending &&
            !downloadDocument.isPending
          }
          onSearch={(value) => updateParam('q', value)}
          onStatus={(value) => updateParam('status', value)}
          onYear={(value) => updateParam('year', value)}
          onNew={() => setAwardModalOpen(true)}
          onExport={() => exportKind('certifications')}
          onCorrect={(row) => {
            setCorrectTarget(row);
            setCorrectDraft({
              outcome: row.outcome,
              awardedOn: formatDate(row.awardedOn) || todayISO(),
              expiresOn: formatDate(row.expiresOn),
              validationSource: row.validationSource || 'document_verified',
            });
          }}
          onDownload={(row) => {
            if (!row.documentId) return;
            downloadDocument.mutate(
              { documentId: row.documentId, filename: row.documentFilename || 'attestato.pdf' },
              { onError: (error) => toast(apiErrorMessage(error, 'Download attestato non riuscito'), 'error') },
            );
          }}
          onValidate={(documentId) => {
            validateDocument.mutate(
              documentId,
              {
                onSuccess: () => toast('Attestato validato'),
                onError: (error) => toast(apiErrorMessage(error, 'Validazione attestato non riuscita'), 'error'),
              },
            );
          }}
          onUpload={(awardId) => {
            setUploadAwardId(awardId);
            uploadInputRef.current?.click();
          }}
        />
      )}

      {view === 'report' && (
        <ReportsView
          planBudget={workspace?.planBudget ?? []}
          expiring={workspace?.expiringCertifications ?? []}
          gaps={workspace?.mandatoryComplianceGaps ?? []}
          isPeopleAdmin={isPeopleAdmin}
          actionsEnabled={!trainingExport.isPending && !runTrainingJobs.isPending}
          onExport={exportKind}
          onRunJobs={() => setRunJobsConfirm(true)}
        />
      )}

      <input
        ref={uploadInputRef}
        type="file"
        accept="application/pdf,.pdf"
        hidden
        onChange={(event) => {
          const file = event.currentTarget.files?.[0];
          event.currentTarget.value = '';
          if (!file || !uploadAwardId) return;
          uploadAwardDocument.mutate(
            { awardId: uploadAwardId, file },
            {
              onSuccess: () => toast('Attestato caricato'),
              onError: (error) => toast(apiErrorMessage(error, 'Caricamento attestato non riuscito'), 'error'),
              onSettled: () => setUploadAwardId(null),
            },
          );
        }}
      />

      <input
        ref={uploadEnrollmentInputRef}
        type="file"
        accept="application/pdf,.pdf"
        hidden
        onChange={(event) => {
          const file = event.currentTarget.files?.[0];
          event.currentTarget.value = '';
          if (!file || !uploadEnrollmentId) return;
          uploadEnrollmentDocument.mutate(
            { enrollmentId: uploadEnrollmentId, file },
            {
              onSuccess: () => toast('Documento caricato'),
              onError: (error) => toast(apiErrorMessage(error, 'Caricamento documento non riuscito'), 'error'),
              onSettled: () => setUploadEnrollmentId(null),
            },
          );
        }}
      />

      <Modal
        open={confirmTarget !== null}
        onClose={() => setConfirmTarget(null)}
        title="Conferma annullamento"
        size="sm"
      >
        <div className={styles.modalBody}>
          <p>
            Vuoi annullare l&apos;iscrizione a {confirmTarget?.courseTitle}? L&apos;azione resta
            visibile nello storico.
          </p>
          <div className={styles.modalActions}>
            <Button variant="secondary" onClick={() => setConfirmTarget(null)}>
              Mantieni
            </Button>
            <Button
              variant="danger"
              onClick={() => {
                if (confirmTarget) {
                  enrollmentTransition.mutate(
                    { id: confirmTarget.id, transition: 'cancel', reason: 'Annullamento confermato da People' },
                    {
                      onSuccess: () => toast('Iscrizione annullata'),
                      onError: (error) => toast(apiErrorMessage(error, 'Annullamento iscrizione non riuscito'), 'error'),
                    },
                  );
                }
                setConfirmTarget(null);
              }}
            >
              Annulla iscrizione
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={reasonTransitionTarget !== null}
        onClose={() => {
          setReasonTransitionTarget(null);
          setTransitionReason('');
        }}
        title={reasonTransitionTarget?.transition === 'reopen' ? 'Riapri iscrizione' : 'Riporta a proposta'}
        size="sm"
      >
        <form
          className={styles.formStack}
          onSubmit={(event) => {
            event.preventDefault();
            const reason = transitionReason.trim();
            if (!reasonTransitionTarget || reason.length < 3) {
              toast('Indica un motivo valido', 'warning');
              return;
            }
            enrollmentTransition.mutate(
              { id: reasonTransitionTarget.row.id, transition: reasonTransitionTarget.transition, reason },
              {
                onSuccess: () => {
                  setReasonTransitionTarget(null);
                  setTransitionReason('');
                  toast('Piano aggiornato');
                },
                onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento piano non riuscito'), 'error'),
              },
            );
          }}
        >
          <p className={styles.modalText}>
            {reasonTransitionTarget?.row.employeeName} - {reasonTransitionTarget?.row.courseTitle}
          </p>
          <label>
            <span>Motivo</span>
            <textarea
              value={transitionReason}
              onChange={(event) => setTransitionReason(event.target.value)}
              minLength={3}
              required
            />
          </label>
          <div className={styles.modalActions}>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setReasonTransitionTarget(null);
                setTransitionReason('');
              }}
            >
              Annulla
            </Button>
            <Button type="submit" disabled={enrollmentTransition.isPending || transitionReason.trim().length < 3}>
              Conferma
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={rejectTarget !== null}
        onClose={() => {
          setRejectTarget(null);
          setRejectReason('');
        }}
        title="Respingi richiesta"
        size="sm"
      >
        <form
          className={styles.formStack}
          onSubmit={(event) => {
            event.preventDefault();
            const reason = rejectReason.trim();
            if (!rejectTarget || reason.length < 3) {
              toast('Indica un motivo valido', 'warning');
              return;
            }
            requestAction.mutate(
              { id: rejectTarget.id, transition: 'reject', reason },
              {
                onSuccess: () => {
                  setRejectTarget(null);
                  setRejectReason('');
                  toast('Richiesta respinta');
                },
                onError: (error) => toast(apiErrorMessage(error, 'Rifiuto richiesta non riuscito'), 'error'),
              },
            );
          }}
        >
          <p className={styles.modalText}>
            {rejectTarget?.courseTitle || rejectTarget?.freeTextTitle}
          </p>
          <label>
            <span>Motivo</span>
            <textarea
              value={rejectReason}
              onChange={(event) => setRejectReason(event.target.value)}
              minLength={3}
              required
            />
          </label>
          <div className={styles.modalActions}>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setRejectTarget(null);
                setRejectReason('');
              }}
            >
              Annulla
            </Button>
            <Button type="submit" variant="danger" disabled={requestAction.isPending || rejectReason.trim().length < 3}>
              Respingi
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={runJobsConfirm}
        onClose={() => setRunJobsConfirm(false)}
        title="Esegui controlli"
        size="sm"
      >
        <div className={styles.modalBody}>
          <p>
            Vuoi aggiornare piani chiusi, formazione obbligatoria e scadenze certificazioni?
            Le notifiche previste verranno preparate.
          </p>
          <div className={styles.modalActions}>
            <Button variant="secondary" onClick={() => setRunJobsConfirm(false)}>
              Annulla
            </Button>
            <Button
              onClick={() => {
                runTrainingJobs.mutate(undefined, {
                  onSuccess: (result) => {
                    setRunJobsConfirm(false);
                    toast(`${result.complianceNotifications + result.certificationNotifications} notifiche preparate`);
                  },
                  onError: (error) => toast(apiErrorMessage(error, 'Esecuzione controlli non riuscita'), 'error'),
                });
              }}
              disabled={runTrainingJobs.isPending}
            >
              Esegui
            </Button>
          </div>
        </div>
      </Modal>

      <Modal open={enrollmentModalOpen} onClose={() => setEnrollmentModalOpen(false)} title="Nuova iscrizione" size="md">
        <form
          className={styles.formStack}
          onSubmit={(event) => {
            event.preventDefault();
            createEnrollment.mutate(enrollmentDraft, {
              onSuccess: () => {
                setEnrollmentModalOpen(false);
                setEnrollmentDraft({ employeeId: '', courseId: '', trainingPlanId: '' });
                toast('Iscrizione creata');
              },
              onError: (error) => toast(apiErrorMessage(error, 'Creazione iscrizione non riuscita'), 'error'),
            });
          }}
        >
          <label>
            <span>Persona</span>
            <select value={enrollmentDraft.employeeId} onChange={(event) => setEnrollmentDraft((draft) => ({ ...draft, employeeId: event.target.value }))} required>
              <option value="">Seleziona</option>
              {(lookups.data?.employees ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </select>
          </label>
          <label>
            <span>Corso</span>
            <select value={enrollmentDraft.courseId} onChange={(event) => setEnrollmentDraft((draft) => ({ ...draft, courseId: event.target.value }))} required>
              <option value="">Seleziona</option>
              {(lookups.data?.courses ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </select>
          </label>
          <label>
            <span>Piano</span>
            <select value={enrollmentDraft.trainingPlanId} onChange={(event) => setEnrollmentDraft((draft) => ({ ...draft, trainingPlanId: event.target.value }))} required>
              <option value="">Seleziona</option>
              {(lookups.data?.plans ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </select>
          </label>
          <div className={styles.modalActions}>
            <Button type="button" variant="secondary" onClick={() => setEnrollmentModalOpen(false)}>Annulla</Button>
            <Button type="submit" disabled={createEnrollment.isPending || lookups.isLoading}>Crea</Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={editEnrollmentTarget !== null}
        onClose={() => {
          setEditEnrollmentTarget(null);
          setEditEnrollmentDraft(emptyEditEnrollmentDraft);
        }}
        title="Modifica iscrizione"
        size="xwide"
      >
        <form
          className={`${styles.formStack} ${styles.modalScroll} ${styles.editEnrollmentForm}`}
          onSubmit={(event) => {
            event.preventDefault();
            if (!editEnrollmentTarget) return;
            updateEnrollment.mutate(
              {
                id: editEnrollmentTarget.id,
                body: {
                  priority: optionalNumber(editEnrollmentDraft.priority),
                  levelAsIs: optionalNumber(editEnrollmentDraft.levelAsIs),
                  levelToBe: optionalNumber(editEnrollmentDraft.levelToBe),
                  plannedStart: editEnrollmentDraft.plannedStart,
                  plannedEnd: editEnrollmentDraft.plannedEnd,
                  hoursPlanned: optionalNumber(editEnrollmentDraft.hoursPlanned),
                  costPlanned: optionalNumber(editEnrollmentDraft.costPlanned),
                  motivation: editEnrollmentDraft.motivation,
                  objective: editEnrollmentDraft.objective,
                  notes: editEnrollmentDraft.notes,
                },
              },
              {
                onSuccess: () => {
                  setEditEnrollmentTarget(null);
                  setEditEnrollmentDraft(emptyEditEnrollmentDraft);
                  toast('Iscrizione aggiornata');
                },
                onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento iscrizione non riuscito'), 'error'),
              },
            );
          }}
        >
          <p className={styles.modalText}>
            {editEnrollmentTarget?.employeeName} - {editEnrollmentTarget?.courseTitle}
          </p>
          <div className={`${styles.formGrid} ${styles.editEnrollmentGrid}`}>
            <label>
              <span>Priorita</span>
              <input type="number" min="1" value={editEnrollmentDraft.priority} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, priority: event.target.value }))} inputMode="numeric" />
            </label>
            <label>
              <span>Livello attuale</span>
              <input type="number" min="0" value={editEnrollmentDraft.levelAsIs} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, levelAsIs: event.target.value }))} inputMode="numeric" />
            </label>
            <label>
              <span>Livello obiettivo</span>
              <input type="number" min="0" value={editEnrollmentDraft.levelToBe} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, levelToBe: event.target.value }))} inputMode="numeric" />
            </label>
            <label>
              <span>Ore</span>
              <input type="number" min="1" value={editEnrollmentDraft.hoursPlanned} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, hoursPlanned: event.target.value }))} inputMode="numeric" />
            </label>
            <label>
              <span>Inizio</span>
              <input type="date" value={editEnrollmentDraft.plannedStart} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, plannedStart: event.target.value }))} />
            </label>
            <label>
              <span>Fine</span>
              <input type="date" value={editEnrollmentDraft.plannedEnd} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, plannedEnd: event.target.value }))} />
            </label>
            <label>
              <span>Costo</span>
              <input type="number" min="0" step="0.01" value={editEnrollmentDraft.costPlanned} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, costPlanned: event.target.value }))} inputMode="decimal" />
            </label>
          </div>
          <div className={styles.editEnrollmentTextGrid}>
            <label>
              <span>Motivazione</span>
              <textarea value={editEnrollmentDraft.motivation} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, motivation: event.target.value }))} />
            </label>
            <label>
              <span>Obiettivo</span>
              <textarea value={editEnrollmentDraft.objective} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, objective: event.target.value }))} />
            </label>
            <label>
              <span>Note</span>
              <textarea value={editEnrollmentDraft.notes} onChange={(event) => setEditEnrollmentDraft((draft) => ({ ...draft, notes: event.target.value }))} />
            </label>
          </div>
          <div className={styles.modalActions}>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setEditEnrollmentTarget(null);
                setEditEnrollmentDraft(emptyEditEnrollmentDraft);
              }}
            >
              Annulla
            </Button>
            <Button type="submit" disabled={updateEnrollment.isPending}>
              Salva
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={convertTarget !== null}
        onClose={() => {
          setConvertTarget(null);
          setConvertPlanId('');
        }}
        title="Converti richiesta"
        size="sm"
      >
        <form
          className={styles.formStack}
          onSubmit={(event) => {
            event.preventDefault();
            if (!convertTarget || !convertPlanId) {
              toast('Seleziona un piano', 'warning');
              return;
            }
            requestAction.mutate(
              { id: convertTarget.id, transition: 'convert', trainingPlanId: convertPlanId },
              {
                onSuccess: () => {
                  setConvertTarget(null);
                  setConvertPlanId('');
                  toast('Richiesta convertita');
                },
                onError: (error) => toast(apiErrorMessage(error, 'Conversione richiesta non riuscita'), 'error'),
              },
            );
          }}
        >
          <p className={styles.modalText}>
            {convertTarget?.courseTitle}
          </p>
          <label>
            <span>Piano</span>
            <select value={convertPlanId} onChange={(event) => setConvertPlanId(event.target.value)} required>
              <option value="">Seleziona</option>
              {(lookups.data?.plans ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </select>
          </label>
          <div className={styles.modalActions}>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setConvertTarget(null);
                setConvertPlanId('');
              }}
            >
              Annulla
            </Button>
            <Button type="submit" disabled={requestAction.isPending || !convertPlanId || lookups.isLoading}>
              Converti
            </Button>
          </div>
        </form>
      </Modal>

      <Modal open={requestModalOpen} onClose={() => setRequestModalOpen(false)} title="Nuova richiesta" size="md">
        <form
          className={styles.formStack}
          onSubmit={(event) => {
            event.preventDefault();
            if (!requestDraft.courseId && !requestDraft.title.trim()) {
              toast('Scegli un corso o indica un titolo', 'warning');
              return;
            }
            createRequest.mutate(
              {
                courseId: requestDraft.courseId || undefined,
                freeTextTitle: requestDraft.courseId ? undefined : requestDraft.title,
                motivation: requestDraft.motivation,
                desiredYear: Number(requestDraft.desiredYear) || undefined,
              },
              {
                onSuccess: () => {
                  setRequestModalOpen(false);
                  setRequestDraft({ courseId: '', title: '', motivation: '', desiredYear: String(new Date().getFullYear()) });
                  toast('Richiesta inviata');
                },
                onError: (error) => toast(apiErrorMessage(error, 'Invio richiesta non riuscito'), 'error'),
              },
            );
          }}
        >
          <label>
            <span>Corso</span>
            <select value={requestDraft.courseId} onChange={(event) => setRequestDraft((draft) => ({ ...draft, courseId: event.target.value }))}>
              <option value="">Titolo libero</option>
              {(lookups.data?.courses ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </select>
          </label>
          <label>
            <span>Titolo</span>
            <input value={requestDraft.title} onChange={(event) => setRequestDraft((draft) => ({ ...draft, title: event.target.value }))} disabled={Boolean(requestDraft.courseId)} required={!requestDraft.courseId} />
          </label>
          <label>
            <span>Motivazione</span>
            <textarea value={requestDraft.motivation} onChange={(event) => setRequestDraft((draft) => ({ ...draft, motivation: event.target.value }))} required />
          </label>
          <label>
            <span>Anno</span>
            <input type="number" min="2020" max="2100" value={requestDraft.desiredYear} onChange={(event) => setRequestDraft((draft) => ({ ...draft, desiredYear: event.target.value }))} inputMode="numeric" />
          </label>
          <div className={styles.modalActions}>
            <Button type="button" variant="secondary" onClick={() => setRequestModalOpen(false)}>Annulla</Button>
            <Button type="submit" disabled={createRequest.isPending}>Invia</Button>
          </div>
        </form>
      </Modal>

      <Modal open={courseModalOpen} onClose={closeCourseModal} title={editCourseTarget ? 'Modifica corso' : 'Nuovo corso'} size="wide">
        <form
          className={`${styles.formStack} ${styles.modalScroll}`}
          onSubmit={(event) => {
            event.preventDefault();
            const body = coursePayloadFromDraft();
            if (editCourseTarget) {
              updateCourse.mutate(
                { id: editCourseTarget.id, body },
                {
                  onSuccess: () => {
                    closeCourseModal();
                    toast('Corso aggiornato');
                  },
                  onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento corso non riuscito'), 'error'),
                },
              );
              return;
            }
            createCourse.mutate(body, {
              onSuccess: () => {
                closeCourseModal();
                toast('Corso creato');
              },
              onError: (error) => toast(apiErrorMessage(error, 'Creazione corso non riuscita'), 'error'),
            });
          }}
        >
          <label>
            <span>Titolo</span>
            <input value={courseDraft.title} onChange={(event) => setCourseDraft((draft) => ({ ...draft, title: event.target.value }))} required />
          </label>
          <div className={styles.formGrid}>
            <label>
              <span>Fornitore</span>
              <select value={courseDraft.vendorId} onChange={(event) => setCourseDraft((draft) => ({ ...draft, vendorId: event.target.value }))}>
                <option value="">Da assegnare</option>
                {(lookups.data?.vendors ?? []).filter((item) => item.active || item.id === courseDraft.vendorId).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
              </select>
            </label>
            <label>
              <span>Area</span>
              <select value={courseDraft.skillAreaId} onChange={(event) => setCourseDraft((draft) => ({ ...draft, skillAreaId: event.target.value }))}>
                <option value="">Da assegnare</option>
                {(lookups.data?.skillAreas ?? []).filter((item) => item.active || item.id === courseDraft.skillAreaId).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
              </select>
            </label>
          </div>
          <div className={styles.formGrid}>
            <label>
              <span>Certificazione collegata</span>
              <select value={courseDraft.leadsToCertId} onChange={(event) => setCourseDraft((draft) => ({ ...draft, leadsToCertId: event.target.value }))}>
                <option value="">Non collegata</option>
                {(lookups.data?.certifications ?? []).filter((item) => item.active || item.id === courseDraft.leadsToCertId).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
              </select>
            </label>
            <label>
              <span>Pagina corso</span>
              <input type="url" value={courseDraft.courseUrl} onChange={(event) => setCourseDraft((draft) => ({ ...draft, courseUrl: event.target.value }))} />
            </label>
          </div>
          <div className={styles.formGrid}>
            <label>
              <span>Modalita</span>
              <select value={courseDraft.deliveryMode} onChange={(event) => setCourseDraft((draft) => ({ ...draft, deliveryMode: event.target.value }))}>
                <option value="mixed">Mista</option>
                <option value="classroom">Aula</option>
                <option value="online_live">Online live</option>
                <option value="online_self">Online autonoma</option>
                <option value="on_the_job">Affiancamento</option>
              </select>
            </label>
            <label>
              <span>Origine</span>
              <select value={courseDraft.providerKind} onChange={(event) => setCourseDraft((draft) => ({ ...draft, providerKind: event.target.value }))}>
                <option value="external">Esterna</option>
                <option value="internal">Interna</option>
              </select>
            </label>
          </div>
          <div className={styles.formGrid}>
            <label>
              <span>Ore</span>
              <input type="number" min="1" value={courseDraft.defaultHours} onChange={(event) => setCourseDraft((draft) => ({ ...draft, defaultHours: event.target.value }))} inputMode="numeric" />
            </label>
            <label>
              <span>Costo</span>
              <input type="number" min="0" step="0.01" value={courseDraft.defaultCost} onChange={(event) => setCourseDraft((draft) => ({ ...draft, defaultCost: event.target.value }))} inputMode="decimal" />
            </label>
          </div>
          <label>
            <span>Descrizione</span>
            <textarea value={courseDraft.description} onChange={(event) => setCourseDraft((draft) => ({ ...draft, description: event.target.value }))} />
          </label>
          <div className={styles.formGrid}>
            <label>
              <span>Ambito obbligatorio</span>
              <input value={courseDraft.complianceFramework} onChange={(event) => setCourseDraft((draft) => ({ ...draft, complianceFramework: event.target.value }))} />
            </label>
            <label>
              <span>Ricorrenza mesi</span>
              <input type="number" min="1" value={courseDraft.recurrenceMonths} onChange={(event) => setCourseDraft((draft) => ({ ...draft, recurrenceMonths: event.target.value }))} inputMode="numeric" />
            </label>
          </div>
          <label className={styles.checkboxRow}>
            <input type="checkbox" checked={courseDraft.mandatory} onChange={(event) => setCourseDraft((draft) => ({ ...draft, mandatory: event.target.checked }))} />
            <span>Obbligatorio</span>
          </label>
          <ActiveCheckbox checked={courseDraft.active} onChange={(active) => setCourseDraft((draft) => ({ ...draft, active }))} />
          <div className={styles.modalActions}>
            <Button type="button" variant="secondary" onClick={closeCourseModal}>Annulla</Button>
            <Button type="submit" disabled={createCourse.isPending || updateCourse.isPending}>
              {editCourseTarget ? 'Salva' : 'Crea'}
            </Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={catalogCreateKind !== null}
        onClose={closeCatalogCreate}
        title={catalogCreateKind ? (catalogEditTarget ? catalogEditLabels[catalogCreateKind] : catalogCreateLabels[catalogCreateKind]) : 'Nuova voce'}
        size="md"
      >
        <form
          className={styles.formStack}
          onSubmit={(event) => {
            event.preventDefault();
            if (catalogCreateKind) submitCatalogCreate(catalogCreateKind);
          }}
        >
          {catalogCreateKind === 'vendors' && (
            <>
              <label>
                <span>Nome</span>
                <input value={vendorDraft.name} onChange={(event) => setVendorDraft((draft) => ({ ...draft, name: event.target.value }))} required />
              </label>
              <label>
                <span>Sito</span>
                <input type="url" value={vendorDraft.website} onChange={(event) => setVendorDraft((draft) => ({ ...draft, website: event.target.value }))} />
              </label>
              <label>
                <span>Note</span>
                <textarea value={vendorDraft.notes} onChange={(event) => setVendorDraft((draft) => ({ ...draft, notes: event.target.value }))} />
              </label>
              <ActiveCheckbox checked={vendorDraft.active} onChange={(active) => setVendorDraft((draft) => ({ ...draft, active }))} />
            </>
          )}

          {catalogCreateKind === 'teams' && (
            <>
              <div className={styles.formGrid}>
                <label>
                  <span>Codice</span>
                  <input value={teamDraft.code} onChange={(event) => setTeamDraft((draft) => ({ ...draft, code: event.target.value }))} required />
                </label>
                <label>
                  <span>Nome</span>
                  <input value={teamDraft.name} onChange={(event) => setTeamDraft((draft) => ({ ...draft, name: event.target.value }))} required />
                </label>
              </div>
              <label>
                <span>Descrizione</span>
                <textarea value={teamDraft.description} onChange={(event) => setTeamDraft((draft) => ({ ...draft, description: event.target.value }))} />
              </label>
              <ActiveCheckbox checked={teamDraft.active} onChange={(active) => setTeamDraft((draft) => ({ ...draft, active }))} />
            </>
          )}

          {catalogCreateKind === 'skill-areas' && (
            <>
              <div className={styles.formGrid}>
                <label>
                  <span>Codice</span>
                  <input value={skillAreaDraft.code} onChange={(event) => setSkillAreaDraft((draft) => ({ ...draft, code: event.target.value }))} required />
                </label>
                <label>
                  <span>Nome</span>
                  <input value={skillAreaDraft.name} onChange={(event) => setSkillAreaDraft((draft) => ({ ...draft, name: event.target.value }))} required />
                </label>
              </div>
              <label>
                <span>Area superiore</span>
                <select value={skillAreaDraft.parentId} onChange={(event) => setSkillAreaDraft((draft) => ({ ...draft, parentId: event.target.value }))}>
                  <option value="">Nessuna</option>
                  {(lookups.data?.skillAreas ?? []).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
                </select>
              </label>
              <label>
                <span>Descrizione</span>
                <textarea value={skillAreaDraft.description} onChange={(event) => setSkillAreaDraft((draft) => ({ ...draft, description: event.target.value }))} />
              </label>
              <ActiveCheckbox checked={skillAreaDraft.active} onChange={(active) => setSkillAreaDraft((draft) => ({ ...draft, active }))} />
            </>
          )}

          {catalogCreateKind === 'certifications' && (
            <>
              <div className={styles.formGrid}>
                <label>
                  <span>Codice</span>
                  <input value={certificationDraft.code} onChange={(event) => setCertificationDraft((draft) => ({ ...draft, code: event.target.value }))} required />
                </label>
                <label>
                  <span>Nome</span>
                  <input value={certificationDraft.name} onChange={(event) => setCertificationDraft((draft) => ({ ...draft, name: event.target.value }))} required />
                </label>
              </div>
              <label>
                <span>Ente</span>
                <select value={certificationDraft.issuerVendorId} onChange={(event) => setCertificationDraft((draft) => ({ ...draft, issuerVendorId: event.target.value }))}>
                  <option value="">Da assegnare</option>
                  {(lookups.data?.vendors ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
                </select>
              </label>
              <label>
                <span>Area</span>
                <select value={certificationDraft.skillAreaId} onChange={(event) => setCertificationDraft((draft) => ({ ...draft, skillAreaId: event.target.value }))}>
                  <option value="">Da assegnare</option>
                  {(lookups.data?.skillAreas ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
                </select>
              </label>
              <label>
                <span>Validita mesi</span>
                <input type="number" min="1" value={certificationDraft.typicalValidityMonths} onChange={(event) => setCertificationDraft((draft) => ({ ...draft, typicalValidityMonths: event.target.value }))} inputMode="numeric" />
              </label>
              <label>
                <span>Descrizione</span>
                <textarea value={certificationDraft.description} onChange={(event) => setCertificationDraft((draft) => ({ ...draft, description: event.target.value }))} />
              </label>
              <ActiveCheckbox checked={certificationDraft.active} onChange={(active) => setCertificationDraft((draft) => ({ ...draft, active }))} />
            </>
          )}

          {catalogCreateKind === 'plans' && (
            <>
              <div className={styles.formGrid}>
                <label>
                  <span>Anno</span>
                  <input type="number" min="2020" max="2100" value={planDraft.year} onChange={(event) => setPlanDraft((draft) => ({ ...draft, year: event.target.value }))} inputMode="numeric" required />
                </label>
                <label>
                  <span>Stato</span>
                  <select value={planDraft.status} onChange={(event) => setPlanDraft((draft) => ({ ...draft, status: event.target.value }))}>
                    <option value="draft">Bozza</option>
                    <option value="open">Aperto</option>
                    <option value="frozen">Congelato</option>
                    <option value="closed">Chiuso</option>
                  </select>
                </label>
              </div>
              <label>
                <span>Budget</span>
                <input type="number" min="0" step="0.01" value={planDraft.budgetTotal} onChange={(event) => setPlanDraft((draft) => ({ ...draft, budgetTotal: event.target.value }))} inputMode="decimal" />
              </label>
              <label>
                <span>Note</span>
                <textarea value={planDraft.notes} onChange={(event) => setPlanDraft((draft) => ({ ...draft, notes: event.target.value }))} />
              </label>
            </>
          )}

          {catalogCreateKind === 'mandatory-rules' && (
            <>
              <label>
                <span>Corso</span>
                <select value={mandatoryRuleDraft.courseId} onChange={(event) => setMandatoryRuleDraft((draft) => ({ ...draft, courseId: event.target.value }))} required>
                  <option value="">Seleziona</option>
                  {(lookups.data?.courses ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
                </select>
              </label>
              <label>
                <span>Team</span>
                <select value={mandatoryRuleDraft.teamId} onChange={(event) => setMandatoryRuleDraft((draft) => ({ ...draft, teamId: event.target.value }))}>
                  <option value="">Tutti</option>
                  {(lookups.data?.teams ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
                </select>
              </label>
              <label>
                <span>Ruolo</span>
                <input value={mandatoryRuleDraft.roleFilter} onChange={(event) => setMandatoryRuleDraft((draft) => ({ ...draft, roleFilter: event.target.value }))} />
              </label>
              <label>
                <span>Note</span>
                <textarea value={mandatoryRuleDraft.notes} onChange={(event) => setMandatoryRuleDraft((draft) => ({ ...draft, notes: event.target.value }))} />
              </label>
              <ActiveCheckbox checked={mandatoryRuleDraft.active} onChange={(active) => setMandatoryRuleDraft((draft) => ({ ...draft, active }))} />
            </>
          )}

          <div className={styles.modalActions}>
            <Button type="button" variant="secondary" onClick={closeCatalogCreate}>Annulla</Button>
            <Button type="submit" disabled={createMasterData.isPending || updateMasterData.isPending || lookups.isLoading}>
              {catalogEditTarget ? 'Salva' : 'Crea'}
            </Button>
          </div>
        </form>
      </Modal>

      <Modal open={awardModalOpen} onClose={() => setAwardModalOpen(false)} title="Nuova certificazione" size="md">
        <form
          className={styles.formStack}
          onSubmit={(event) => {
            event.preventDefault();
            createAward.mutate(
              {
                employeeId: isPeopleAdmin ? awardDraft.employeeId : undefined,
                certificationId: awardDraft.certificationId,
                outcome: awardDraft.outcome,
                awardedOn: awardDraft.awardedOn,
                expiresOn: awardDraft.expiresOn || undefined,
                validationSource: 'declared_cv',
              },
              {
                onSuccess: () => {
                  setAwardModalOpen(false);
                  setAwardDraft({ employeeId: '', certificationId: '', outcome: 'passed_exam', awardedOn: todayISO(), expiresOn: '' });
                  toast('Certificazione registrata');
                },
                onError: (error) => toast(apiErrorMessage(error, 'Registrazione certificazione non riuscita'), 'error'),
              },
            );
          }}
        >
          {isPeopleAdmin && (
            <label>
              <span>Persona</span>
              <select value={awardDraft.employeeId} onChange={(event) => setAwardDraft((draft) => ({ ...draft, employeeId: event.target.value }))} required>
                <option value="">Seleziona</option>
                {(lookups.data?.employees ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
              </select>
            </label>
          )}
          <label>
            <span>Certificazione</span>
            <select value={awardDraft.certificationId} onChange={(event) => setAwardDraft((draft) => ({ ...draft, certificationId: event.target.value }))} required>
              <option value="">Seleziona</option>
              {(lookups.data?.certifications ?? []).filter((item) => item.active).map((item) => <option key={item.id} value={item.id}>{item.label}</option>)}
            </select>
          </label>
          <label>
            <span>Esito</span>
            <select value={awardDraft.outcome} onChange={(event) => setAwardDraft((draft) => ({ ...draft, outcome: event.target.value }))} required>
              <option value="passed_exam">Esame superato</option>
              <option value="attendance_only">Frequenza</option>
            </select>
          </label>
          <label>
            <span>Data</span>
            <input type="date" value={awardDraft.awardedOn} onChange={(event) => setAwardDraft((draft) => ({ ...draft, awardedOn: event.target.value }))} required />
          </label>
          <label>
            <span>Scadenza</span>
            <input type="date" value={awardDraft.expiresOn} onChange={(event) => setAwardDraft((draft) => ({ ...draft, expiresOn: event.target.value }))} />
          </label>
          <div className={styles.modalActions}>
            <Button type="button" variant="secondary" onClick={() => setAwardModalOpen(false)}>Annulla</Button>
            <Button type="submit" disabled={createAward.isPending || lookups.isLoading}>Registra</Button>
          </div>
        </form>
      </Modal>

      <Modal
        open={correctTarget !== null}
        onClose={() => {
          setCorrectTarget(null);
          setCorrectDraft({
            outcome: 'passed_exam',
            awardedOn: todayISO(),
            expiresOn: '',
            validationSource: 'document_verified',
          });
        }}
        title="Modifica certificazione"
        size="md"
      >
        <form
          className={styles.formStack}
          onSubmit={(event) => {
            event.preventDefault();
            if (!correctTarget) {
              return;
            }
            updateAward.mutate(
              {
                id: correctTarget.awardId,
                outcome: correctDraft.outcome,
                awardedOn: correctDraft.awardedOn,
                expiresOn: correctDraft.expiresOn,
                validationSource: correctDraft.validationSource,
              },
              {
                onSuccess: () => {
                  setCorrectTarget(null);
                  setCorrectDraft({
                    outcome: 'passed_exam',
                    awardedOn: todayISO(),
                    expiresOn: '',
                    validationSource: 'document_verified',
                  });
                  toast('Certificazione aggiornata');
                },
                onError: (error) => toast(apiErrorMessage(error, 'Aggiornamento certificazione non riuscito'), 'error'),
              },
            );
          }}
        >
          <p className={styles.modalText}>
            {correctTarget?.employeeName} - {correctTarget?.certificationName}
          </p>
          <label>
            <span>Esito</span>
            <select value={correctDraft.outcome} onChange={(event) => setCorrectDraft((draft) => ({ ...draft, outcome: event.target.value }))} required>
              <option value="passed_exam">Esame superato</option>
              <option value="attendance_only">Frequenza</option>
            </select>
          </label>
          <div className={styles.formGrid}>
            <label>
              <span>Data</span>
              <input type="date" value={correctDraft.awardedOn} onChange={(event) => setCorrectDraft((draft) => ({ ...draft, awardedOn: event.target.value }))} required />
            </label>
            <label>
              <span>Scadenza</span>
              <input type="date" value={correctDraft.expiresOn} onChange={(event) => setCorrectDraft((draft) => ({ ...draft, expiresOn: event.target.value }))} />
            </label>
          </div>
          <label>
            <span>Origine</span>
            <select value={correctDraft.validationSource} onChange={(event) => setCorrectDraft((draft) => ({ ...draft, validationSource: event.target.value }))} required>
              {Object.entries(validationSourceLabels).map(([value, sourceLabel]) => (
                <option key={value} value={value}>{sourceLabel}</option>
              ))}
            </select>
          </label>
          <div className={styles.modalActions}>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setCorrectTarget(null);
                setCorrectDraft({
                  outcome: 'passed_exam',
                  awardedOn: todayISO(),
                  expiresOn: '',
                  validationSource: 'document_verified',
                });
              }}
            >
              Annulla
            </Button>
            <Button type="submit" disabled={updateAward.isPending}>
              Salva
            </Button>
          </div>
        </form>
      </Modal>
    </main>
  );
}

function PageHeader({ view, isPeopleAdmin }: { view: TrainingView; isPeopleAdmin: boolean }) {
  return (
    <header className={styles.header}>
      <div>
        <h1>{viewTitles[view]}</h1>
        <p>{viewSubtitles[view]}</p>
      </div>
      <span className={styles.scopeBadge}>{isPeopleAdmin ? 'Vista People' : 'La mia vista'}</span>
    </header>
  );
}

function StateBlock({ title, text, tone }: { title: string; text: string; tone: 'info' | 'warning' }) {
  return (
    <section className={`${styles.stateBlock} ${styles[tone]}`}>
      <Icon name={tone === 'info' ? 'info' : 'triangle-alert'} />
      <div>
        <h2>{title}</h2>
        <p>{text}</p>
      </div>
    </section>
  );
}

function PlanView({
  rows,
  allRows,
  q,
  team,
  status,
  year,
  isPeopleAdmin,
  actionsEnabled,
  onSearch,
  onTeam,
  onStatus,
  onYear,
  onConfirm,
  onReasonTransition,
  onDownloadDocument,
  onEdit,
  onValidateDocument,
  onUploadDocument,
  onTransition,
  onExport,
  onNew,
}: {
  rows: PlanEnrollment[];
  allRows: PlanEnrollment[];
  q: string;
  team: string;
  status: string;
  year: string;
  isPeopleAdmin: boolean;
  actionsEnabled: boolean;
  onSearch: (value: string) => void;
  onTeam: (value: string | null) => void;
  onStatus: (value: string | null) => void;
  onYear: (value: string | null) => void;
  onConfirm: (row: PlanEnrollment) => void;
  onReasonTransition: (row: PlanEnrollment, transition: ReasonTransitionTarget['transition']) => void;
  onDownloadDocument: (row: PlanEnrollment) => void;
  onEdit: (row: PlanEnrollment) => void;
  onValidateDocument: (documentId: string) => void;
  onUploadDocument: (enrollmentId: string) => void;
  onTransition: (id: string, transition: string) => void;
  onExport: () => void;
  onNew: () => void;
}) {
  const years = uniqueOptions(allRows.map((item) => String(item.year)));
  const teams = uniqueOptions(allRows.map((item) => item.teamCode));
  const statuses = uniqueOptions(allRows.map((item) => item.status)).map((item) => ({
    value: item.value,
    label: label(item.value),
  }));
  const hours = rows.reduce((sum, row) => sum + (row.hoursPlanned ?? 0), 0);
  const cost = rows.reduce((sum, row) => sum + (row.costPlanned ?? 0), 0);

  return (
    <section className={styles.surface}>
      <div className={styles.toolbar}>
        <SearchInput value={q} onChange={onSearch} placeholder="Cerca persone o corsi" />
        <SingleSelect options={years} selected={year || null} onChange={onYear} placeholder="Anno" allowClear />
        <SingleSelect options={teams} selected={team || null} onChange={onTeam} placeholder="Team" allowClear />
        <SingleSelect options={statuses} selected={status || null} onChange={onStatus} placeholder="Stato" allowClear />
        {isPeopleAdmin && (
          <Button variant="secondary" leftIcon={<Icon name="plus" size={16} />} disabled={!actionsEnabled} onClick={onNew}>
            Nuova iscrizione
          </Button>
        )}
        <Button variant="secondary" leftIcon={<Icon name="download" size={16} />} disabled={!actionsEnabled} onClick={onExport}>
          Esporta XLSX
        </Button>
      </div>

      <div className={styles.resultBar}>
        <span>{rows.length} iscrizioni</span>
        <span>{hours} ore pianificate</span>
        <span>{formatMoney(cost)} budget filtrato</span>
      </div>

      {rows.length === 0 ? (
        <EmptyState
          title="Nessuna iscrizione"
          text={isPeopleAdmin ? 'Modifica i filtri o crea una nuova iscrizione per il piano.' : 'Non ci sono iscrizioni coerenti con i filtri selezionati.'}
        />
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Persona</th>
                <th>Team</th>
                <th>Corso</th>
                <th>Fornitore</th>
                <th>Stato</th>
                <th>Periodo</th>
                <th className={styles.numCol}>Ore</th>
                <th className={styles.numCol}>Costo</th>
                <th>Documento</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.id}>
                  <td>
                    <strong>{row.employeeName}</strong>
                    <span>{row.employeeEmail}</span>
                  </td>
                  <td>{row.teamCode}</td>
                  <td>
                    {row.courseTitle}
                    {row.mandatory && <span className={styles.inlineBadge}>Obbligatoria</span>}
                  </td>
                  <td>{row.vendorName}</td>
                  <td><StatusPill value={row.status} /></td>
                  <td>{formatDate(row.plannedStart)} {row.plannedEnd ? `- ${formatDate(row.plannedEnd)}` : ''}</td>
                  <td className={styles.numCol}>{row.hoursPlanned ?? ''}</td>
                  <td className={styles.numCol}>{formatMoney(row.costPlanned)}</td>
                  <td>
                    {row.documentFilename ? (
                      <>
                        <strong>{row.documentFilename}</strong>
                        <span className={styles.inlineBadge}>{row.documentValidated ? 'Validato' : 'Da validare'}</span>
                      </>
                    ) : (
                      'Non caricato'
                    )}
                  </td>
                  <td>
                    {isPeopleAdmin ? (
                      <div className={styles.rowActions}>
                        <Button variant="ghost" size="sm" leftIcon={<Icon name="pencil" size={15} />} disabled={!actionsEnabled} onClick={() => onEdit(row)}>Modifica</Button>
                        <Button variant="ghost" size="sm" leftIcon={<Icon name="download" size={15} />} disabled={!actionsEnabled || !row.documentId} onClick={() => onDownloadDocument(row)}>Scarica</Button>
                        <Button variant="ghost" size="sm" leftIcon={<Icon name="file-up" size={15} />} disabled={!actionsEnabled} onClick={() => onUploadDocument(row.id)}>Carica</Button>
                        <Button variant="ghost" size="sm" leftIcon={<Icon name="check-circle" size={15} />} disabled={!actionsEnabled || !row.documentId || row.documentValidated} onClick={() => row.documentId && onValidateDocument(row.documentId)}>Valida</Button>
                        <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'proposed'} onClick={() => onTransition(row.id, 'approve')}>Approva</Button>
                        <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'approved'} onClick={() => onReasonTransition(row, 'revert_to_proposed')}>Riporta</Button>
                        <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'approved'} onClick={() => onTransition(row.id, 'start')}>Avvia</Button>
                        <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'in_progress'} onClick={() => onTransition(row.id, 'complete')}>Completa</Button>
                        <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'in_progress'} onClick={() => onTransition(row.id, 'fail')}>Non superata</Button>
                        <Button variant="ghost" size="sm" disabled={!actionsEnabled || !['completed', 'failed', 'cancelled', 'expired'].includes(row.status)} onClick={() => onReasonTransition(row, 'reopen')}>Riapri</Button>
                        <Button
                          variant="danger"
                          size="sm"
                          disabled={!actionsEnabled || !['proposed', 'approved', 'in_progress'].includes(row.status)}
                          onClick={() => onConfirm(row)}
                        >
                          Annulla
                        </Button>
                      </div>
                    ) : (
                      <div className={styles.rowActions}>
                        <Button variant="ghost" size="sm" leftIcon={<Icon name="download" size={15} />} disabled={!actionsEnabled || !row.documentId} onClick={() => onDownloadDocument(row)}>Scarica</Button>
                        <Button variant="ghost" size="sm" leftIcon={<Icon name="file-up" size={15} />} disabled={!actionsEnabled} onClick={() => onUploadDocument(row.id)}>Carica</Button>
                        <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'approved'} onClick={() => onTransition(row.id, 'start')}>Avvia</Button>
                        <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'in_progress'} onClick={() => onTransition(row.id, 'complete')}>Completa</Button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function RequestsView({
  rows,
  allRows,
  q,
  status,
  isPeopleAdmin,
  actionsEnabled,
  onSearch,
  onStatus,
  onNew,
  onExport,
  onAction,
  onReject,
  onConvert,
}: {
  rows: TrainingRequest[];
  allRows: TrainingRequest[];
  q: string;
  status: string;
  isPeopleAdmin: boolean;
  actionsEnabled: boolean;
  onSearch: (value: string) => void;
  onStatus: (value: string | null) => void;
  onNew: () => void;
  onExport: () => void;
  onAction: (id: string, transition: string) => void;
  onReject: (row: TrainingRequest) => void;
  onConvert: (row: TrainingRequest) => void;
}) {
  const statuses = uniqueOptions(allRows.map((item) => item.status)).map((item) => ({
    value: item.value,
    label: label(item.value),
  }));

  return (
    <section className={styles.surface}>
      <div className={styles.toolbar}>
        <SearchInput value={q} onChange={onSearch} placeholder="Cerca richieste" />
        <SingleSelect options={statuses} selected={status || null} onChange={onStatus} placeholder="Stato" allowClear />
        <Button variant="secondary" leftIcon={<Icon name="plus" size={16} />} disabled={!actionsEnabled} onClick={onNew}>
          Nuova richiesta
        </Button>
        <Button variant="secondary" leftIcon={<Icon name="download" size={16} />} disabled={!actionsEnabled} onClick={onExport}>
          Esporta XLSX
        </Button>
      </div>
      {rows.length === 0 ? (
        <EmptyState title="Nessuna richiesta" text="Non ci sono richieste coerenti con i filtri selezionati." />
      ) : (
        <div className={styles.requestGrid}>
          {rows.map((row) => (
            <article key={row.id} className={styles.requestItem}>
              <div>
                <div className={styles.itemTopline}>
                  <span>{row.employeeName}</span>
                  <StatusPill value={row.status} />
                </div>
                <h2>{row.courseTitle || row.freeTextTitle}</h2>
                <p>{row.motivation}</p>
              </div>
              <div className={styles.itemFooter}>
                <span>{row.skillAreaName}</span>
                <span>{row.desiredYear}</span>
              </div>
              {isPeopleAdmin && (
                <div className={styles.rowActions}>
                  <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'submitted'} onClick={() => onAction(row.id, 'start_review')}>Valuta</Button>
                  <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'under_review'} onClick={() => onAction(row.id, 'accept')}>Accetta</Button>
                  <Button variant="ghost" size="sm" disabled={!actionsEnabled || row.status !== 'accepted' || !row.courseId} onClick={() => onConvert(row)}>Converti</Button>
                  <Button variant="danger" size="sm" disabled={!actionsEnabled || row.status !== 'under_review'} onClick={() => onReject(row)}>Respingi</Button>
                </div>
              )}
              {!isPeopleAdmin && (
                <div className={styles.rowActions}>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={!actionsEnabled || !['submitted', 'under_review'].includes(row.status)}
                    onClick={() => onAction(row.id, 'withdraw')}
                  >
                    Ritira
                  </Button>
                </div>
              )}
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

function CatalogView({
  rows,
  q,
  status,
  isPeopleAdmin,
  actionsEnabled,
  masterData,
  onSearch,
  onStatus,
  onExport,
  onNew,
  onEditCourse,
  onToggleCourse,
  onNewData,
  onEditData,
  onToggleData,
}: {
  rows: CatalogCourse[];
  q: string;
  status: string;
  isPeopleAdmin: boolean;
  actionsEnabled: boolean;
  masterData?: CatalogMasterData;
  onSearch: (value: string) => void;
  onStatus: (value: string | null) => void;
  onExport: () => void;
  onNew: () => void;
  onEditCourse: (course: CatalogCourse) => void;
  onToggleCourse: (course: CatalogCourse) => void;
  onNewData: (kind: TrainingMasterDataKind) => void;
  onEditData: (kind: TrainingMasterDataKind, id: string) => void;
  onToggleData: (kind: TrainingMasterDataKind, id: string, active: boolean) => void;
}) {
  const filters = [
    { value: 'active', label: 'Attivi' },
    { value: 'mandatory', label: 'Obbligatori' },
  ];

  return (
    <section className={styles.surface}>
      <div className={styles.toolbar}>
        <SearchInput value={q} onChange={onSearch} placeholder="Cerca corsi o fornitori" />
        <SingleSelect options={filters} selected={status || null} onChange={onStatus} placeholder="Tipo" allowClear />
        {isPeopleAdmin && (
          <>
            <Button variant="secondary" leftIcon={<Icon name="plus" size={16} />} disabled={!actionsEnabled} onClick={onNew}>
              Nuovo corso
            </Button>
            {(['vendors', 'skill-areas', 'certifications', 'teams', 'plans', 'mandatory-rules'] as TrainingMasterDataKind[]).map((kind) => (
              <Button key={kind} variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} disabled={!actionsEnabled} onClick={() => onNewData(kind)}>
                {catalogCreateLabels[kind]}
              </Button>
            ))}
          </>
        )}
        <Button variant="secondary" leftIcon={<Icon name="download" size={16} />} disabled={!actionsEnabled} onClick={onExport}>
          Esporta XLSX
        </Button>
      </div>
      {rows.length === 0 ? (
        <EmptyState title="Nessun corso" text="Il catalogo non contiene corsi coerenti con i filtri selezionati." />
      ) : (
        <div className={styles.catalogGrid}>
          {rows.map((course) => (
            <article key={course.id} className={styles.catalogItem}>
              <div className={styles.itemTopline}>
                <span>{course.skillAreaName || 'Area non assegnata'}</span>
                <span className={styles.rowActions}>
                  {course.mandatory && <span className={styles.inlineBadge}>Obbligatorio</span>}
                  {!course.active && <span className={styles.statusPill}>Non attivo</span>}
                </span>
              </div>
              <h2>{course.title}</h2>
              <dl>
                <div>
                  <dt>Fornitore</dt>
                  <dd>{course.vendorName || 'Da assegnare'}</dd>
                </div>
                <div>
                  <dt>Certificazione</dt>
                  <dd>{course.certificationName || 'Non collegata'}</dd>
                </div>
                <div>
                  <dt>Impegno</dt>
                  <dd>{course.defaultHours ?? '-'} ore</dd>
                </div>
                <div>
                  <dt>Costo</dt>
                  <dd>{formatMoney(course.defaultCost)}</dd>
                </div>
              </dl>
              {isPeopleAdmin && (
                <div className={styles.itemFooter}>
                  <span>{label(course.deliveryMode)}</span>
                  <div className={styles.rowActions}>
                    <Button variant="secondary" size="sm" leftIcon={<Icon name="pencil" size={14} />} disabled={!actionsEnabled} onClick={() => onEditCourse(course)}>
                      Modifica
                    </Button>
                    <Button variant={course.active ? 'danger' : 'secondary'} size="sm" disabled={!actionsEnabled} onClick={() => onToggleCourse(course)}>
                      {course.active ? 'Disattiva' : 'Riattiva'}
                    </Button>
                  </div>
                </div>
              )}
            </article>
          ))}
        </div>
      )}
      {isPeopleAdmin && masterData && (
        <MasterDataPanel
          data={masterData}
          actionsEnabled={actionsEnabled}
          onEdit={onEditData}
          onToggle={onToggleData}
        />
      )}
    </section>
  );
}

function MasterDataPanel({
  data,
  actionsEnabled,
  onEdit,
  onToggle,
}: {
  data: CatalogMasterData;
  actionsEnabled: boolean;
  onEdit: (kind: TrainingMasterDataKind, id: string) => void;
  onToggle: (kind: TrainingMasterDataKind, id: string, active: boolean) => void;
}) {
  return (
    <div className={styles.masterDataPanel}>
      <MasterDataSection
        title="Fornitori"
        rows={data.vendors.map((row) => ({
          id: row.id,
          primary: row.name,
          secondary: row.website || row.notes,
          active: row.active,
        }))}
        kind="vendors"
        actionsEnabled={actionsEnabled}
        onEdit={onEdit}
        onToggle={onToggle}
      />
      <MasterDataSection
        title="Team"
        rows={data.teams.map((row) => ({
          id: row.id,
          primary: `${row.code} - ${row.name}`,
          secondary: row.description,
          active: row.active,
        }))}
        kind="teams"
        actionsEnabled={actionsEnabled}
        onEdit={onEdit}
        onToggle={onToggle}
      />
      <MasterDataSection
        title="Aree"
        rows={data.skillAreas.map((row) => ({
          id: row.id,
          primary: `${row.code} - ${row.name}`,
          secondary: row.parentLabel || row.description,
          active: row.active,
        }))}
        kind="skill-areas"
        actionsEnabled={actionsEnabled}
        onEdit={onEdit}
        onToggle={onToggle}
      />
      <MasterDataSection
        title="Certificazioni"
        rows={data.certifications.map((row) => ({
          id: row.id,
          primary: `${row.code} - ${row.name}`,
          secondary: row.issuerVendorName || row.skillAreaLabel,
          active: row.active,
        }))}
        kind="certifications"
        actionsEnabled={actionsEnabled}
        onEdit={onEdit}
        onToggle={onToggle}
      />
      <MasterDataSection
        title="Piani"
        rows={data.plans.map((row) => ({
          id: row.id,
          primary: `${row.year} - ${label(row.status)}`,
          secondary: formatMoney(row.budgetTotal),
        }))}
        kind="plans"
        actionsEnabled={actionsEnabled}
        onEdit={onEdit}
      />
      <MasterDataSection
        title="Regole"
        rows={data.mandatoryRules.map((row) => ({
          id: row.id,
          primary: row.courseTitle,
          secondary: [row.teamLabel || 'Tutti', row.roleFilter].filter(Boolean).join(' - '),
          active: row.active,
        }))}
        kind="mandatory-rules"
        actionsEnabled={actionsEnabled}
        onEdit={onEdit}
        onToggle={onToggle}
      />
    </div>
  );
}

function MasterDataSection({
  title,
  rows,
  kind,
  actionsEnabled,
  onEdit,
  onToggle,
}: {
  title: string;
  rows: Array<{ id: string; primary: string; secondary?: string; active?: boolean }>;
  kind: TrainingMasterDataKind;
  actionsEnabled: boolean;
  onEdit: (kind: TrainingMasterDataKind, id: string) => void;
  onToggle?: (kind: TrainingMasterDataKind, id: string, active: boolean) => void;
}) {
  const previewRows = rows.slice(0, 4);
  return (
    <section className={styles.masterDataSection}>
      <div className={styles.masterDataHeader}>
        <h3>{title}</h3>
        <span>{rows.length}</span>
      </div>
      {previewRows.length === 0 ? (
        <p className={styles.masterDataEmpty}>Nessuna voce</p>
      ) : (
        <ul>
          {previewRows.map((row) => (
            <li key={row.id}>
              <div>
                <strong>{row.primary}</strong>
                {row.secondary && <span>{row.secondary}</span>}
                {row.active === false && <span className={styles.statusPill}>Non attiva</span>}
              </div>
              <div className={styles.rowActions}>
                <Button variant="ghost" size="sm" leftIcon={<Icon name="pencil" size={14} />} disabled={!actionsEnabled} onClick={() => onEdit(kind, row.id)}>
                  Modifica
                </Button>
                {onToggle && row.active !== undefined && (
                  <Button variant="ghost" size="sm" disabled={!actionsEnabled} onClick={() => onToggle(kind, row.id, row.active === true)}>
                    {row.active ? 'Disattiva' : 'Riattiva'}
                  </Button>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function CertificationsView({
  rows,
  allRows,
  q,
  status,
  year,
  isPeopleAdmin,
  actionsEnabled,
  onSearch,
  onStatus,
  onYear,
  onNew,
  onExport,
  onCorrect,
  onDownload,
  onValidate,
  onUpload,
}: {
  rows: CertificationRow[];
  allRows: CertificationRow[];
  q: string;
  status: string;
  year: string;
  isPeopleAdmin: boolean;
  actionsEnabled: boolean;
  onSearch: (value: string) => void;
  onStatus: (value: string | null) => void;
  onYear: (value: string | null) => void;
  onNew: () => void;
  onExport: () => void;
  onCorrect: (row: CertificationRow) => void;
  onDownload: (row: CertificationRow) => void;
  onValidate: (documentId: string) => void;
  onUpload: (awardId: string) => void;
}) {
  const statuses = uniqueOptions(allRows.map((row) => row.currentStatus)).map((item) => ({
    value: item.value,
    label: label(item.value),
  }));
  const years = uniqueOptions(allRows.map((row) => row.awardedOn?.slice(0, 4)));

  return (
    <section className={styles.surface}>
      <div className={styles.toolbar}>
        <SearchInput value={q} onChange={onSearch} placeholder="Cerca certificazioni" />
        <SingleSelect options={statuses} selected={status || null} onChange={onStatus} placeholder="Stato" allowClear />
        <SingleSelect options={years} selected={year || null} onChange={onYear} placeholder="Anno" allowClear />
        <div className={styles.resultBar}>
          <span>{rows.length} certificazioni</span>
          <span>{rows.filter((row) => row.currentStatus === 'valid').length} valide</span>
        </div>
        <Button variant="secondary" leftIcon={<Icon name="plus" size={16} />} disabled={!actionsEnabled} onClick={onNew}>
          Nuova certificazione
        </Button>
        <Button variant="secondary" leftIcon={<Icon name="download" size={16} />} disabled={!actionsEnabled} onClick={onExport}>
          Esporta XLSX
        </Button>
      </div>
      {rows.length === 0 ? (
        <EmptyState title="Nessuna certificazione" text="Non risultano certificazioni per la vista corrente." />
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                {isPeopleAdmin && <th>Persona</th>}
                <th>Certificazione</th>
                <th>Esito</th>
                <th>Data</th>
                <th>Scadenza</th>
                <th>Stato</th>
                <th>Attestato</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.awardId}>
                  {isPeopleAdmin && <td>{row.employeeName}</td>}
                  <td>
                    <strong>{row.certificationCode}</strong>
                    <span>{row.certificationName}</span>
                  </td>
                  <td>{label(row.outcome)}</td>
                  <td>{formatDate(row.awardedOn)}</td>
                  <td>{formatDate(row.expiresOn) || 'Senza scadenza'}</td>
                  <td><StatusPill value={row.currentStatus} /></td>
                  <td>
                    {row.documentFilename ? (
                      <>
                        <strong>{row.documentFilename}</strong>
                        <span className={styles.inlineBadge}>{row.documentValidated ? 'Validato' : 'Da validare'}</span>
                      </>
                    ) : (
                      'Non caricato'
                    )}
                  </td>
                  <td>
                    <div className={styles.rowActions}>
                      <Button variant="ghost" size="sm" leftIcon={<Icon name="download" size={15} />} disabled={!actionsEnabled || !row.documentId} onClick={() => onDownload(row)}>
                        Scarica
                      </Button>
                      {isPeopleAdmin && (
                        <>
                          <Button variant="ghost" size="sm" leftIcon={<Icon name="check-circle" size={15} />} disabled={!actionsEnabled || !row.documentId || row.documentValidated} onClick={() => row.documentId && onValidate(row.documentId)}>
                            Valida
                          </Button>
                          <Button variant="ghost" size="sm" leftIcon={<Icon name="pencil" size={15} />} disabled={!actionsEnabled} onClick={() => onCorrect(row)}>
                            Modifica
                          </Button>
                        </>
                      )}
                      <Button variant="ghost" size="sm" leftIcon={<Icon name="file-up" size={15} />} disabled={!actionsEnabled} onClick={() => onUpload(row.awardId)}>
                        Carica
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function ReportsView({
  planBudget,
  expiring,
  gaps,
  isPeopleAdmin,
  actionsEnabled,
  onExport,
  onRunJobs,
}: {
  planBudget: PlanBudgetRow[];
  expiring: ExpiringCertificationRow[];
  gaps: ComplianceGapRow[];
  isPeopleAdmin: boolean;
  actionsEnabled: boolean;
  onExport: (kind: string) => void;
  onRunJobs: () => void;
}) {
  return (
    <section className={styles.reportGrid}>
      {isPeopleAdmin && (
        <section className={styles.surface}>
          <div className={styles.toolbar}>
            <div className={styles.resultBar}>
              <span>Controlli piano e scadenze</span>
            </div>
            <Button variant="secondary" leftIcon={<Icon name="settings" size={16} />} disabled={!actionsEnabled} onClick={onRunJobs}>
              Esegui controlli
            </Button>
          </div>
        </section>
      )}
      <ReportPanel title="Budget piano" action={isPeopleAdmin ? 'Esporta XLSX' : undefined} actionsEnabled={actionsEnabled} onAction={() => onExport('plan-budget')}>
        <ReportTable
          headers={['Anno', 'Team', 'Iscrizioni', 'Costo', 'Ore']}
          rows={planBudget.map((row) => [
            String(row.year),
            row.teamCode || '-',
            String(row.enrollmentsCount),
            formatMoney(row.costTotal),
            row.hoursTotal?.toString() ?? '',
          ])}
        />
      </ReportPanel>
      <ReportPanel
        title="Certificazioni in scadenza"
        action={isPeopleAdmin ? 'Esporta XLSX' : undefined}
        actionsEnabled={actionsEnabled}
        onAction={() => onExport('expiring-certifications')}
      >
        <ReportTable
          headers={['Persona', 'Certificazione', 'Scadenza', 'Giorni']}
          rows={expiring.map((row) => [
            row.employeeName,
            row.certificationCode,
            formatDate(row.expiresOn),
            String(row.daysToExpiry),
          ])}
        />
      </ReportPanel>
      <ReportPanel
        title="Formazione obbligatoria"
        action={isPeopleAdmin ? 'Esporta XLSX' : undefined}
        actionsEnabled={actionsEnabled}
        onAction={() => onExport('compliance-gaps')}
      >
        <ReportTable
          headers={['Persona', 'Corso', 'Ambito', 'Stato']}
          rows={gaps.map((row) => [
            row.employeeName,
            row.courseTitle,
            row.complianceFramework || '-',
            label(row.complianceStatus),
          ])}
        />
      </ReportPanel>
    </section>
  );
}

function ReportPanel({
  title,
  action,
  actionsEnabled,
  onAction,
  children,
}: {
  title: string;
  action?: string;
  actionsEnabled: boolean;
  onAction?: () => void;
  children: ReactNode;
}) {
  return (
    <article className={styles.reportPanel}>
      <div className={styles.reportHeader}>
        <h2>{title}</h2>
        {action && (
          <Button
            variant="secondary"
            size="sm"
            leftIcon={<Icon name="download" size={15} />}
            disabled={!actionsEnabled}
            onClick={onAction}
          >
            {action}
          </Button>
        )}
      </div>
      {children}
    </article>
  );
}

function ReportTable({ headers, rows }: { headers: string[]; rows: string[][] }) {
  if (rows.length === 0) {
    return <EmptyState title="Nessun dato" text="Non ci sono dati disponibili per questo report." compact />;
  }
  return (
    <div className={styles.tableWrap}>
      <table className={styles.table}>
        <thead>
          <tr>{headers.map((header) => <th key={header}>{header}</th>)}</tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr key={`${rowIndex}-${row.join('|')}`}>
              {row.map((cell, cellIndex) => (
                <td key={`${cellIndex}-${cell}`}>{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EmptyState({ title, text, compact = false }: { title: string; text: string; compact?: boolean }) {
  return (
    <div className={`${styles.emptyState} ${compact ? styles.emptyCompact : ''}`}>
      <Icon name="file-text" />
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

function ActiveCheckbox({ checked, onChange }: { checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className={styles.checkboxRow}>
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      <span>Attivo</span>
    </label>
  );
}

function StatusPill({ value }: { value: string }) {
  const terminal = ['completed', 'passed_exam', 'valid', 'valid_no_expiry', 'compliant'].includes(value);
  const warning = ['proposed', 'approved', 'submitted', 'under_review', 'missing_or_expired', 'no_cert_linked'].includes(value);
  return (
    <span className={`${styles.statusPill} ${terminal ? styles.okPill : ''} ${warning ? styles.warnPill : ''}`}>
      {label(value)}
    </span>
  );
}
