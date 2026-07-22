import { Button, Drawer, Icon, Modal, MoneyInput, Skeleton, StatusBadge, useToast } from '@mrsmith/ui';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState, type ChangeEvent, type DragEvent, type KeyboardEvent, type RefObject } from 'react';
import { useApiClient } from '../../api/client';
import type {
  MACompanyOverview,
  MADeepValuation,
  MAFilingAcquireResponse,
  MAFilingAcquisitionView,
  MAFilingIdentityOverrideResponse,
  MAFilingProposalsResponse,
  MAFilingRow,
  MAFilingSearchStartResponse,
  MAFilingUploadResponse,
  MAFilingsResponse,
  MANIDecisionResponse,
  MANIDecisionView,
  MANIProposalView,
} from '../../api/types';
import { dateLabel, downloadBlob, errorLabel } from '../../pages/ricerche/helpers';
import { DepositedValuationView } from './DepositedValuationView';
import {
  acquisitionExceptionBadge,
  acquisitionExceptionFact,
  acquisitionInProgress,
  acquisitionProgressLabel,
  defaultTreatmentChoice,
  effectPhrase,
  exerciseLabel,
  exerciseYearOf,
  filingErrorMessage,
  filingExceptionBadge,
  filingExceptionFact,
  filingIsIdentityOverridable,
  filingOriginLabel,
  filingProcessingLabel,
  filingTypeLabel,
  formatEuro,
  formatSignedEuro,
  incertezzaLabel,
  isFilingInProgress,
  proposalDisplayAmount,
  proposalIsDecided,
  proposalNeedsTreatmentChoice,
  proposalStateText,
  proposalThemeKey,
  treatmentLabel,
} from './filingFacts';
import styles from './filings.module.css';

const UPLOAD_MAX_BYTES = 30 * 1024 * 1024;
const NARROW_QUERY = '(max-width: 1000px)';

// Work is in flight when any filing is non-terminal, an acquisition has a LIVE job (inflight), or
// the latest search is still resolving. A stalled/failed acquisition (inflight=false) is NOT work
// in progress — it waits on the analyst's Riprendi, so it must not keep the block polling forever.
function filingsWorkInProgress(data: MAFilingsResponse | undefined): boolean {
  if (!data) return false;
  if (data.filings.some((f) => isFilingInProgress(f.status))) return true;
  if (data.acquisitionsOpen.some((a) => a.inflight)) return true;
  const s = data.latestSearch?.status;
  return s === 'intent' || s === 'requested';
}

function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() =>
    typeof window !== 'undefined' && typeof window.matchMedia === 'function' ? window.matchMedia(query).matches : false,
  );
  useEffect(() => {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    const list = window.matchMedia(query);
    const handler = () => setMatches(list.matches);
    handler();
    list.addEventListener('change', handler);
    return () => list.removeEventListener('change', handler);
  }, [query]);
  return matches;
}

// A year group: the primary ready/degraded filing (whose NI proposals fill the rows) plus any
// exceptions on the same exercise (non-ready filings, open acquisitions). Undated items (a filing
// whose closing date is not yet resolved, an acquisition without an exercise) sit ABOVE the groups.
interface YearGroup {
  key: string;
  year: string;
  primary?: MAFilingRow;
  proposals: MANIProposalView[];
  proposalsLoading: boolean;
  exceptionFilings: MAFilingRow[];
  acquisitions: MAFilingAcquisitionView[];
}

// A flat index entry for a selectable proposal row.
interface ProposalEntry {
  proposal: MANIProposalView;
  filing: MAFilingRow;
  year: string;
}

