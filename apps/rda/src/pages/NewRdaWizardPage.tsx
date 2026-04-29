import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState, type ChangeEvent } from 'react';
import { Navigate, useNavigate, useParams } from 'react-router-dom';
import {
  useBudgets,
  useCreatePO,
  useDeleteAttachment,
  usePaymentMethodDefault,
  usePaymentMethods,
  usePODetail,
  useProvider,
  useProviders,
  useRdaDownloads,
  useDeleteRow,
  useTransitionMutation,
  useUploadAttachment,
  usePatchPO,
} from '../api/queries';
import type { PoAttachment, PoDetail, PoRow, ProviderReference, ProviderSummary } from '../api/types';
import { BudgetSelect } from '../components/BudgetSelect';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { PaymentMethodSelect } from '../components/PaymentMethodSelect';
import { headerStateFromPO } from '../components/PoHeaderForm';
import { ProviderCombobox } from '../components/ProviderCombobox';
import { ProviderRequestModal } from '../components/ProviderRequestModal';
import { ProviderRefTable } from '../components/ProviderRefTable';
import { ReadinessChecklist, type ReadinessItem } from '../components/ReadinessChecklist';
import { RowModal } from '../components/RowModal';
import { RowTable } from '../components/RowTable';
import { WizardStepper } from '../components/WizardStepper';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import {
  coerceID,
  DEFAULT_RDA_CURRENCY,
  downloadBlob,
  formatDateIT,
  formatMoney,
  isRequester,
  normalizeCurrency,
  parseMistraMoney,
  RDA_CURRENCIES,
} from '../lib/format';
import {
  buildCreatePOPayload,
  buildPatchPOPayload,
  type POHeaderDraft,
  type POType,
} from '../lib/po-payload';
import {
  buildPaymentMethodOptions,
  paymentCodeFromProvider,
  preferredPaymentMethodCode,
  requiresPaymentMethodVerification,
} from '../lib/payment-options';
import { QUALIFICATION_REF } from '../lib/provider-refs';
import { PO_STATES } from '../lib/state-labels';
import { firstError, validateNewPO } from '../lib/validation';

const steps = ['Dati richiesta', 'Righe', 'Allegati', 'Invio fornitore', 'Approvazione'];

interface WizardHeaderState extends POHeaderDraft {
  type: POType;
}

const emptyHeader: WizardHeaderState = {
  type: 'STANDARD',
  budget_id: '',
  object: '',
  project: '',
  provider_id: '',
  payment_method: '',
  currency: DEFAULT_RDA_CURRENCY,
  provider_offer_code: '',
  provider_offer_date: '',
  description: '',
  note: '',
};

function recipientIDs(recipients?: ProviderReference[]): number[] {
  return (recipients ?? []).map((ref) => ref.id).filter((id): id is number => id != null);
}

function headerFromPO(po: PoDetail): WizardHeaderState {
  return {
    ...headerStateFromPO(po),
    type: po.type === 'ECOMMERCE' ? 'ECOMMERCE' : 'STANDARD',
    currency: normalizeCurrency(po.currency),
  };
}

function suggestedStep(po: PoDetail): number {
  const total = parseMistraMoney(po.total_price);
  if ((po.rows ?? []).length === 0) return 1;
  if (total >= 3000 && (po.attachments ?? []).length < 2) return 2;
  return 3;
}

function providerRefs(provider?: ProviderSummary): ProviderReference[] {
  if (!provider) return [];
  if (provider.refs?.length) return provider.refs;
  return provider.ref ? [provider.ref] : [];
}

function hasQualificationFallback(provider?: ProviderSummary): boolean {
  return providerRefs(provider).some((ref) => ref.reference_type === QUALIFICATION_REF && Boolean(ref.email));
}

function attachmentTypeLabel(value?: string): string {
  if (value === 'quote') return 'Preventivo';
  if (value === 'transport_document') return 'Documento di trasporto';
  return 'Altro';
}

