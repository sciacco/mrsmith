import { Button, Icon, Modal, Skeleton, StatusBadge, useToast } from '@mrsmith/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState, type ChangeEvent, type DragEvent } from 'react';
import { useApiClient } from '../../api/client';
import type {
  MAFilingAcquireResponse,
  MAFilingAcquisitionView,
  MAFilingIdentityOverrideResponse,
  MAFilingRow,
  MAFilingSearchStartResponse,
  MAFilingUploadResponse,
  MAFilingsResponse,
} from '../../api/types';
import { dateLabel, downloadBlob, errorLabel } from '../../pages/ricerche/helpers';
import { ProposalsPanel } from './ProposalsPanel';
import {
  acquisitionExceptionBadge,
  acquisitionExceptionFact,
  acquisitionInProgress,
  acquisitionProgressLabel,
  exerciseLabel,
  filingErrorMessage,
  filingExceptionBadge,
  filingExceptionFact,
  filingIsIdentityOverridable,
  filingOriginLabel,
  filingProcessingLabel,
  filingTypeLabel,
  isFilingInProgress,
} from './filingFacts';
import styles from './filings.module.css';

const UPLOAD_MAX_BYTES = 30 * 1024 * 1024;

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

// Deliverable C: the "Bilanci depositati" block body. Deposited fascicoli (list + download +
// per-filing NI ratification), analyst upload, and on-demand camerale search. Exception-only,
// facts-only, no prices. Polling runs ONLY while there is work in flight and never re-triggers an
// entrance animation (there are none on these rows).
export function FilingsBlock({ companyKey, baselineExercise }: { companyKey: string; baselineExercise?: string }) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const encodedKey = encodeURIComponent(companyKey);

  const filingsKey = ['ma-filings', companyKey];
  const query = useQuery({
    queryKey: filingsKey,
    enabled: Boolean(companyKey),
    queryFn: () => api.get<MAFilingsResponse>(`/binocolo/v1/ma/companies/${encodedKey}/filings`),
    // Poll ONLY while work is in flight: any filing non-terminal, an acquisition inflight, or the
    // latest search still resolving (intent/requested). The state is always read from the filings
    // query — never the searchId directly (a dedup-race can return an orphan intent id). A calm
    // surface (everything terminal) stops polling entirely.
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

  // ── Upload ──
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

  function onDrop(event: DragEvent<HTMLLabelElement>) {
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

  // ── Resume a stalled/failed acquisition ──
  // The paid acquisition is re-driven from its persisted state (no re-charge by default); the
  // error is kept inline per row (persistent) so the analyst can act on it, not only via the toast.
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

  // ── Expand → proposals ──
  const [expandedId, setExpandedId] = useState<string | null>(null);

  if (query.isLoading) return <Skeleton rows={4} />;
  if (query.isError) {
    return (
      <div className={styles.statePanel} role="alert">
        <Icon name="triangle-alert" size={20} />
        <p>{filingErrorMessage(query.error, errorLabel(query.error))}</p>
      </div>
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

  const uploadDropzone = (
    <label
      className={`${styles.dropzone} ${upload.isPending ? styles.dropzoneDisabled : ''}`}
      onDragOver={(event) => event.preventDefault()}
      onDrop={onDrop}
    >
      <Icon name={upload.isPending ? 'refresh-cw' : 'file-up'} size={20} aria-hidden="true" />
      <span>{upload.isPending ? 'Caricamento in corso…' : 'Trascina un PDF o scegli un file'}</span>
      <small>PDF, fino a 30 MB</small>
      <input type="file" accept="application/pdf" onChange={onPick} disabled={upload.isPending} />
    </label>
  );

  return (
    <div className={styles.body}>
      {/* Fascicoli — empty only when there is neither a filing nor an open acquisition (an
          in-progress/stalled acquisition must never leave the block looking empty). */}
      {data.filings.length === 0 && data.acquisitionsOpen.length === 0 ? (
        <div className={styles.empty}>
          <span className={styles.emptyIcon} aria-hidden="true">
            <Icon name="file-text" size={28} />
          </span>
          <h3>Nessun bilancio depositato</h3>
          <p>Carica un fascicolo o cerca i depositi camerali per questa identità fiscale.</p>
        </div>
      ) : data.filings.length > 0 ? (
        <div className={styles.filingList}>
          {data.filings.map((row) => {
            const badge = filingExceptionBadge(row);
            const processing = isFilingInProgress(row.status) ? filingProcessingLabel(row.status) : null;
            const fact = filingExceptionFact(row);
            const expandable = row.status === 'ready' || row.status === 'degraded';
            const expanded = expandedId === row.id;
            const isBaseline = Boolean(baselineExercise && row.closingDate === baselineExercise);
            return (
              <div key={row.id} className={styles.filingRow}>
                <div className={styles.filingTop}>
                  {expandable ? (
                    <button
                      type="button"
                      className={styles.rowMain}
                      aria-expanded={expanded}
                      onClick={() => setExpandedId(expanded ? null : row.id)}
                    >
                      <Icon name={expanded ? 'chevron-down' : 'chevron-right'} size={15} aria-hidden="true" />
                      <RowFacts row={row} isBaseline={isBaseline} />
                    </button>
                  ) : (
                    <div className={styles.rowMainStatic}>
                      <RowFacts row={row} isBaseline={isBaseline} />
                    </div>
                  )}
                  <div className={styles.rowSide}>
                    {processing ? <span className={styles.processingPill}>{processing}</span> : null}
                    {badge ? <StatusBadge value={badge.label} variant={badge.variant} dot={false} /> : null}
                    <button
                      type="button"
                      className={styles.iconAction}
                      title="Scarica il PDF"
                      aria-label="Scarica il PDF"
                      disabled={downloadingId === row.id}
                      onClick={() => void downloadPdf(row)}
                    >
                      <Icon name={downloadingId === row.id ? 'refresh-cw' : 'file-text'} size={16} />
                    </button>
                  </div>
                </div>
                {fact ? (
                  <div className={styles.filingFact}>
                    <span>{fact}</span>
                    {filingIsIdentityOverridable(row) ? (
                      <Button size="sm" variant="secondary" onClick={() => setOverrideTarget(row)}>
                        Conferma identità
                      </Button>
                    ) : null}
                  </div>
                ) : null}
                {expanded ? <ProposalsPanel filingId={row.id} companyKey={companyKey} /> : null}
              </div>
            );
          })}
        </div>
      ) : null}

      {/* Acquisizioni aperte — procurements not yet bound to a filing: in-progress (calm pill) or
          stalled/failed (exception + Riprendi). Sits between the fascicoli and the search so an
          "in acquisizione" — the only in-flight signal — never disappears with the search results. */}
      {data.acquisitionsOpen.length > 0 ? (
        <div className={styles.filingList}>
          {data.acquisitionsOpen.map((acq) => {
            const inProgress = acquisitionInProgress(acq);
            const badge = inProgress ? null : acquisitionExceptionBadge(acq);
            const retryError = retryErrors[acq.id];
            return (
              <div key={acq.id} className={styles.filingRow}>
                <div className={styles.filingTop}>
                  <div className={styles.rowMainStatic}>
                    <span className={styles.rowFacts}>
                      <span className={styles.rowExercise}>{exerciseLabel(acq.closingDate)}</span>
                      <span className={styles.rowMeta}>{filingTypeLabel(acq.balanceSheetType)}</span>
                    </span>
                  </div>
                  <div className={styles.rowSide}>
                    {inProgress ? <span className={styles.processingPill}>{acquisitionProgressLabel(acq)}</span> : null}
                    {badge ? <StatusBadge value={badge.label} variant={badge.variant} dot={false} /> : null}
                  </div>
                </div>
                {!inProgress ? (
                  <div className={styles.filingFact}>
                    <span>{acquisitionExceptionFact(acq)}</span>
                    <Button
                      size="sm"
                      variant="secondary"
                      loading={retry.isPending && retryingId === acq.id}
                      disabled={acq.inflight || (retry.isPending && retryingId !== acq.id)}
                      onClick={() => retry.mutate(acq.id)}
                    >
                      Riprendi
                    </Button>
                  </div>
                ) : null}
                {retryError ? <p className={styles.inlineError}>{retryError}</p> : null}
              </div>
            );
          })}
        </div>
      ) : null}

      {/* Carica un bilancio */}
      <div className={styles.subBlock}>
        <h4 className={styles.subHead}>Carica un bilancio</h4>
        {uploadDropzone}
        {uploadError ? <p className={styles.inlineError}>{uploadError}</p> : null}
      </div>

      {/* Ricerca depositi camerali */}
      <div className={styles.subBlock}>
        <h4 className={styles.subHead}>Ricerca depositi camerali</h4>
        {searchResolving ? (
          <p className={styles.subtle}>Ricerca in corso… la lista si aggiorna automaticamente.</p>
        ) : (
          <Button size="sm" variant="secondary" loading={search.isPending} onClick={() => search.mutate()}>
            Cerca depositi camerali
          </Button>
        )}
        {searchError ? <p className={styles.inlineError}>{searchError}</p> : null}
        {search0?.status === 'unknown' ? (
          <p className={styles.subtle}>Esito della ricerca non determinato: riprova più tardi.</p>
        ) : null}
        {search0?.status === 'failed' && !searchError ? (
          <p className={styles.subtle}>Ultima ricerca non riuscita.</p>
        ) : null}

        {searchResults.length > 0 ? (
          <div className={styles.searchResults}>
            {searchResults.map((r) => {
              const already = presentBsIds.has(r.balanceSheetId) || Boolean(r.closingDate && presentExercises.has(r.closingDate));
              // An open acquisition already owns this deposit: don't offer a duplicate acquire — the
              // acquisitions section above shows its progress (or a Riprendi for a stalled one).
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

      <IdentityOverrideModal
        row={overrideTarget}
        onClose={() => setOverrideTarget(null)}
        onConfirmed={() => {
          setOverrideTarget(null);
          refetchFilings();
          void invalidateOverview();
        }}
      />
    </div>
  );
}

function RowFacts({ row, isBaseline }: { row: MAFilingRow; isBaseline: boolean }) {
  return (
    <span className={styles.rowFacts}>
      <span className={styles.rowExercise}>
        {exerciseLabel(row.closingDate)}
        {isBaseline ? <span className={styles.baselineMarker}>Baseline</span> : null}
      </span>
      <span className={styles.rowMeta}>
        {filingTypeLabel(row.balanceSheetType)}
        {row.origins.length > 0 ? ` · ${row.origins.map(filingOriginLabel).join(' · ')}` : ''}
        {` · ${dateLabel(row.createdAt)}`}
      </span>
    </span>
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

  // Reset transient state whenever the modal opens on a new row.
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
          La pagina 1 non è leggibile: confermi manualmente che il documento appartiene a questa identità fiscale. L’operazione
          è <strong>irreversibile</strong> e riprende l’elaborazione del fascicolo.
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