// Redesign of the "Bilanci depositati" block (issue #80, Fase N2, struttura A). Deposited fascicoli
// are grouped by exercise; each ready/degraded filing's NI rettifiche become one-line rows worked in
// a single sticky panel on the right (a Drawer below ~1000px). The adjusted-valuation summary sits
// above the groups. Upload + camerale search moved to the block title row. Exception-only, facts-only,
// no prices. Polling runs ONLY while there is work in flight and never re-triggers an entrance.
export function FilingsBlock({
  companyKey,
  overview,
  baselineValuation,
}: {
  companyKey: string;
  overview: MACompanyOverview;
  baselineValuation?: MADeepValuation;
}) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const encodedKey = encodeURIComponent(companyKey);
  const isNarrow = useMediaQuery(NARROW_QUERY);
  const baselineExercise = overview.adjusted?.baselineExercise;

  const filingsKey = ['ma-filings', companyKey];
  const query = useQuery({
    queryKey: filingsKey,
    enabled: Boolean(companyKey),
    queryFn: () => api.get<MAFilingsResponse>(`/binocolo/v1/ma/companies/${encodedKey}/filings`),
    // Poll ONLY while work is in flight; the state is always read from the filings query (never the
    // searchId directly — a dedup-race can return an orphan intent id). A calm surface stops polling.
    refetchInterval: (q) => (filingsWorkInProgress(q.state.data) ? 4500 : false),
  });
  const data = query.data;

  const refetchFilings = () => void query.refetch();
  const invalidateOverview = () => queryClient.invalidateQueries({ queryKey: ['ma-company-overview', companyKey] });

  // React ONLY to the search-status transition (prevRef), never on every poll tick.
  const prevSearchRef = useRef<{ id?: string; status?: string }>({});
  useEffect(() => {
    const search = data?.latestSearch;
    const prev = prevSearchRef.current;
    const sameSearch = prev.id === search?.id;
    prevSearchRef.current = { id: search?.id, status: search?.status };
    if (!search || !sameSearch) return; // first sight of a search: baseline, no announcement
    const wasResolving = prev.status === 'intent' || prev.status === 'requested';
    if (!wasResolving) return;
    if (search.status === 'results') toast('Depositi camerali disponibili.', 'success');
    else if (search.status === 'failed') toast('Ricerca depositi non riuscita.', 'error');
  }, [data?.latestSearch, toast]);

  // ── Proposals for every ready/degraded filing (all groups render their rows at once) ──
  const readyFilings = useMemo(
    () => (data?.filings ?? []).filter((f) => f.status === 'ready' || f.status === 'degraded'),
    [data?.filings],
  );
  const proposalResults = useQueries({
    queries: readyFilings.map((f) => ({
      queryKey: ['ma-filing-proposals', f.id],
      queryFn: () => api.get<MAFilingProposalsResponse>(`/binocolo/v1/ma/filings/${encodeURIComponent(f.id)}/proposals`),
    })),
  });
  const proposalsByFiling = useMemo(() => {
    const map = new Map<string, { proposals: MANIProposalView[]; loading: boolean }>();
    readyFilings.forEach((f, i) => {
      const r = proposalResults[i];
      map.set(f.id, { proposals: r?.data?.proposals ?? [], loading: Boolean(r?.isLoading) });
    });
    return map;
  }, [readyFilings, proposalResults]);

  // ── Group by exercise ──
  const { datedGroups, undatedFilings, undatedAcquisitions } = useMemo(() => {
    const byYear = new Map<string, YearGroup>();
    const undatedF: MAFilingRow[] = [];
    const undatedA: MAFilingAcquisitionView[] = [];
    const ensure = (year: string): YearGroup => {
      let g = byYear.get(year);
      if (!g) {
        g = { key: year, year, proposals: [], proposalsLoading: false, exceptionFilings: [], acquisitions: [] };
        byYear.set(year, g);
      }
      return g;
    };
    for (const f of data?.filings ?? []) {
      const year = exerciseYearOf(f.closingDate);
      const isReady = f.status === 'ready' || f.status === 'degraded';
      if (!year) {
        // A ready filing without a resolved date is unexpected; keep its rows visible under undated.
        undatedF.push(f);
        continue;
      }
      const g = ensure(year);
      if (isReady && !g.primary) {
        g.primary = f;
        const p = proposalsByFiling.get(f.id);
        g.proposals = p?.proposals ?? [];
        g.proposalsLoading = p?.loading ?? false;
      } else {
        g.exceptionFilings.push(f);
      }
    }
    for (const a of data?.acquisitionsOpen ?? []) {
      const year = exerciseYearOf(a.closingDate);
      if (!year) undatedA.push(a);
      else ensure(year).acquisitions.push(a);
    }
    const dated = [...byYear.values()].sort((a, b) => b.year.localeCompare(a.year));
    return { datedGroups: dated, undatedFilings: undatedF, undatedAcquisitions: undatedA };
  }, [data?.filings, data?.acquisitionsOpen, proposalsByFiling]);

  // Flat index of selectable proposals in display order (for auto-advance + keyboard nav).
  const proposalEntries = useMemo(() => {
    const entries: ProposalEntry[] = [];
    for (const g of datedGroups) {
      if (!g.primary) continue;
      for (const p of g.proposals) entries.push({ proposal: p, filing: g.primary, year: g.year });
    }
    return entries;
  }, [datedGroups]);
  const entryById = useMemo(() => {
    const m = new Map<string, ProposalEntry>();
    for (const e of proposalEntries) m.set(e.proposal.id, e);
    return m;
  }, [proposalEntries]);

  // ── Selection + drafts (kept per proposal so switching rows never loses in-progress work) ──
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drafts, setDrafts] = useState<Record<string, ProposalDraft>>({});

  // Reset selection + drafts when the company changes (staffetta lands on a fresh scheda).
  useEffect(() => {
    setSelectedId(null);
    setDrawerOpen(false);
    setDrafts({});
  }, [companyKey]);

  // Auto-select the first actionable proposal once (never overrides a live selection, never opens
  // the drawer — that stays an explicit gesture).
  useEffect(() => {
    if (selectedId && entryById.has(selectedId)) return;
    if (selectedId && !entryById.has(selectedId)) {
      setSelectedId(null);
      return;
    }
    const first = proposalEntries.find((e) => !proposalIsDecided(e.proposal.state)) ?? proposalEntries[0];
    if (first) setSelectedId(first.proposal.id);
  }, [proposalEntries, entryById, selectedId]);

  const selectInline = (id: string) => setSelectedId(id);
  const selectAndOpen = (id: string) => {
    setSelectedId(id);
    if (isNarrow) setDrawerOpen(true);
  };
  const selectFirstActionable = () => {
    const first = proposalEntries.find((e) => !proposalIsDecided(e.proposal.state)) ?? proposalEntries[0];
    if (first) selectAndOpen(first.proposal.id);
  };

  function nextActionableAfter(id: string): string | null {
    const idx = proposalEntries.findIndex((e) => e.proposal.id === id);
    if (idx === -1) return null;
    const ordered = [...proposalEntries.slice(idx + 1), ...proposalEntries.slice(0, idx)];
    const next = ordered.find((e) => e.proposal.id !== id && !proposalIsDecided(e.proposal.state));
    return next ? next.proposal.id : null;
  }

  // ── Decisions ──
  const decide = useMutation({
    mutationFn: (payload: { proposalId: string; action: 'ratify' | 'reject' | 'revoke'; ratifiedAmount?: number; ratifiedTreatment?: string; reason?: string }) =>
      api.post<MANIDecisionResponse>(`/binocolo/v1/ma/proposals/${encodeURIComponent(payload.proposalId)}/decision`, {
        action: payload.action,
        ratifiedAmount: payload.ratifiedAmount,
        ratifiedTreatment: payload.ratifiedTreatment,
        reason: payload.reason,
      }),
    onSuccess: async (_res, variables) => {
      toast(
        variables.action === 'ratify' ? 'Rettifica ratificata.' : variables.action === 'reject' ? 'Proposta scartata.' : 'Ratifica revocata.',
        'success',
      );
      setDrafts((prev) => {
        const next = { ...prev };
        delete next[variables.proposalId];
        return next;
      });
      // Auto-advance to the next pending proposal after a ratify/reject; a revoke re-opens the row,
      // so keep it selected for a fresh decision.
      if (variables.action !== 'revoke') {
        const nextId = nextActionableAfter(variables.proposalId);
        if (nextId) setSelectedId(nextId);
        else if (isNarrow) setDrawerOpen(false);
      }
      const entry = entryById.get(variables.proposalId);
      await Promise.all([
        entry ? queryClient.invalidateQueries({ queryKey: ['ma-filing-proposals', entry.filing.id] }) : Promise.resolve(),
        invalidateOverview(),
      ]);
    },
    onError: (error) => toast(filingErrorMessage(error), 'error'),
  });

  // A shared decision mutation surfaces its error in the panel; clear it when the analyst moves to
  // another row so a stale error never bleeds across proposals.
  useEffect(() => {
    decide.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId]);

  // ── Upload (title-row action → file picker; drag&drop stays as an invisible affordance) ──
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const upload = useMutation({
    mutationFn: (file: File) => {
      const body = new FormData();
      body.append('file', file);
      return api.postFormData<MAFilingUploadResponse>(`/binocolo/v1/ma/companies/${encodedKey}/filings`, body);
    },
    onSuccess: (res) => {
      setUploadError(null);
      toast(res.merged ? 'Fascicolo già presente.' : 'Bilancio caricato.', 'success');
      refetchFilings();
      void invalidateOverview();
    },
    onError: (error) => {
      const message = filingErrorMessage(error);
      setUploadError(message);
      toast(message, 'error');
    },
  });

  function submitFile(file: File | undefined) {
    if (!file) return;
    const isPdf = file.type === 'application/pdf' || /\.pdf$/i.test(file.name);
    if (!isPdf) {
      setUploadError('Il file non è un PDF.');
      return;
    }
    if (file.size > UPLOAD_MAX_BYTES) {
      setUploadError('Il file supera i 30 MB.');
      return;
    }
    setUploadError(null);
    upload.mutate(file);
  }

  function onDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    if (upload.isPending) return;
    submitFile(event.dataTransfer.files?.[0]);
  }

  function onPick(event: ChangeEvent<HTMLInputElement>) {
    const input = event.currentTarget;
    submitFile(input.files?.[0]);
    input.value = '';
  }

  // ── Search + acquire ──
  const [searchError, setSearchError] = useState<string | null>(null);
  const search = useMutation({
    mutationFn: () => api.post<MAFilingSearchStartResponse>(`/binocolo/v1/ma/companies/${encodedKey}/filings/search`, {}),
    onSuccess: () => {
      setSearchError(null);
      refetchFilings();
    },
    onError: (error) => {
      const message = filingErrorMessage(error);
      setSearchError(message);
      toast(message, 'error');
    },
  });

  const [selectedBsId, setSelectedBsId] = useState<string>('');
  const acquire = useMutation({
    mutationFn: (balanceSheetId: string) =>
      api.post<MAFilingAcquireResponse>(`/binocolo/v1/ma/companies/${encodedKey}/filings/acquire`, {
        searchId: data?.latestSearch?.id,
        balanceSheetIds: [balanceSheetId],
      }),
    onSuccess: () => {
      setSelectedBsId('');
      toast('Acquisizione avviata.', 'success');
      refetchFilings();
    },
    onError: (error) => toast(filingErrorMessage(error), 'error'),
  });

  // ── Resume a stalled/failed acquisition (re-driven from persisted state, no re-charge) ──
  const [retryingId, setRetryingId] = useState<string | null>(null);
  const [retryErrors, setRetryErrors] = useState<Record<string, string>>({});
  const retry = useMutation({
    mutationFn: (acquisitionId: string) =>
      api.post<MAFilingAcquireResponse>(
        `/binocolo/v1/ma/companies/${encodedKey}/filings/acquisitions/${encodeURIComponent(acquisitionId)}/retry`,
        {},
      ),
    onMutate: (acquisitionId) => setRetryingId(acquisitionId),
    onSuccess: (_res, acquisitionId) => {
      setRetryErrors((prev) => {
        const next = { ...prev };
        delete next[acquisitionId];
        return next;
      });
      toast('Ripresa avviata.', 'success');
      refetchFilings();
    },
    onError: (error, acquisitionId) => {
      const message = filingErrorMessage(error);
      setRetryErrors((prev) => ({ ...prev, [acquisitionId]: message }));
      toast(message, 'error');
    },
    onSettled: () => setRetryingId(null),
  });

  // ── Identity override ──
  const [overrideTarget, setOverrideTarget] = useState<MAFilingRow | null>(null);

  // ── Downloads ──
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  async function downloadPdf(row: MAFilingRow) {
    setDownloadingId(row.id);
    try {
      const blob = await api.getBlob(`/binocolo/v1/ma/filings/${encodeURIComponent(row.id)}/pdf`);
      downloadBlob(blob, `bilancio_${row.closingDate ?? row.id}.pdf`);
    } catch (error) {
      toast(errorLabel(error), 'error');
    } finally {
      setDownloadingId(null);
    }
  }

  // Draft accessors.
  const draftFor = (p: MANIProposalView): ProposalDraft => drafts[p.id] ?? initialDraft(p);
  const isDraftDirty = (p: MANIProposalView): boolean => {
    const d = drafts[p.id];
    if (!d) return false;
    const init = initialDraft(p);
    return d.amount !== init.amount || d.treatment !== init.treatment || d.reason.trim() !== '';
  };
  const updateDraft = (p: MANIProposalView, patch: Partial<ProposalDraft>) => {
    setDrafts((prev) => ({ ...prev, [p.id]: { ...(prev[p.id] ?? initialDraft(p)), ...patch } }));
  };

  // Keyboard nav SCOPED to the queue (no global j/k — those drive the scheda cohort). Arrows move
  // focus between rows and update the inline panel; Enter/Space on a row selects natively.
  function onQueueKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    const container = event.currentTarget;
    const rows = Array.from(container.querySelectorAll<HTMLButtonElement>('[data-prow]'));
    const idx = rows.findIndex((r) => r === document.activeElement);
    if (idx === -1) return;
    event.preventDefault();
    event.stopPropagation();
    const target = event.key === 'ArrowDown' ? rows[idx + 1] : rows[idx - 1];
    if (!target) return;
    target.focus();
    const id = target.getAttribute('data-proposal-id');
    if (id) selectInline(id);
  }

  if (query.isLoading) {
    return (
      <>
        <BlockHeader onUpload={() => fileInputRef.current?.click()} onSearch={() => search.mutate()} searchBusy={false} uploadBusy={false} fileInputRef={fileInputRef} onPick={onPick} disabled />
        <Skeleton rows={4} />
      </>
    );
  }
  if (query.isError) {
    return (
      <>
        <BlockHeader onUpload={() => fileInputRef.current?.click()} onSearch={() => search.mutate()} searchBusy={false} uploadBusy={false} fileInputRef={fileInputRef} onPick={onPick} disabled />
        <div className={styles.statePanel} role="alert">
          <Icon name="triangle-alert" size={20} />
          <p>{filingErrorMessage(query.error, errorLabel(query.error))}</p>
        </div>
      </>
    );
  }
  if (!data) return null;

  const presentBsIds = new Set(data.filings.map((f) => f.balanceSheetId).filter(Boolean) as string[]);
  const presentExercises = new Set(data.filings.map((f) => f.closingDate).filter(Boolean) as string[]);
  const openByBsId = new Map<string, MAFilingAcquisitionView>();
  for (const a of data.acquisitionsOpen) {
    if (a.balanceSheetId) openByBsId.set(a.balanceSheetId, a);
  }

  const search0 = data.latestSearch;
  const searchResolving = search0?.status === 'intent' || search0?.status === 'requested';
  const searchResults = search0?.status === 'results' ? search0.results : [];
  const selectableResults = searchResults.filter(
    (r) =>
      !presentBsIds.has(r.balanceSheetId) &&
      !(r.closingDate && presentExercises.has(r.closingDate)) &&
      !openByBsId.has(r.balanceSheetId),
  );

  // The deposited-fascicoli empty state depends ONLY on there being no fascicolo, no open
  // acquisition and no search — the adjusted-valuation summary above is a separate surface that
  // stays silent on its own when there is nothing to say.
  const nothingYet = data.filings.length === 0 && data.acquisitionsOpen.length === 0 && !search0;

  const selectedEntry = selectedId ? entryById.get(selectedId) : undefined;

  const workPanel = selectedEntry ? (
    <WorkPanel
      entry={selectedEntry}
      draft={draftFor(selectedEntry.proposal)}
      others={sameThemeJumps(selectedEntry, datedGroups)}
      isBaseline={Boolean(baselineExercise && selectedEntry.filing.closingDate === baselineExercise)}
      deciding={decide.isPending}
      decideError={decide.isError ? filingErrorMessage(decide.error) : null}
      onDraftChange={(patch) => updateDraft(selectedEntry.proposal, patch)}
      onSelect={(id) => (isNarrow ? selectAndOpen(id) : selectInline(id))}
      onOpenPdf={() => void downloadPdf(selectedEntry.filing)}
      onRatify={(amount, treatment, reason) =>
        decide.mutate({ proposalId: selectedEntry.proposal.id, action: 'ratify', ratifiedAmount: amount, ratifiedTreatment: treatment, reason })
      }
      onReject={(reason) => decide.mutate({ proposalId: selectedEntry.proposal.id, action: 'reject', reason })}
      onRevoke={(reason) => decide.mutate({ proposalId: selectedEntry.proposal.id, action: 'revoke', reason })}
    />
  ) : (
    <p className={styles.panelEmpty}>Seleziona una rettifica per lavorarla qui.</p>
  );

  return (
    <>
      <BlockHeader
        onUpload={() => fileInputRef.current?.click()}
        onSearch={() => search.mutate()}
        searchBusy={search.isPending || searchResolving}
        uploadBusy={upload.isPending}
        fileInputRef={fileInputRef}
        onPick={onPick}
      />

      <div className={styles.body} onDragOver={(e) => e.preventDefault()} onDrop={onDrop}>
        {/* Adjusted-valuation summary — synthesis above the detail. */}
        <DepositedValuationView
          overview={overview}
          companyKey={companyKey}
          baselineValuation={baselineValuation}
          onGoToFilings={selectFirstActionable}
        />

        {uploadError ? <p className={styles.inlineError}>{uploadError}</p> : null}

        {nothingYet ? (
          <div className={styles.empty}>
            <span className={styles.emptyIcon} aria-hidden="true">
              <Icon name="file-text" size={28} />
            </span>
            <h3>Nessun bilancio depositato</h3>
            <p>Carica un fascicolo o cerca i depositi camerali per questa identità fiscale.</p>
          </div>
        ) : null}

        {/* Undated exceptions (unresolved-date filing, exercise-less acquisition): above the groups. */}
        {undatedFilings.length > 0 || undatedAcquisitions.length > 0 ? (
          <div className={styles.filingList}>
            {undatedFilings.map((row) => (
              <FilingExceptionCard
                key={row.id}
                row={row}
                downloading={downloadingId === row.id}
                onDownload={() => void downloadPdf(row)}
                onOverride={() => setOverrideTarget(row)}
              />
            ))}
            {undatedAcquisitions.map((acq) => (
              <AcquisitionExceptionCard
                key={acq.id}
                acq={acq}
                retrying={retry.isPending && retryingId === acq.id}
                retryBlocked={retry.isPending && retryingId !== acq.id}
                retryError={retryErrors[acq.id]}
                onRetry={() => retry.mutate(acq.id)}
              />
            ))}
          </div>
        ) : null}

        {/* The work surface: year groups + sticky work panel (Drawer below ~1000px). */}
        {datedGroups.length > 0 ? (
          <div className={styles.cols}>
            {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions */}
            <div className={styles.queue} onKeyDown={onQueueKeyDown}>
              {datedGroups.map((group) => (
                <YearGroupSection
                  key={group.key}
                  group={group}
                  isBaseline={Boolean(group.primary && baselineExercise && group.primary.closingDate === baselineExercise)}
                  selectedId={selectedId}
                  downloadingId={downloadingId}
                  retryPendingId={retry.isPending ? retryingId : null}
                  retryErrors={retryErrors}
                  isDraftDirty={isDraftDirty}
                  onSelect={(id) => (isNarrow ? selectAndOpen(id) : selectInline(id))}
                  onDownload={(row) => void downloadPdf(row)}
                  onOverride={(row) => setOverrideTarget(row)}
                  onRetry={(id) => retry.mutate(id)}
                />
              ))}
            </div>
            {/* Sticky work panel on the wide layout only. Below ~1000px it becomes the Drawer below,
                and the aside is NOT rendered — otherwise the panel subtree (ids ma-tr-*, ma-reason-*,
                the MoneyInput) would exist twice in the DOM. The draft lives in the parent's state, so
                switching between aside and drawer never loses in-progress work. */}
            {!isNarrow ? (
              <aside className={styles.panel} aria-label="Pannello di lavoro">
                {workPanel}
              </aside>
            ) : null}
          </div>
        ) : null}

        {/* Camerale search: trigger is in the title row; the results list + single-select acquire is
            unchanged and lives here below the groups. Rendered only when there is something to say. */}
        {Boolean(searchError) ||
        searchResolving ||
        search0?.status === 'unknown' ||
        search0?.status === 'failed' ||
        search0?.status === 'results' ? (
          <div className={styles.subBlock}>
            <h4 className={styles.subHead}>Ricerca depositi camerali</h4>
            {searchResolving ? (
              <p className={styles.subtle}>Ricerca in corso… la lista si aggiorna automaticamente.</p>
            ) : null}
            {searchError ? <p className={styles.inlineError}>{searchError}</p> : null}
            {search0?.status === 'unknown' ? (
              <p className={styles.subtle}>Esito della ricerca non determinato: riprova più tardi.</p>
            ) : null}
            {search0?.status === 'failed' && !searchError ? <p className={styles.subtle}>Ultima ricerca non riuscita.</p> : null}
            {search0?.status === 'results' && searchResults.length === 0 ? (
              <p className={styles.subtle}>Nessun deposito camerale trovato per questa identità fiscale.</p>
            ) : null}

            {searchResults.length > 0 ? (
              <div className={styles.searchResults}>
                {searchResults.map((r) => {
                  const already = presentBsIds.has(r.balanceSheetId) || Boolean(r.closingDate && presentExercises.has(r.closingDate));
                  const open = openByBsId.get(r.balanceSheetId);
                  const muted = already || Boolean(open);
                  return (
                    <label key={r.balanceSheetId} className={`${styles.resultRow} ${muted ? styles.resultRowMuted : ''}`}>
                      <input
                        type="radio"
                        name="ma-filing-acquire"
                        className={styles.radio}
                        checked={selectedBsId === r.balanceSheetId}
                        disabled={muted}
                        onChange={() => setSelectedBsId(r.balanceSheetId)}
                      />
                      <span className={styles.resultLabel}>
                        {exerciseLabel(r.closingDate)} · {filingTypeLabel(r.balanceSheetType)}
                      </span>
                      {already ? <span className={styles.resultTag}>già presente</span> : null}
                      {!already && open?.inflight ? <span className={styles.processingPill}>In acquisizione…</span> : null}
                      {!already && open && !open.inflight ? <span className={styles.resultTag}>da riprendere</span> : null}
                    </label>
                  );
                })}
                <div className={styles.formActions}>
                  <Button
                    size="sm"
                    loading={acquire.isPending}
                    disabled={!selectedBsId || selectableResults.length === 0}
                    onClick={() => selectedBsId && acquire.mutate(selectedBsId)}
                  >
                    Acquisisci
                  </Button>
                </div>
                {acquire.isError ? <p className={styles.inlineError}>{filingErrorMessage(acquire.error)}</p> : null}
              </div>
            ) : null}
          </div>
        ) : null}
      </div>

      {/* Below ~1000px the work panel is a right-side Drawer. The whole subtree is mounted ONLY on
          the narrow breakpoint (the shared Drawer renders its children even when closed), so the panel
          — and its ids — never coexist with the wide-layout aside. */}
      {isNarrow ? (
        <Drawer
          open={drawerOpen && Boolean(selectedEntry)}
          onClose={() => setDrawerOpen(false)}
          size="sm"
          side="right"
          title="Rettifica"
          subtitle={selectedEntry ? `${selectedEntry.year} · ${filingTypeLabel(selectedEntry.filing.balanceSheetType)}` : undefined}
        >
          <div className={styles.drawerBody}>{workPanel}</div>
        </Drawer>
      ) : null}

      <IdentityOverrideModal
        row={overrideTarget}
        onClose={() => setOverrideTarget(null)}
        onConfirmed={() => {
          setOverrideTarget(null);
          refetchFilings();
          void invalidateOverview();
        }}
      />
    </>
  );
}