function errorState(title: string, message: string) {
  return (
    <main className="rdaPage">
      <section className="stateCard">
        <p className="eyebrow">Nuova richiesta</p>
        <h1>{title}</h1>
        <p>{message}</p>
      </section>
    </main>
  );
}

export function NewRdaWizardPage() {
  const { poId: rawPoId } = useParams();
  const poId = rawPoId ? coerceID(rawPoId) : null;
  const navigate = useNavigate();
  const { user } = useOptionalAuth();
  const { toast } = useToast();
  const [step, setStep] = useState(0);
  const [header, setHeader] = useState<WizardHeaderState>(emptyHeader);
  const [attemptedHeader, setAttemptedHeader] = useState(false);
  const [contactDraftIds, setContactDraftIds] = useState<number[]>([]);
  const [deleteAttachment, setDeleteAttachment] = useState<PoAttachment | null>(null);
  const [submitConfirm, setSubmitConfirm] = useState(false);
  const [requestedProviders, setRequestedProviders] = useState<ProviderSummary[]>([]);
  const [providerRequestOpen, setProviderRequestOpen] = useState(false);
  const [providerRequestSearch, setProviderRequestSearch] = useState('');
  const [rowModalOpen, setRowModalOpen] = useState(false);
  const [editRowTarget, setEditRowTarget] = useState<PoRow | null>(null);
  const [deleteRowTarget, setDeleteRowTarget] = useState<PoRow | null>(null);

  const budgets = useBudgets();
  const providers = useProviders();
  const methods = usePaymentMethods();
  const defaultPayment = usePaymentMethodDefault();
  const po = usePODetail(poId);
  const createPO = useCreatePO();
  const patchPO = usePatchPO(poId);
  const transition = useTransitionMutation();
  const upload = useUploadAttachment();
  const removeAttachment = useDeleteAttachment();
  const removeRow = useDeleteRow();
  const downloads = useRdaDownloads();
  const providerOptions = useMemo(() => {
    const byID = new Map<number, ProviderSummary>();
    for (const providerItem of providers.data ?? []) byID.set(providerItem.id, providerItem);
    for (const providerItem of requestedProviders) byID.set(providerItem.id, providerItem);
    return Array.from(byID.values());
  }, [providers.data, requestedProviders]);
  const selectedProvider = providerOptions.find((providerItem) => providerItem.id === header.provider_id);
  const providerID = typeof header.provider_id === 'number' ? header.provider_id : null;
  const provider = useProvider(providerID);
  const fullProvider = provider.data ?? selectedProvider;

  useEffect(() => {
    if (!po.data) return;
    setHeader(headerFromPO(po.data));
    setContactDraftIds(recipientIDs(po.data.recipients));
    setStep((current) => (current === 0 ? suggestedStep(po.data) : current));
  }, [po.data]);

  useEffect(() => {
    if (poId != null || header.payment_method) return;
    const next = preferredPaymentMethodCode(fullProvider, defaultPayment.data?.code ?? '');
    if (next) setHeader((current) => ({ ...current, payment_method: next }));
  }, [defaultPayment.data?.code, fullProvider, header.payment_method, poId]);

  if (rawPoId && poId == null) return <Navigate to="/rda/new" replace />;

  const loading =
    budgets.isLoading ||
    providers.isLoading ||
    methods.isLoading ||
    defaultPayment.isLoading ||
    (poId != null && po.isLoading);
  const loadError = budgets.error ?? providers.error ?? methods.error ?? defaultPayment.error ?? po.error;

  const detail = po.data;
  const initialHeader = detail ? headerFromPO(detail) : emptyHeader;
  const providerChanged = detail ? header.provider_id !== initialHeader.provider_id : false;
  const headerDirty = detail ? JSON.stringify(header) !== JSON.stringify(initialHeader) : true;
  const requester = isRequester(detail, user?.email);
  const draftEditable = !detail || (detail.state === PO_STATES.DRAFT && requester);
  const providerDefault = paymentCodeFromProvider(fullProvider);
  const cdlanDefault = defaultPayment.data?.code ?? '';
  const paymentOptions = useMemo(
    () =>
      buildPaymentMethodOptions({
        methods: methods.data ?? [],
        providerDefault: fullProvider?.default_payment_method,
        cdlanDefaultCode: cdlanDefault,
        currentCode: header.payment_method,
      }),
    [cdlanDefault, fullProvider?.default_payment_method, header.payment_method, methods.data],
  );
  const paymentRequiresVerification = requiresPaymentMethodVerification(header.payment_method, providerDefault, cdlanDefault);
  const createPayload = useMemo(() => buildCreatePOPayload(header, budgets.data ?? []), [budgets.data, header]);
  const headerValidation = useMemo(() => validateNewPO(createPayload), [createPayload]);
  const headerReady = firstError(headerValidation) == null;
  const rows = detail?.rows ?? [];
  const attachments = detail?.attachments ?? [];
  const displayCurrency = normalizeCurrency(detail?.currency ?? header.currency);
  const total = parseMistraMoney(detail?.total_price);
  const quoteRuleBlocked = total >= 3000 && attachments.length < 2;
  const sendsToProvider = header.type !== 'ECOMMERCE';
  const contactsReady = !sendsToProvider || contactDraftIds.length > 0 || hasQualificationFallback(fullProvider);
  const readinessItems: ReadinessItem[] = [
    {
      id: 'header',
      label: 'Dati richiesta',
      ready: headerReady,
      detail: headerReady ? 'Budget, fornitore, pagamento, progetto e oggetto sono compilati.' : 'Completa i campi obbligatori.',
    },
    {
      id: 'rows',
      label: 'Righe',
      ready: rows.length > 0,
      detail: rows.length > 0 ? `${rows.length} rig${rows.length === 1 ? 'a inserita' : 'he inserite'}.` : 'Aggiungi almeno una riga.',
    },
    {
      id: 'attachments',
      label: 'Preventivi',
      ready: !quoteRuleBlocked,
      detail:
        total >= 3000
          ? `${attachments.length}/2 preventiv${attachments.length === 1 ? 'o caricato' : 'i caricati'} per ${formatMoney(total, displayCurrency)}.`
          : 'La soglia preventivi non richiede altri allegati.',
    },
    {
      id: 'contacts',
      label: 'Invio fornitore',
      ready: contactsReady,
      detail: sendsToProvider
        ? contactsReady
          ? 'Destinatari selezionati o referente qualifica disponibile.'
          : 'Seleziona un contatto o completa il referente qualifica.'
        : 'PO e-commerce: nessun invio automatico al fornitore.',
    },
  ];
  const readyToSubmit = draftEditable && readinessItems.every((item) => item.ready);

  function headerFieldError(key: string): string | undefined {
    if (!attemptedHeader) return undefined;
    return headerValidation.fieldErrors[key];
  }

  function updateHeader<K extends keyof WizardHeaderState>(key: K, value: WizardHeaderState[K]) {
    if (key === 'provider_id') {
      const nextProvider = providerOptions.find((item) => item.id === value);
      setContactDraftIds([]);
      setHeader((current) => ({
        ...current,
        [key]: value,
        payment_method: preferredPaymentMethodCode(nextProvider, defaultPayment.data?.code ?? ''),
      }));
      return;
    }
    if (key === 'type' && value === 'ECOMMERCE') {
      setContactDraftIds([]);
    }
    setHeader((current) => ({ ...current, [key]: value }));
  }

  function handleProviderRequestCreated(providerCreated: ProviderSummary) {
    setRequestedProviders((current) => {
      const withoutProvider = current.filter((item) => item.id !== providerCreated.id);
      return [...withoutProvider, providerCreated];
    });
    setContactDraftIds([]);
    setHeader((current) => ({
      ...current,
      provider_id: providerCreated.id,
      payment_method: preferredPaymentMethodCode(providerCreated, defaultPayment.data?.code ?? ''),
    }));
  }

  async function createDraft(): Promise<boolean> {
    setAttemptedHeader(true);
    if (!headerReady) {
      const message = firstError(headerValidation);
      if (message) toast(message, 'warning');
      return false;
    }
    try {
      const created = await createPO.mutateAsync(createPayload);
      const id = coerceID(created.id);
      if (id == null) {
        toast('Bozza creata, ma non e stato possibile riaprirla.', 'error');
        return false;
      }
      toast('Bozza creata');
      setStep(1);
      navigate(`/rda/new/${id}`, { replace: true });
      return true;
    } catch {
      toast('Creazione non riuscita', 'error');
      return false;
    }
  }

  async function saveHeader(): Promise<boolean> {
    if (poId == null) return createDraft();
    setAttemptedHeader(true);
    if (!headerReady) {
      const message = firstError(headerValidation);
      if (message) toast(message, 'warning');
      return false;
    }
    if (!headerDirty && !providerChanged) return true;
    try {
      await patchPO.mutateAsync(buildPatchPOPayload(header, budgets.data ?? [], providerChanged));
      toast('Dati richiesta salvati');
      return true;
    } catch {
      toast('Salvataggio non riuscito', 'error');
      return false;
    }
  }

  async function saveSupplierDelivery(): Promise<boolean> {
    if (poId == null) return false;
    const deliveryHeader = header.type === 'ECOMMERCE' ? { ...header, note: '' } : header;
    const recipientIds = header.type === 'ECOMMERCE' ? [] : contactDraftIds;
    try {
      await patchPO.mutateAsync(buildPatchPOPayload(deliveryHeader, budgets.data ?? [], false, recipientIds));
      if (header.type === 'ECOMMERCE') setContactDraftIds([]);
      toast('Invio al fornitore salvato');
      return true;
    } catch {
      toast('Salvataggio non riuscito', 'error');
      return false;
    }
  }

  async function nextStep() {
    if (step === 0) {
      if (await saveHeader()) setStep(1);
      return;
    }
    if (step === 3) {
      if (await saveSupplierDelivery()) setStep(4);
      return;
    }
    if (step === 4) {
      setSubmitConfirm(true);
      return;
    }
    setStep((current) => Math.min(current + 1, steps.length - 1));
  }

  async function selectStep(next: number) {
    if (next === step) return;
    if (step === 0 && next > 0) {
      if (await saveHeader()) setStep(next);
      return;
    }
    if (step === 3 && next > 3) {
      if (await saveSupplierDelivery()) setStep(next);
      return;
    }
    setStep(next);
  }

  async function uploadFiles(files: FileList | null) {
    if (!files || poId == null) return;
    try {
      for (const file of Array.from(files)) {
        await upload.mutateAsync({ id: poId, file });
      }
      toast('Allegati caricati');
    } catch {
      toast('Caricamento non riuscito', 'error');
    }
  }

  function handleUpload(event: ChangeEvent<HTMLInputElement>) {
    const input = event.currentTarget;
    void uploadFiles(input.files).finally(() => {
      input.value = '';
    });
  }

  async function download(attachment: PoAttachment) {
    if (poId == null) return;
    try {
      const blob = await downloads.attachment(poId, attachment.id);
      downloadBlob(blob, attachment.file_name || `allegato-${attachment.id}`);
    } catch {
      toast('Download non riuscito', 'error');
    }
  }

  function openNewRow() {
    setEditRowTarget(null);
    setRowModalOpen(true);
  }

  function openEditRow(row: PoRow) {
    setEditRowTarget(row);
    setRowModalOpen(true);
  }

  function closeRowModal() {
    setRowModalOpen(false);
    setEditRowTarget(null);
  }

  async function confirmDeleteRow() {
    if (!deleteRowTarget || poId == null) return;
    try {
      await removeRow.mutateAsync({ id: poId, rowId: deleteRowTarget.id });
      toast('Riga eliminata');
      setDeleteRowTarget(null);
    } catch {
      toast('Eliminazione non riuscita', 'error');
    }
  }

  async function confirmDeleteAttachment() {
    if (!deleteAttachment || poId == null) return;
    try {
      await removeAttachment.mutateAsync({ id: poId, attachmentId: deleteAttachment.id });
      toast('Allegato eliminato');
      setDeleteAttachment(null);
    } catch {
      toast('Eliminazione non riuscita', 'error');
    }
  }

  async function submitPO() {
    if (!detail || poId == null || !readyToSubmit) return;
    const deliveryHeader = header.type === 'ECOMMERCE' ? { ...header, note: '' } : header;
    const recipientIds = header.type === 'ECOMMERCE' ? [] : contactDraftIds;
    try {
      if (headerDirty || providerChanged) await patchPO.mutateAsync(buildPatchPOPayload(deliveryHeader, budgets.data ?? [], providerChanged, recipientIds));
      await transition.mutateAsync({ id: detail.id, action: 'submit' });
      toast('Richiesta mandata in approvazione');
      navigate('/rda');
    } catch {
      toast('Invio non riuscito', 'error');
    }
  }

  if (loading) {
    return (
      <main className="rdaPage">
        <section className="stateCard"><Skeleton rows={10} /></section>
      </main>
    );
  }

  if (loadError) {
    return errorState('Richiesta non disponibile', 'I dati necessari non possono essere caricati in questo momento.');
  }

  if (detail && !draftEditable) {
    return (
      <main className="rdaPage">
        <section className="stateCard">
          <p className="eyebrow">Nuova richiesta</p>
          <h1>Bozza non modificabile</h1>
          <p>La richiesta selezionata non puo essere completata da questa procedura.</p>
          <div className="actionRow">
            <Button variant="secondary" onClick={() => navigate('/rda')}>Torna a Le mie RDA</Button>
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className="rdaPage wizardPage">
      <header className="pageHeader wizardHeader">
        <div>
          <h1>Nuova richiesta</h1>
          <p>{detail ? `Bozza ${detail.code ?? detail.id}` : 'Completa i dati iniziali per creare la bozza.'}</p>
        </div>
        <Button variant="secondary" leftIcon={<Icon name="arrow-left" />} onClick={() => navigate('/rda')}>Le mie RDA</Button>
      </header>

      <WizardStepper steps={steps} current={step} maxAvailable={poId == null ? 0 : steps.length - 1} onStepClick={(next) => void selectStep(next)} />

      <section className="surface wizardSurface">
        {step === 0 ? (
          <div className="wizardPanel">
            <div className="surfaceHeader compactHeader">
              <div>
                <h2>Dati richiesta</h2>
              </div>
            </div>
            <div className="tabBody formGrid three">
              <div className="field">
                <label>Budget</label>
                <BudgetSelect budgets={budgets.data ?? []} value={header.budget_id} onChange={(next) => updateHeader('budget_id', next)} />
                {headerFieldError('budget_id') ? <p className="fieldError">{headerFieldError('budget_id')}</p> : null}
              </div>
              <div className="field">
                <label>Progetto</label>
                <input value={header.project} maxLength={50} onChange={(event) => updateHeader('project', event.target.value)} />
                {headerFieldError('project') ? <p className="fieldError">{headerFieldError('project')}</p> : null}
              </div>
              <div className="field">
                <label>Oggetto</label>
                <input value={header.object} onChange={(event) => updateHeader('object', event.target.value)} />
                {headerFieldError('object') ? <p className="fieldError">{headerFieldError('object')}</p> : null}
              </div>
              <div className="providerPaymentGrid">
                <div className="field providerPaymentProvider">
                  <label>Fornitore</label>
                  <ProviderCombobox
                    providers={providerOptions}
                    value={header.provider_id}
                    onChange={(next) => updateHeader('provider_id', next)}
                    onRequestNewProvider={(search) => {
                      setProviderRequestSearch(search);
                      setProviderRequestOpen(true);
                    }}
                  />
                  {headerFieldError('provider_id') ? <p className="fieldError">{headerFieldError('provider_id')}</p> : null}
                </div>
                <div className="field providerPaymentMethod">
                  <label>Modalità di pagamento</label>
                  <PaymentMethodSelect
                    methods={paymentOptions}
                    value={header.payment_method}
                    requiresVerification={paymentRequiresVerification}
                    onChange={(next) => updateHeader('payment_method', next)}
                  />
                  {paymentRequiresVerification ? <p className="fieldWarning">Richiede approvazione metodo pagamento</p> : null}
                  {headerFieldError('payment_method') ? <p className="fieldError">{headerFieldError('payment_method')}</p> : null}
                </div>
              </div>
              <div className="formGrid three fullWidth">
                <div className="field">
                  <label>Riferimento offerta del fornitore</label>
                  <input value={header.provider_offer_code} onChange={(event) => updateHeader('provider_offer_code', event.target.value)} />
                </div>
                <div className="field">
                  <label>Data offerta</label>
                  <input type="date" value={header.provider_offer_date} onChange={(event) => updateHeader('provider_offer_date', event.target.value)} />
                </div>
                <div className="field">
                  <label>Valuta</label>
                  <select value={header.currency} onChange={(event) => updateHeader('currency', normalizeCurrency(event.target.value))}>
                    {RDA_CURRENCIES.map((currency) => (
                      <option key={currency} value={currency}>
                        {currency}
                      </option>
                    ))}
                  </select>
                  {headerFieldError('currency') ? <p className="fieldError">{headerFieldError('currency')}</p> : null}
                </div>
              </div>
              <div className="field wide">
                <label>Descrizione ad uso interno</label>
                <textarea rows={4} value={header.description} onChange={(event) => updateHeader('description', event.target.value)} />
              </div>
              {attemptedHeader && headerValidation.formErrors.length ? <p className="fieldError fullWidth">{headerValidation.formErrors[0]}</p> : null}
            </div>
          </div>
        ) : null}

        {step === 1 && detail ? (
          <div className="wizardPanel">
            <div className="surfaceHeader compactHeader">
              <div>
                <h2>Righe</h2>
                <p className="muted">Aggiungi beni o servizi alla bozza.</p>
              </div>
              <Button size="sm" leftIcon={<Icon name="plus" />} disabled={!draftEditable} onClick={openNewRow}>
                Nuova riga
              </Button>
            </div>
            <div className="tabBody">
              <RowTable
                rows={rows}
                currency={displayCurrency}
                editable={draftEditable}
                emptyLabel="Nessuna riga inserita."
                onEdit={openEditRow}
                onDelete={setDeleteRowTarget}
              />
            </div>
          </div>
        ) : null}

        {step === 2 && detail ? (
          <div className="wizardPanel">
            <div className="surfaceHeader compactHeader">
              <div>
                <h2>Allegati</h2>
                <p className="muted">
                  {total >= 3000 ? `Carica almeno 2 preventivi per un totale PO di ${formatMoney(total, displayCurrency)}.` : 'Carica preventivi e documenti utili alla richiesta.'}
                </p>
              </div>
              <span className={`badge ${quoteRuleBlocked ? 'warning' : 'success'}`}>{attachments.length} allegat{attachments.length === 1 ? 'o' : 'i'}</span>
            </div>
            <div className="tabBody stack">
              <label className="uploadDrop">
                <Icon name="file-up" size={22} />
                <span>Carica preventivi</span>
                <small>Puoi selezionare piu file insieme.</small>
                <input type="file" multiple disabled={upload.isPending} onChange={handleUpload} />
              </label>
              <div className="tableScroll">
                <table className="dataTable">
                  <thead>
                    <tr><th>File</th><th>Tipo</th><th>Data</th><th className="actionsCell">Azioni</th></tr>
                  </thead>
                  <tbody>
                    {attachments.map((attachment) => (
                      <tr key={attachment.id}>
                        <td>{attachment.file_name ?? attachment.file_id ?? '-'}</td>
                        <td>{attachmentTypeLabel(attachment.attachment_type)}</td>
                        <td>{formatDateIT(attachment.created_at ?? attachment.created)}</td>
                        <td className="actionsCell">
                          <span className="iconActions">
                            <button className="iconButton" type="button" aria-label="Scarica allegato" title="Scarica" onClick={() => void download(attachment)}>
                              <Icon name="download" size={16} />
                            </button>
                            <button
                              className="iconButton dangerButton"
                              type="button"
                              aria-label="Elimina allegato"
                              title="Elimina"
                              onClick={() => setDeleteAttachment(attachment)}
                            >
                              <Icon name="trash" size={16} />
                            </button>
                          </span>
                        </td>
                      </tr>
                    ))}
                    {attachments.length === 0 ? <tr><td colSpan={4} className="emptyInline">Nessun allegato presente.</td></tr> : null}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        ) : null}

        {step === 3 && detail ? (
          <div className="wizardPanel">
            <div className="surfaceHeader compactHeader">
              <div>
                <h2>Invio al fornitore</h2>
                <p className="muted">Definisci se il PO deve essere trasmesso al fornitore e a quali destinatari.</p>
              </div>
            </div>
            <div className="tabBody stack">
              <fieldset className="poTypeChoice">
                <legend>Tipo PO</legend>
                <label className={`poTypeOption ${header.type === 'STANDARD' ? 'selected' : ''}`}>
                  <input
                    type="radio"
                    name="po_type"
                    value="STANDARD"
                    checked={header.type === 'STANDARD'}
                    onChange={() => updateHeader('type', 'STANDARD')}
                  />
                  <span className="poTypeIcon"><Icon name="mail" size={18} /></span>
                  <span className="poTypeCopy">
                    <strong>Standard</strong>
                    <small>Invia il PO al fornitore.</small>
                  </span>
                </label>
                <label className={`poTypeOption ${header.type === 'ECOMMERCE' ? 'selected' : ''}`}>
                  <input
                    type="radio"
                    name="po_type"
                    value="ECOMMERCE"
                    checked={header.type === 'ECOMMERCE'}
                    onChange={() => updateHeader('type', 'ECOMMERCE')}
                  />
                  <span className="poTypeIcon"><Icon name="package" size={18} /></span>
                  <span className="poTypeCopy">
                    <strong>E-commerce</strong>
                    <small>Nessun invio automatico al fornitore.</small>
                  </span>
                </label>
              </fieldset>

              {sendsToProvider ? (
                <>
                  <div className="field wide">
                    <label>Note da trasmettere al fornitore</label>
                    <textarea rows={4} value={header.note} onChange={(event) => updateHeader('note', event.target.value)} />
                  </div>
                  <ProviderRefTable
                    po={detail}
                    provider={fullProvider}
                    editable={draftEditable}
                    showSaveAction={false}
                    onSelectionChange={setContactDraftIds}
                  />
                </>
              ) : (
                <div className="deliveryEmptyState">
                  <span className="deliveryEmptyIcon"><Icon name="check-circle" size={20} /></span>
                  <div>
                    <strong>Invio fornitore non previsto</strong>
                    <p>Per i PO e-commerce il wizard non richiede destinatari e salva l&apos;ordine senza contatti di invio.</p>
                  </div>
                </div>
              )}
            </div>
          </div>
        ) : null}

        {step === 4 && detail ? (
          <div className="wizardPanel">
            <div className="surfaceHeader compactHeader">
              <div>
                <h2>Approvazione</h2>
                <p className="muted">Controlla la richiesta e mandala in approvazione.</p>
              </div>
              <span className={`badge ${readyToSubmit ? 'success' : 'warning'}`}>{readyToSubmit ? 'Pronta per invio' : 'Completa dati'}</span>
            </div>
            <div className="tabBody submitGrid">
              <ReadinessChecklist items={readinessItems} />
              <div className="submitSummary">
                <div>
                  <span>Oggetto</span>
                  <strong>{header.object || '-'}</strong>
                </div>
                <div>
                  <span>Fornitore</span>
                  <strong>{fullProvider?.company_name ?? '-'}</strong>
                </div>
                <div>
                  <span>Tipo PO</span>
                  <strong>{sendsToProvider ? 'Standard, invio al fornitore' : 'E-commerce, nessun invio fornitore'}</strong>
                </div>
                <div>
                  <span>Totale PO</span>
                  <strong>{formatMoney(detail.total_price, displayCurrency)}</strong>
                </div>
                <div>
                  <span>Righe</span>
                  <strong>{rows.length}</strong>
                </div>
              </div>
            </div>
          </div>
        ) : null}
      </section>

      <div className="wizardFooter">
        <Button variant="ghost" disabled={step === 0 || createPO.isPending || patchPO.isPending || transition.isPending} onClick={() => setStep((current) => Math.max(0, current - 1))}>
          Indietro
        </Button>
        <div className="wizardFooterSummary" aria-live="polite">
          <span className="wizardFooterStep">Passo {step + 1} di {steps.length}</span>
          <span className="wizardFooterMetric total">
            <span>Totale PO</span>
            <strong>{formatMoney(total, displayCurrency)}</strong>
          </span>
          {!detail ? <span className="wizardFooterDraft">Bozza non ancora creata</span> : null}
        </div>
        <Button
          leftIcon={step === steps.length - 1 ? <Icon name="check" /> : <Icon name="arrow-right" />}
          disabled={(step === steps.length - 1 && !readyToSubmit) || createPO.isPending || patchPO.isPending || transition.isPending}
          loading={createPO.isPending || patchPO.isPending || transition.isPending}
          onClick={() => void nextStep()}
        >
          {step === 0 && poId == null ? 'Crea bozza' : step === steps.length - 1 ? 'Manda in approvazione' : 'Continua'}
        </Button>
      </div>

      {detail ? <RowModal poId={detail.id} currency={displayCurrency} open={rowModalOpen} row={editRowTarget} onClose={closeRowModal} /> : null}

      <ConfirmDialog
        open={deleteRowTarget != null}
        title="Elimina riga"
        message="Confermi eliminazione della riga selezionata?"
        confirmLabel="Elimina"
        danger
        loading={removeRow.isPending}
        onClose={() => setDeleteRowTarget(null)}
        onConfirm={() => void confirmDeleteRow()}
      />

      <ConfirmDialog
        open={deleteAttachment != null}
        title="Elimina allegato"
        message="Confermi eliminazione dell'allegato selezionato?"
        confirmLabel="Elimina"
        danger
        loading={removeAttachment.isPending}
        onClose={() => setDeleteAttachment(null)}
        onConfirm={() => void confirmDeleteAttachment()}
      />
      <ProviderRequestModal
        open={providerRequestOpen}
        initialCompanyName={providerRequestSearch}
        onClose={() => setProviderRequestOpen(false)}
        onCreated={handleProviderRequestCreated}
      />
      <ConfirmDialog
        open={submitConfirm}
        title="Manda in approvazione"
        message="Confermi l'invio della richiesta in approvazione?"
        confirmLabel="Manda in approvazione"
        loading={patchPO.isPending || transition.isPending}
        onClose={() => setSubmitConfirm(false)}
        onConfirm={() => void submitPO()}
      />
    </main>
  );
}
