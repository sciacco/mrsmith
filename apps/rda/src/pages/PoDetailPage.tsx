import { Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { getRdaQuoteThreshold } from '../runtime-config';
import {
  useBudgets,
  usePatchPaymentMethod,
  usePatchPO,
  usePaymentMethodDefault,
  usePaymentMethods,
  usePODetail,
  usePOComments,
  useProvider,
  useProviders,
  useRdaDownloads,
  useTransitionMutation,
  type TransitionAction,
} from '../api/queries';
import type { PoDetail, ProviderReference, ProviderSummary } from '../api/types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { CommentsPanel } from '../components/CommentsPanel';
import { headerStateFromPO, PoHeaderEditModal, PoHeaderSummary, type HeaderFormState } from '../components/PoHeaderForm';
import { POCommandBar } from '../components/POCommandBar';
import { POReadinessPanel, POWorkflowRail } from '../components/POWorkspacePanels';
import { PoTabs } from '../components/PoTabs';
import { ProviderRequestModal } from '../components/ProviderRequestModal';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { countQuoteAttachments } from '../lib/attachments';
import { coerceID, downloadBlob, isRequester, parseMistraMoney } from '../lib/format';
import { buildPatchPOPayload } from '../lib/po-payload';
import { buildPOReadinessItems, buildTabBadges, selectedModeID } from '../lib/po-detail-view-model';
import { buildPaymentMethodOptions, paymentCodeFromProvider, preferredPaymentMethodCode, requiresPaymentMethodVerification } from '../lib/payment-options';
import { PO_STATES } from '../lib/state-labels';

function afterTransitionRoute(po: PoDetail, action: TransitionAction): string | null {
  if (action === 'send-to-provider') return '/rda';
  if (action === 'approve' || action === 'reject') {
    if (po.state === PO_STATES.PENDING_APPROVAL) return '/rda/inbox/level1-2';
    if (po.state === PO_STATES.PENDING_APPROVAL_PAYMENT_METHOD) return '/rda/inbox/payment-method';
    if (po.state === PO_STATES.PENDING_APPROVAL_NO_LEASING) return '/rda/inbox/no-leasing';
  }
  if (action.startsWith('leasing/')) return '/rda/inbox/leasing';
  if (action.startsWith('budget-increment/')) return '/rda/inbox/budget-increment';
  return null;
}

function recipientIDs(recipients?: ProviderReference[]): number[] {
  return (recipients ?? []).map((ref) => ref.id).filter((id): id is number => id != null);
}

function providerRefs(provider?: ProviderSummary): ProviderReference[] {
  if (!provider) return [];
  if (provider.refs?.length) return provider.refs;
  return provider.ref ? [provider.ref] : [];
}

export function PoDetailPage() {
  const { poId: rawPoId } = useParams();
  const poId = coerceID(rawPoId);
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { user } = useOptionalAuth();
  const { toast } = useToast();
  const [header, setHeader] = useState<HeaderFormState | null>(null);
  const [recipientDraftIds, setRecipientDraftIds] = useState<number[]>([]);
  const [submitConfirm, setSubmitConfirm] = useState(false);
  const [requestedProviders, setRequestedProviders] = useState<ProviderSummary[]>([]);
  const [providerRequestOpen, setProviderRequestOpen] = useState(false);
  const [providerRequestSearch, setProviderRequestSearch] = useState('');
  const [selectedActionMode, setSelectedActionMode] = useState<string>('read_only');
  const [headerModalOpen, setHeaderModalOpen] = useState(false);
  const [editHeader, setEditHeader] = useState<HeaderFormState | null>(null);

  const po = usePODetail(poId);
  const comments = usePOComments(poId);
  const budgets = useBudgets();
  const providers = useProviders();
  const methods = usePaymentMethods();
  const defaultPayment = usePaymentMethodDefault();
  const persistedProviderID = typeof header?.provider_id === 'number' ? header.provider_id : po.data?.provider?.id ?? null;
  const editProviderID = typeof editHeader?.provider_id === 'number' ? editHeader.provider_id : null;
  const provider = useProvider(persistedProviderID);
  const editProvider = useProvider(editProviderID && editProviderID !== persistedProviderID ? editProviderID : null);
  const patchPO = usePatchPO(poId);
  const patchPayment = usePatchPaymentMethod(poId);
  const transition = useTransitionMutation();
  const downloads = useRdaDownloads();
  const providerOptions = useMemo(() => {
    const byID = new Map<number, ProviderSummary>();
    for (const providerItem of providers.data ?? []) byID.set(providerItem.id, providerItem);
    if (po.data?.provider) byID.set(po.data.provider.id, po.data.provider);
    if (provider.data) byID.set(provider.data.id, provider.data);
    if (editProvider.data) byID.set(editProvider.data.id, editProvider.data);
    for (const providerItem of requestedProviders) byID.set(providerItem.id, providerItem);
    return Array.from(byID.values());
  }, [editProvider.data, po.data?.provider, provider.data, providers.data, requestedProviders]);
  const selectedPersistedProvider = providerOptions.find((providerItem) => providerItem.id === header?.provider_id);
  const selectedEditProvider = providerOptions.find((providerItem) => providerItem.id === editHeader?.provider_id);
  const fullProviderForDraft = provider.data ?? selectedPersistedProvider ?? po.data?.provider;
  const fullProviderForEdit = editProvider.data ?? (editProviderID === persistedProviderID ? provider.data : undefined) ?? selectedEditProvider ?? fullProviderForDraft;
  const providerChangedForDraft = Boolean(header && po.data && header.provider_id !== (po.data.provider?.id ?? ''));
  const displayedRecipients = useMemo(() => {
    if (!providerChangedForDraft) return po.data?.recipients;
    const selected = new Set(recipientDraftIds);
    return providerRefs(fullProviderForDraft).filter((ref) => ref.id != null && selected.has(ref.id));
  }, [fullProviderForDraft, po.data?.recipients, providerChangedForDraft, recipientDraftIds]);
  const detailWithDisplayedRecipients = useMemo(() => {
    if (!po.data) return null;
    if (displayedRecipients === po.data.recipients) return po.data;
    return { ...po.data, recipients: displayedRecipients };
  }, [displayedRecipients, po.data]);

  useEffect(() => {
    if (!po.data) return;
    setHeader(headerStateFromPO(po.data));
    setRecipientDraftIds(recipientIDs(po.data.recipients));
  }, [po.data]);

  useEffect(() => {
    if (!po.data?.action_model) return;
    setSelectedActionMode((current) => selectedModeID(po.data?.action_model, current));
  }, [po.data?.action_model]);

  if (poId == null) return <Navigate to="/rda" replace />;

  if (po.error) {
    return (
      <main className="rdaPage">
        <section className="stateCard">
          <p className="eyebrow">Richiesta</p>
          <h1>Richiesta non disponibile</h1>
          <p>La richiesta selezionata non puo essere caricata in questo momento.</p>
        </section>
      </main>
    );
  }

  if (po.isLoading) {
    return (
      <main className="rdaPage">
        <section className="stateCard"><Skeleton rows={10} /></section>
      </main>
    );
  }

  if (!po.data || !header) {
    return (
      <main className="rdaPage">
        <section className="stateCard">
          <p className="eyebrow">Richiesta</p>
          <h1>Richiesta non disponibile</h1>
          <p>La richiesta selezionata non puo essere caricata in questo momento.</p>
        </section>
      </main>
    );
  }

  const detail = po.data;
  const currentHeader = header;
  const initialHeader = headerStateFromPO(detail);
  const providerChanged = currentHeader.provider_id !== initialHeader.provider_id;
  const requester = isRequester(detail, user?.email);
  const draftEditable = detail.state === PO_STATES.DRAFT && requester;
  const paymentEditable = detail.state === PO_STATES.PENDING_APPROVAL_PAYMENT_METHOD && requester;
  const headerCanEdit = draftEditable || paymentEditable;
  const headerEditDisabledReason = requester
    ? 'I dati di testata non sono modificabili in questo stato.'
    : 'Solo il richiedente puo modificare i dati della richiesta.';
  const total = parseMistraMoney(detail.total_price);
  const quoteRuleBlocked = total >= getRdaQuoteThreshold() && countQuoteAttachments(detail.attachments) < 2;
  const canSubmit = draftEditable && (detail.rows?.length ?? 0) > 0 && !quoteRuleBlocked;
  const fullProvider = fullProviderForDraft ?? detail.provider;
  const editProviderForPayment = fullProviderForEdit ?? detail.provider;
  const providerDefault = paymentCodeFromProvider(fullProvider);
  const editProviderDefault = paymentCodeFromProvider(editProviderForPayment);
  const cdlanDefault = defaultPayment.data?.code ?? '';
  const paymentOptions = buildPaymentMethodOptions({
    methods: methods.data ?? [],
    providerDefault: fullProvider?.default_payment_method,
    cdlanDefaultCode: cdlanDefault,
    currentCode: currentHeader.payment_method,
  });
  const editPaymentOptions = buildPaymentMethodOptions({
    methods: methods.data ?? [],
    providerDefault: editProviderForPayment?.default_payment_method,
    cdlanDefaultCode: cdlanDefault,
    currentCode: (editHeader ?? currentHeader).payment_method,
  });
  const paymentRequiresVerification = requiresPaymentMethodVerification(currentHeader.payment_method, providerDefault, cdlanDefault);
  const editPaymentRequiresVerification = requiresPaymentMethodVerification((editHeader ?? currentHeader).payment_method, editProviderDefault, cdlanDefault);
  const readinessItems = buildPOReadinessItems(detailWithDisplayedRecipients ?? detail, currentHeader, {
    provider: fullProvider,
    recipients: displayedRecipients,
    quoteThreshold: getRdaQuoteThreshold(),
  });
  const tabBadges = buildTabBadges(detailWithDisplayedRecipients ?? detail, currentHeader, initialHeader, fullProvider, getRdaQuoteThreshold());

  function handleProviderChange(value: number | '') {
    const nextProvider = providerOptions.find((item) => item.id === value);
    setEditHeader((current) =>
      current
        ? {
            ...current,
            provider_id: value,
            payment_method: preferredPaymentMethodCode(nextProvider, cdlanDefault),
          }
        : current,
    );
  }

  function handleProviderRequestCreated(providerCreated: ProviderSummary) {
    setRequestedProviders((current) => {
      const withoutProvider = current.filter((item) => item.id !== providerCreated.id);
      return [...withoutProvider, providerCreated];
    });
    setEditHeader((current) =>
      current
        ? {
            ...current,
            provider_id: providerCreated.id,
            payment_method: preferredPaymentMethodCode(providerCreated, cdlanDefault),
          }
        : current,
    );
  }

  function openHeaderModal() {
    if (!headerCanEdit) return;
    setEditHeader(currentHeader);
    setHeaderModalOpen(true);
  }

  function closeHeaderModal() {
    setHeaderModalOpen(false);
    setEditHeader(null);
  }

  async function saveHeader(nextHeader: HeaderFormState) {
    if (!draftEditable) return;
    try {
      const nextProviderChanged = nextHeader.provider_id !== initialHeader.provider_id;
      await patchPO.mutateAsync(buildPatchPOPayload(nextHeader, budgets.data ?? [], nextProviderChanged));
      if (nextProviderChanged) setRecipientDraftIds([]);
      toast('Bozza aggiornata');
      closeHeaderModal();
    } catch {
      toast('Salvataggio non riuscito', 'error');
    }
  }

  async function savePaymentMethod(paymentMethod = currentHeader.payment_method) {
    if (!paymentEditable) return;
    try {
      await patchPayment.mutateAsync(paymentMethod);
      toast('Metodo pagamento aggiornato');
      closeHeaderModal();
    } catch {
      toast('Salvataggio non riuscito', 'error');
    }
  }

  async function saveHeaderModal(nextHeader: HeaderFormState) {
    if (draftEditable) {
      await saveHeader(nextHeader);
      return;
    }
    if (paymentEditable) {
      await savePaymentMethod(nextHeader.payment_method);
    }
  }

  async function submitPO() {
    try {
      await transition.mutateAsync({ id: detail.id, action: 'submit' });
      toast('Richiesta mandata in approvazione');
      setSubmitConfirm(false);
      navigate('/rda');
    } catch {
      toast('Invio non riuscito', 'error');
    }
  }

  async function runTransition(action: TransitionAction) {
    try {
      const incrementPromise = searchParams.get('increment_promise') ?? detail.budget_increment_needed;
      const body = action.startsWith('budget-increment/') && incrementPromise ? { increment_promise: incrementPromise } : undefined;
      await transition.mutateAsync({ id: detail.id, action, body });
      toast('Operazione completata');
      const next = afterTransitionRoute(detail, action);
      if (next) navigate(next);
    } catch {
      toast('Operazione non riuscita', 'error');
    }
  }

  async function saveRecipients(ids: number[]) {
    try {
      setRecipientDraftIds(ids);
      if (providerChanged && draftEditable) {
        await patchPO.mutateAsync(buildPatchPOPayload(currentHeader, budgets.data ?? [], providerChanged, ids));
      } else {
        await patchPO.mutateAsync({ recipient_ids: ids });
      }
      toast('Contatti aggiornati');
    } catch {
      toast('Salvataggio contatti non riuscito', 'error');
    }
  }

  async function downloadPDF() {
    try {
      const blob = await downloads.pdf(detail.id);
      downloadBlob(blob, `rda-${detail.code ?? detail.id}.pdf`);
    } catch {
      toast('Download non riuscito', 'error');
    }
  }

  return (
    <main className="rdaPage">
      <POCommandBar
        po={detail}
        selectedMode={selectedActionMode}
        canSubmit={canSubmit}
        quoteRuleBlocked={quoteRuleBlocked}
        saving={patchPO.isPending || patchPayment.isPending}
        transitioning={transition.isPending}
        onModeChange={setSelectedActionMode}
        onClose={() => navigate('/rda')}
        onSubmit={() => setSubmitConfirm(true)}
        onTransition={(action) => void runTransition(action)}
        onPDF={() => void downloadPDF()}
      />
      <div className="detailLayout">
        <div className="detailMain">
          <PoHeaderSummary
            po={detail}
            value={currentHeader}
            budgets={budgets.data ?? []}
            providers={providerOptions}
            paymentMethods={paymentOptions}
            paymentRequiresVerification={paymentRequiresVerification}
            recipients={displayedRecipients}
            canEdit={headerCanEdit}
            editDisabledReason={headerEditDisabledReason}
            onEdit={openHeaderModal}
          />
          <PoTabs
            po={detailWithDisplayedRecipients ?? detail}
            provider={fullProvider}
            editable={draftEditable}
            header={currentHeader}
            badges={tabBadges}
            onRecipientSelectionChange={setRecipientDraftIds}
            onSaveRecipients={(ids) => void saveRecipients(ids)}
          />
        </div>
        <div className="detailSide">
          <POWorkflowRail stage={detail.action_model?.workflow_stage} />
          <POReadinessPanel items={readinessItems} />
          <CommentsPanel poId={detail.id} comments={comments.data ?? []} />
        </div>
      </div>
      <ConfirmDialog
        open={submitConfirm}
        title="Manda in approvazione"
        message="Confermi l'invio della richiesta in approvazione?"
        confirmLabel="Manda in approvazione"
        loading={patchPO.isPending || transition.isPending}
        onClose={() => setSubmitConfirm(false)}
        onConfirm={() => void submitPO()}
      />
      <PoHeaderEditModal
        open={headerModalOpen}
        po={detail}
        value={editHeader ?? currentHeader}
        budgets={budgets.data ?? []}
        providers={providerOptions}
        paymentMethods={editPaymentOptions}
        paymentRequiresVerification={editPaymentRequiresVerification}
        draftEditable={draftEditable}
        paymentEditable={paymentEditable}
        saving={patchPO.isPending || patchPayment.isPending}
        onChange={setEditHeader}
        onSave={(next) => void saveHeaderModal(next)}
        onClose={closeHeaderModal}
        onProviderChange={handleProviderChange}
        onRequestNewProvider={(search) => {
          if (!draftEditable) return;
          setProviderRequestSearch(search);
          setProviderRequestOpen(true);
        }}
      />
      {draftEditable ? (
        <ProviderRequestModal
          open={providerRequestOpen}
          initialCompanyName={providerRequestSearch}
          onClose={() => setProviderRequestOpen(false)}
          onCreated={handleProviderRequestCreated}
        />
      ) : null}
    </main>
  );
}