// ── Block header (title row + right-aligned actions) ──
function BlockHeader({
  onUpload,
  onSearch,
  searchBusy,
  uploadBusy,
  fileInputRef,
  onPick,
  disabled = false,
}: {
  onUpload: () => void;
  onSearch: () => void;
  searchBusy: boolean;
  uploadBusy: boolean;
  fileInputRef: RefObject<HTMLInputElement>;
  onPick: (event: ChangeEvent<HTMLInputElement>) => void;
  disabled?: boolean;
}) {
  return (
    <div className={styles.blockHeader}>
      <span className={styles.blockIndex}>5</span>
      <div className={styles.blockHeadText}>
        <h2 id="scheda-bilanci-title">Bilanci depositati</h2>
        <p>Fascicoli camerali, rettifiche di nota integrativa, valutazione aggiustata.</p>
      </div>
      <div className={styles.headActions}>
        <Button size="sm" variant="secondary" loading={uploadBusy} disabled={disabled} onClick={onUpload} leftIcon={<Icon name="file-up" size={15} />}>
          Carica PDF
        </Button>
        <Button size="sm" variant="secondary" loading={searchBusy} disabled={disabled} onClick={onSearch} leftIcon={<Icon name="search" size={15} />}>
          Cerca depositi camerali
        </Button>
      </div>
      <input ref={fileInputRef} type="file" accept="application/pdf" onChange={onPick} hidden />
    </div>
  );
}

