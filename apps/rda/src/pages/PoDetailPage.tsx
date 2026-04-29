import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState } from 'react';
import { Navigate, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import {
  useBudgets,
  usePatchPaymentMethod,
  usePatchPO,
  usePaymentMethodDefault,
  usePaymentMethods,
  usePermissions,
  usePODetail,
  usePOComments,
  useProvider,
  useProviders,
  useRdaDownloads,
  useTransitionMutation,
  type TransitionAction,
} from '../api/queries';
import type { PoDetail, ProviderReference, ProviderSummary } from '../api/types';
import { ActionBar } from '../components/ActionBar';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { CommentsPanel } from '../components/CommentsPanel';
import { headerStateFromPO, PoHeaderForm, type HeaderFormState } from '../components/PoHeaderForm';
import { PoTabs } from '../components/PoTabs';
import { ProviderRequestModal } from '../components/ProviderRequestModal';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { coerceID, downloadBlob, isRequester, parseMistraMoney } from '../lib/format';
import { buildPatchPOPayload } from '../lib/po-payload';
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

  const po = usePODetail(poId);
  const comments = usePOComments(poId);
  const permissions = usePermissions();
  const budgets = useBudgets();
  const providers = useProviders();
  const methods = usePaymentMethods();
  const defaultPayment = usePaymentMethodDefault();
  const providerID = header?.provider_id || po.data?.provider?.id || null;
  const provider = useProvider(typeof providerID === 'number' ? providerID : null);
  const patchPO = usePatchPO(poId);
  const patchPayment = usePatchPaymentMethod(poId);
  const transition = useTransitionMutation();
  const downloads = useRdaDownloads();
  const providerOptions = useMemo(() => {
    const byID = new Map<number, ProviderSummary>();
    for (const providerItem of providers.data ?? []) byID.set(providerItem.id, providerItem);
    if (po.data?.provider) byID.set(po.data.provider.id, po.data.provider);
    if (provider.data) byID.set(provider.data.id, provider.data);
    for (const providerItem of requestedProviders) byID.set(providerItem.id, providerItem);
    return Array.from(byID.values());
  }, [po.data?.provider, provider.data, providers.data, requestedProviders]);
  const selectedProvider = providerOptions.find((providerItem) => providerItem.id === header?.provider_id);
  const fullProviderForDraft = provider.data ?? selectedProvider ?? po.data?.provider;
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

  if (poId == null) return <Navigate to="/rda" replace />;

  if (po.isLoading || !header) {
    return (
      <main className="rdaPage">
        <section className="stateCard"><Skeleton rows={10} /></section>
      </main>
    );
  }

  if (po.error || !po.data) {
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
  const dirty = JSON.stringify(currentHeader) !== JSON.stringify(initialHeader);
  const requester = isRequester(detail, user?.email);
  const draftEditable = detail.state === PO_STATES.DRAFT && requester;
  const paymentEditable = detail.state === PO_STATES.PENDING_APPROVAL_PAYMENT_METHOD && requester;
  const total = parseMistraMoney(detail.total_price);
  const quoteRuleBlocked = total >= 3000 && (detail.attachments?.length ?? 0) < 2;
  const canSubmit = draftEditable && (detail.rows?.length ?? 0) > 0 && !quoteRuleBlocked;
  const fullProvider = fullProviderForDraft ?? detail.provider;
  const providerDefault = paymentCodeFromProvider(fullProvider);
  const cdlanDefault = defaultPayment.data?.code ?? '';
  const paymentOptions = buildPaymentMethodOptions({
    methods: methods.data ?? [],
    providerDefault: fullProvider?.default_payment_method,
    cdlanDefaultCode: cdlanDefault,
    currentCode: currentHeader.payment_method,
  });
  const paymentRequiresVerification = requiresPaymentMethodVerification(currentHeader.payment_method, providerDefault, cdlanDefault);

  function handleProviderChange(value: number | '') {
    const nextProvider = providerOptions.find((item) => item.id === value);
    setRecipientDraftIds([]);
    setHeader((current) =>
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
    setRecipientDraftIds([]);
    setHeader((current) =>
      current
        ? {
            ...current,
            provider_id: providerCreated.id,
            payment_method: preferredPaymentMethodCode(providerCreated, cdlanDefault),
          }
        : current,
    );
  }

  async function saveHeader() {
    if (!draftEditable) return;
    try {
      await patchPO.mutateAsync(buildPatchPOPayload(currentHeader, budgets.data ?? [], providerChanged));
      toast('Bozza aggiornata');
    } catch {
      toast('Salvataggio non riuscito', 'error');
    }
  }

  async function savePaymentMethod() {
    if (!paymentEditable) return;
    try {
      await patchPayment.mutateAsync(currentHeader.payment_method);
      toast('Metodo pagamento aggiornato');
    } catch {
      toast('Salvataggio non riuscito', 'error');
    }
  }

  async function submitPO() {
    try {
      if (dirty) await patchPO.mutateAsync(buildPatchPOPayload(currentHeader, budgets.data ?? [], providerChanged));
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
      <ActionBar
        po={detail}
        permissions={permissions.data}
        currentEmail={user?.email}
        dirty={dirty}
        canSubmit={canSubmit}
        quoteRuleBlocked={quoteRuleBlocked}
        saving={patchPO.isPending || patchPayment.isPending}
        transitioning={transition.isPending}
        onClose={() => navigate('/rda')}
        onSave={() => void saveHeader()}
        onSubmit={() => setSubmitConfirm(true)}
        onTransition={(action) => void runTransition(action)}
        onPDF={() => void downloadPDF()}
      />
      <div className="detailLayout">
        <div className="detailMain">
          <PoHeaderForm
            po={detail}
            value={currentHeader}
            budgets={budgets.data ?? []}
            providers={providerOptions}
            paymentMethods={paymentOptions}
            paymentRequiresVerification={paymentRequiresVerification}
            recipients={displayedRecipients}
            draftEditable={draftEditable}
            paymentEditable={paymentEditable}
            onChange={setHeader}
            onProviderChange={handleProviderChange}
            onRequestNewProvider={(search) => {
              if (!draftEditable) return;
              setProviderRequestSearch(search);
              setProviderRequestOpen(true);
            }}
          />
          {paymentEditable ? (
            <section className="surface actionBar">
              <span className="muted">Aggiorna il metodo di pagamento richiesto.</span>
              <Button leftIcon={<Icon name="check" />} loading={patchPayment.isPending} onClick={() => void savePaymentMethod()}>
                Aggiorna metodo pagamento
              </Button>
            </section>
          ) : null}
          <PoTabs
            po={detailWithDisplayedRecipients ?? detail}
            provider={fullProvider}
            editable={draftEditable}
            header={currentHeader}
            saving={patchPO.isPending}
            onHeaderChange={setHeader}
            onRecipientSelectionChange={setRecipientDraftIds}
            onSaveHeader={() => void saveHeader()}
            onSaveRecipients={(ids) => void saveRecipients(ids)}
          />
        </div>
        <CommentsPanel poId={detail.id} comments={comments.data ?? []} />
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