// ── Year group ──
function YearGroupSection({
  group,
  isBaseline,
  selectedId,
  downloadingId,
  retryPendingId,
  retryErrors,
  isDraftDirty,
  onSelect,
  onDownload,
  onOverride,
  onRetry,
}: {
  group: YearGroup;
  isBaseline: boolean;
  selectedId: string | null;
  downloadingId: string | null;
  retryPendingId: string | null;
  retryErrors: Record<string, string>;
  isDraftDirty: (p: MANIProposalView) => boolean;
  onSelect: (id: string) => void;
  onDownload: (row: MAFilingRow) => void;
  onOverride: (row: MAFilingRow) => void;
  onRetry: (id: string) => void;
}) {
  const { primary, proposals, proposalsLoading, exceptionFilings, acquisitions } = group;
  const pending = proposals.filter((p) => !proposalIsDecided(p.state)).length;
  const degradedNote = primary?.status === 'degraded' ? filingExceptionFact(primary) : null;
  const typeLine = primary
    ? `${filingTypeLabel(primary.balanceSheetType)}${primary.origins.length > 0 ? ` · ${primary.origins.map(filingOriginLabel).join(' · ')}` : ''}`
    : filingTypeLabel(acquisitions[0]?.balanceSheetType ?? exceptionFilings[0]?.balanceSheetType);

  return (
    <section className={styles.ygroup} aria-label={`Esercizio ${group.year}`}>
      <div className={styles.yhead}>
        <span className={styles.yYear}>{group.year}</span>
        {isBaseline ? <span className={styles.baseTag}>Baseline</span> : null}
        <span className={styles.yMeta}>{typeLine}</span>
        {primary ? (
          <button
            type="button"
            className={`${styles.pdfBtn} ${pending === 0 ? styles.pdfBtnRight : ''}`}
            title="Scarica il PDF"
            aria-label={`Scarica il PDF dell'esercizio ${group.year}`}
            disabled={downloadingId === primary.id}
            onClick={() => onDownload(primary)}
          >
            <Icon name={downloadingId === primary.id ? 'refresh-cw' : 'file-text'} size={14} />
            PDF
          </button>
        ) : null}
        {pending > 0 ? <span className={styles.pendChip}>{pending} in attesa</span> : null}
      </div>

      <div className={styles.rows}>
        {proposalsLoading && proposals.length === 0 ? (
          <div className={styles.exRow}>
            <span className={styles.subtle}>Lettura rettifiche…</span>
          </div>
        ) : null}

        {primary && !proposalsLoading && proposals.length === 0 && exceptionFilings.length === 0 && acquisitions.length === 0 ? (
          <div className={styles.exRow}>
            <span className={styles.subtle}>Nessuna rettifica proposta dalla nota integrativa.</span>
          </div>
        ) : null}

        {proposals.map((p) => (
          <ProposalRow
            key={p.id}
            proposal={p}
            selected={selectedId === p.id}
            dirty={isDraftDirty(p)}
            onSelect={() => onSelect(p.id)}
          />
        ))}

        {/* NI narrative rows (divider «Lettura nota integrativa») are wired in Fase N3 and slot here,
            beneath the rettifiche, once the narrative endpoint exists. */}

        {degradedNote ? <p className={styles.groupNote}>{degradedNote}</p> : null}

        {exceptionFilings.map((row) => (
          <FilingExceptionRow
            key={row.id}
            row={row}
            downloading={downloadingId === row.id}
            onDownload={() => onDownload(row)}
            onOverride={() => onOverride(row)}
          />
        ))}

        {acquisitions.map((acq) => (
          <AcquisitionExceptionRow
            key={acq.id}
            acq={acq}
            retrying={retryPendingId === acq.id}
            retryBlocked={retryPendingId != null && retryPendingId !== acq.id}
            retryError={retryErrors[acq.id]}
            onRetry={() => onRetry(acq.id)}
          />
        ))}
      </div>
    </section>
  );
}

function ProposalRow({ proposal, selected, dirty, onSelect }: { proposal: MANIProposalView; selected: boolean; dirty: boolean; onSelect: () => void }) {
  const decided = proposalIsDecided(proposal.state);
  const amount = proposalDisplayAmount(proposal);
  const stateText = proposalStateText(proposal.state);
  const chipClass =
    proposal.trattamentoCandidato === 'ebitda' ? styles.chipEbitda : proposal.trattamentoCandidato === 'pfn' ? styles.chipPfn : styles.chipDd;
  return (
    <button
      type="button"
      data-prow
      data-proposal-id={proposal.id}
      className={`${styles.prow} ${decided ? styles.prowDone : ''} ${proposal.state === 'effective' ? styles.prowRatified : ''}`}
      aria-current={selected ? 'true' : undefined}
      onClick={onSelect}
    >
      <span className={styles.dot} />
      <span className={styles.fact}>
        {proposal.label || proposal.fattoOsservato}
        {dirty ? (
          <span className={styles.draftMark} title="Bozza in corso" aria-label="bozza in corso">
            ▲
          </span>
        ) : null}
      </span>
      <span className={`${styles.chip} ${chipClass}`}>{treatmentLabel(proposal.trattamentoCandidato)}</span>
      <span className={styles.amt}>{formatSignedEuro(amount)}</span>
      <span className={styles.rowState}>
        {decided ? '✓ ' : ''}
        {stateText}
        {proposal.pageNo != null ? ` · p.${proposal.pageNo}` : ''}
      </span>
    </button>
  );
}

// ── Work panel ──
interface ProposalDraft {
  amount: string; // canonical wire (signed), "" = empty
  treatment: string; // "", "ebitda", "pfn"
  reason: string;
}

function initialDraft(p: MANIProposalView): ProposalDraft {
  const needsTreatment = proposalNeedsTreatmentChoice(p);
  return {
    amount: !needsTreatment && p.importoRettifica != null ? p.importoRettifica.toFixed(2) : '',
    treatment: needsTreatment ? '' : defaultTreatmentChoice(p),
    reason: '',
  };
}

function WorkPanel({
  entry,
  draft,
  others,
  isBaseline,
  deciding,
  decideError,
  onDraftChange,
  onSelect,
  onOpenPdf,
  onRatify,
  onReject,
  onRevoke,
}: {
  entry: ProposalEntry;
  draft: ProposalDraft;
  others: SameThemeJump[];
  isBaseline: boolean;
  deciding: boolean;
  decideError: string | null;
  onDraftChange: (patch: Partial<ProposalDraft>) => void;
  onSelect: (id: string) => void;
  onOpenPdf: () => void;
  onRatify: (amount: number, treatment: string | undefined, reason: string | undefined) => void;
  onReject: (reason: string | undefined) => void;
  onRevoke: (reason: string | undefined) => void;
}) {
  const { proposal: p, filing, year } = entry;
  const decided = proposalIsDecided(p.state);
  const needsTreatment = proposalNeedsTreatmentChoice(p);
  const amountNum = draft.amount === '' ? null : Number(draft.amount);
  const treatment = needsTreatment ? draft.treatment : draft.treatment || p.trattamentoCandidato;
  const phrase = effectPhrase(treatment, amountNum, year);

  const treatmentMissing = needsTreatment && !draft.treatment;
  const amountValid = amountNum != null && Number.isFinite(amountNum) && amountNum !== 0;
  const canRatify = amountValid && !treatmentMissing;
  const whyDisabled = treatmentMissing
    ? 'Serve il trattamento (è un punto DD: la promozione richiede importo e destinazione).'
    : 'Serve un importo con segno (+ o −) diverso da zero.';

  return (
    <>
      <div className={styles.pctx}>
        <span className={styles.pctxYear}>{year}</span>
        {isBaseline ? <span className={styles.baseTag}>Baseline</span> : null}
        <span className={styles.pctxProv}>
          {filingTypeLabel(filing.balanceSheetType)}
          {filing.origins.length > 0 ? ` · ${filing.origins.map(filingOriginLabel).join(' · ')}` : ''}
        </span>
        <span
          className={`${styles.pstate} ${
            p.state === 'effective' ? styles.pstateOk : p.state === 'rejected' ? styles.pstateNeutral : styles.pstateWait
          }`}
        >
          {proposalStateText(p.state)}
        </span>
      </div>

      <p className={styles.pclaim}>{p.label || p.fattoOsservato}</p>

      {p.quote ? <div className={styles.quote}>«{p.quote}»</div> : null}
      <div className={styles.qmeta}>
        {p.pageNo != null ? <span className={styles.qpage}>pag. {p.pageNo}</span> : null}
        <button type="button" className={styles.openPdf} onClick={onOpenPdf}>
          <Icon name="external-link" size={13} /> Apri il PDF
        </button>
      </div>

      <dl className={styles.facts}>
        {p.importoLordo != null ? (
          <>
            <dt>Importo lordo</dt>
            <dd>{formatEuro(p.importoLordo)}</dd>
          </>
        ) : null}
        <dt>Trattamento candidato</dt>
        <dd>{treatmentLabel(p.trattamentoCandidato)}</dd>
        {incertezzaLabel(p.incertezza) ? (
          <>
            <dt>Incertezza</dt>
            <dd>{incertezzaLabel(p.incertezza)}</dd>
          </>
        ) : null}
      </dl>

      {decided ? (
        <div className={styles.decided}>
          <p className={styles.lastDec}>{decisionSummary(p)}</p>
          {p.decisions.length > 1 ? <DecisionHistory decisions={p.decisions} /> : null}
          {decideError ? <p className={styles.inlineError}>{decideError}</p> : null}
          <div className={styles.pactions}>
            <Button size="sm" variant="secondary" loading={deciding} onClick={() => onRevoke(draft.reason.trim() || undefined)}>
              Revoca
            </Button>
          </div>
        </div>
      ) : (
        <div className={styles.form}>
          <MoneyInput label="Rettifica (+/−)" value={draft.amount} onChange={(v) => onDraftChange({ amount: v })} allowNegative />
          {needsTreatment ? (
            <div className={styles.field}>
              <label htmlFor={`ma-tr-${p.id}`}>Trattamento</label>
              <select id={`ma-tr-${p.id}`} className={styles.select} value={draft.treatment} onChange={(e) => onDraftChange({ treatment: e.target.value })}>
                <option value="">— scegli —</option>
                <option value="ebitda">EBITDA</option>
                <option value="pfn">PFN</option>
              </select>
            </div>
          ) : null}
          <div className={styles.field}>
            <label htmlFor={`ma-reason-${p.id}`}>Motivo (opzionale)</label>
            <input
              id={`ma-reason-${p.id}`}
              type="text"
              className={styles.textInput}
              value={draft.reason}
              onChange={(e) => onDraftChange({ reason: e.target.value })}
              maxLength={300}
              placeholder="Nota per la traccia decisionale"
            />
          </div>

          <div className={`${styles.effect} ${phrase ? styles.effectOn : ''}`}>
            {phrase ?? 'L’anteprima dell’effetto appare qui mentre digiti.'}
          </div>

          <div className={styles.pactions}>
            <Button
              size="sm"
              loading={deciding}
              disabled={!canRatify}
              onClick={() => onRatify(Number(draft.amount), needsTreatment ? draft.treatment : undefined, draft.reason.trim() || undefined)}
            >
              Ratifica
            </Button>
            <Button size="sm" variant="secondary" loading={deciding} onClick={() => onReject(draft.reason.trim() || undefined)}>
              Scarta
            </Button>
          </div>
          {!canRatify ? <p className={styles.whyDisabled}>{whyDisabled}</p> : null}
          {decideError ? <p className={styles.inlineError}>{decideError}</p> : null}
          {p.decisions.length > 0 ? <DecisionHistory decisions={p.decisions} /> : null}
        </div>
      )}

      <div className={styles.others}>
        <p className={styles.othersLabel}>Stesso tema negli altri esercizi</p>
        {others.length > 0 ? (
          <div className={styles.othersBtns}>
            {others.map((o) =>
              o.proposalId ? (
                <button key={o.year} type="button" className={styles.obtn} onClick={() => onSelect(o.proposalId as string)}>
                  {o.year}
                </button>
              ) : (
                <span key={o.year} className={`${styles.obtn} ${styles.obtnMiss}`}>
                  {o.year}: non presente
                </span>
              ),
            )}
          </div>
        ) : (
          <p className={styles.subtle}>Nessun altro esercizio depositato.</p>
        )}
      </div>
    </>
  );
}

function DecisionHistory({ decisions }: { decisions: MANIDecisionView[] }) {
  return (
    <details className={styles.hist}>
      <summary>Storico ({decisions.length})</summary>
      <ul className={styles.histList}>
        {decisions.map((d) => (
          <li key={d.id}>{decisionLine(d)}</li>
        ))}
      </ul>
    </details>
  );
}

// ── Same-theme jumps (heuristic in filingFacts.proposalThemeKey) ──
interface SameThemeJump {
  year: string;
  proposalId: string | null; // null → "non presente" for that exercise
}

function sameThemeJumps(entry: ProposalEntry, groups: YearGroup[]): SameThemeJump[] {
  const key = proposalThemeKey(entry.proposal);
  const out: SameThemeJump[] = [];
  for (const g of groups) {
    if (g.year === entry.year) continue;
    const match = g.primary ? g.proposals.find((p) => proposalThemeKey(p) === key) : undefined;
    out.push({ year: g.year, proposalId: match ? match.id : null });
  }
  return out;
}

function decisionSummary(p: MANIProposalView): string {
  const last = [...p.decisions].reverse().find((d) => d.action === 'ratify' || d.action === 'reject');
  if (!last) return proposalStateText(p.state);
  return decisionLine(last);
}

function decisionLine(d: MANIDecisionView): string {
  const action = d.action === 'ratify' ? 'Ratificata' : d.action === 'reject' ? 'Scartata' : 'Revocata';
  const parts: string[] = [action];
  if (d.actor) parts.push(`da ${d.actor}`);
  if (d.action === 'ratify') {
    if (d.ratifiedTreatment) parts.push(treatmentLabel(d.ratifiedTreatment));
    if (d.ratifiedAmount != null) parts.push(formatSignedEuro(d.ratifiedAmount));
  }
  if (d.autoReconfirmedFrom) parts.push('riconfermata automaticamente');
  let line = parts.join(' · ');
  if (d.reason) line += ` — «${d.reason}»`;
  const when = dateLabel(d.createdAt);
  if (when) line += ` (${when})`;
  return line;
}

// ── Exception rows/cards (acquisitions + non-ready filings) ──
function FilingExceptionInner({ row, downloading, onDownload, onOverride }: { row: MAFilingRow; downloading: boolean; onDownload: () => void; onOverride: () => void }) {
  const badge = filingExceptionBadge(row);
  const processing = isFilingInProgress(row.status) ? filingProcessingLabel(row.status) : null;
  const fact = filingExceptionFact(row);
  const canDownload = row.status === 'ready' || row.status === 'degraded' || row.status === 'identity_blocked' || row.status === 'failed';
  return (
    <>
      <div className={styles.exMain}>
        <span className={styles.rowExercise}>{exerciseLabel(row.closingDate)}</span>
        {fact ? <span className={styles.exFact}>{fact}</span> : null}
      </div>
      <div className={styles.exActions}>
        {processing ? <span className={styles.processingPill}>{processing}</span> : null}
        {badge ? <StatusBadge value={badge.label} variant={badge.variant} dot={false} /> : null}
        {filingIsIdentityOverridable(row) ? (
          <Button size="sm" variant="secondary" onClick={onOverride}>
            Conferma identità
          </Button>
        ) : null}
        {canDownload ? (
          <button type="button" className={styles.iconAction} title="Scarica il PDF" aria-label="Scarica il PDF" disabled={downloading} onClick={onDownload}>
            <Icon name={downloading ? 'refresh-cw' : 'file-text'} size={16} />
          </button>
        ) : null}
      </div>
    </>
  );
}

function FilingExceptionRow(props: { row: MAFilingRow; downloading: boolean; onDownload: () => void; onOverride: () => void }) {
  return (
    <div className={styles.exRow}>
      <FilingExceptionInner {...props} />
    </div>
  );
}

function FilingExceptionCard(props: { row: MAFilingRow; downloading: boolean; onDownload: () => void; onOverride: () => void }) {
  return (
    <div className={styles.filingRow}>
      <div className={styles.filingTop}>
        <FilingExceptionInner {...props} />
      </div>
    </div>
  );
}

function AcquisitionExceptionInner({
  acq,
  retrying,
  retryBlocked,
  onRetry,
}: {
  acq: MAFilingAcquisitionView;
  retrying: boolean;
  retryBlocked: boolean;
  onRetry: () => void;
}) {
  const inProgress = acquisitionInProgress(acq);
  const badge = inProgress ? null : acquisitionExceptionBadge(acq);
  return (
    <>
      <div className={styles.exMain}>
        <span className={styles.rowExercise}>{exerciseLabel(acq.closingDate)}</span>
        {!inProgress ? <span className={styles.exFact}>{acquisitionExceptionFact(acq)}</span> : null}
      </div>
      <div className={styles.exActions}>
        {inProgress ? <span className={styles.processingPill}>{acquisitionProgressLabel(acq)}</span> : null}
        {badge ? <StatusBadge value={badge.label} variant={badge.variant} dot={false} /> : null}
        {!inProgress ? (
          <Button size="sm" variant="secondary" loading={retrying} disabled={acq.inflight || retryBlocked} onClick={onRetry}>
            Riprendi
          </Button>
        ) : null}
      </div>
    </>
  );
}

function AcquisitionExceptionRow({
  acq,
  retrying,
  retryBlocked,
  retryError,
  onRetry,
}: {
  acq: MAFilingAcquisitionView;
  retrying: boolean;
  retryBlocked: boolean;
  retryError?: string;
  onRetry: () => void;
}) {
  return (
    <>
      <div className={styles.exRow}>
        <AcquisitionExceptionInner acq={acq} retrying={retrying} retryBlocked={retryBlocked} onRetry={onRetry} />
      </div>
      {retryError ? <p className={styles.groupNote}>{retryError}</p> : null}
    </>
  );
}

function AcquisitionExceptionCard({
  acq,
  retrying,
  retryBlocked,
  retryError,
  onRetry,
}: {
  acq: MAFilingAcquisitionView;
  retrying: boolean;
  retryBlocked: boolean;
  retryError?: string;
  onRetry: () => void;
}) {
  return (
    <div className={styles.filingRow}>
      <div className={styles.filingTop}>
        <AcquisitionExceptionInner acq={acq} retrying={retrying} retryBlocked={retryBlocked} onRetry={onRetry} />
      </div>
      {retryError ? <p className={styles.inlineError}>{retryError}</p> : null}
    </div>
  );
}

// Identity override: one-shot, irreversible acceptance of an unreadable-identity filing. Strong
// confirm (mandatory reason + explicit acknowledgement) because it cannot be undone.
function IdentityOverrideModal({
  row,
  onClose,
  onConfirmed,
}: {
  row: MAFilingRow | null;
  onClose: () => void;
  onConfirmed: () => void;
}) {
  const api = useApiClient();
  const { toast } = useToast();
  const [reason, setReason] = useState('');
  const [acknowledged, setAcknowledged] = useState(false);

  const override = useMutation({
    mutationFn: (filingId: string) =>
      api.post<MAFilingIdentityOverrideResponse>(`/binocolo/v1/ma/filings/${encodeURIComponent(filingId)}/identity-override`, {
        reason: reason.trim(),
      }),
    onSuccess: () => {
      toast('Identità confermata: elaborazione ripresa.', 'success');
      onConfirmed();
    },
    onError: (error) => toast(filingErrorMessage(error), 'error'),
  });

  useEffect(() => {
    if (row) {
      setReason('');
      setAcknowledged(false);
      override.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [row?.id]);

  const canConfirm = reason.trim().length > 0 && acknowledged && !override.isPending;

  return (
    <Modal open={row != null} onClose={onClose} title="Conferma identità del fascicolo" size="sm" dismissible={!override.isPending}>
      <div className={styles.overrideBody}>
        <p>
          La pagina 1 non è leggibile: confermi manualmente che il documento appartiene a questa identità fiscale. L’operazione è{' '}
          <strong>irreversibile</strong> e riprende l’elaborazione del fascicolo.
        </p>
        <div className={styles.field}>
          <label htmlFor="ma-identity-reason">Motivo</label>
          <input
            id="ma-identity-reason"
            type="text"
            className={styles.textInput}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            maxLength={300}
            placeholder="Perché il documento è di questa azienda"
          />
        </div>
        <label className={styles.ackRow}>
          <input type="checkbox" checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)} />
          <span>Confermo l’identità; l’operazione non è revocabile.</span>
        </label>
        {override.isError ? <p className={styles.inlineError}>{filingErrorMessage(override.error)}</p> : null}
        <div className={styles.formActions}>
          <Button variant="secondary" onClick={onClose} disabled={override.isPending}>
            Annulla
          </Button>
          <Button disabled={!canConfirm} loading={override.isPending} onClick={() => row && override.mutate(row.id)}>
            Conferma identità
          </Button>
        </div>
      </div>
    </Modal>
  );
}
