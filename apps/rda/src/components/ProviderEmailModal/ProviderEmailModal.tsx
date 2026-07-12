import { Button, Icon, Modal, Skeleton } from '@mrsmith/ui';
import { useEffect, useId, useMemo, useRef, useState, type FormEvent } from 'react';
import { useProviderMutations, useSendProviderEmail } from '../../api/queries';
import type {
  ProviderEmailContact,
  ProviderEmailLanguage,
  ProviderEmailPreparation,
  SendProviderEmailResponse,
  ProviderReference,
  SendProviderEmailPayload,
} from '../../api/types';
import { apiErrorMessage, mapProviderEmailError, type ProviderEmailMappedError, type ProviderEmailErrorSection } from '../../lib/api-error';
import { referenceTypeLabel } from '../../lib/provider-refs';
import { ProviderContactForm } from '../ProviderContactModal';
import styles from './ProviderEmailModal.module.css';

type DeliveryMode = 'standard' | 'personal';
type ContactRole = 'to' | 'cc';

type FieldErrors = Partial<Record<'to' | 'cc' | 'subject' | 'introduction' | 'conclusion' | 'documents', string>>;

export interface ProviderEmailModalProps {
  open: boolean;
  poId: number;
  preparation?: ProviderEmailPreparation | null;
  preparationGeneration?: number;
  preparationLoading?: boolean;
  preparationError?: unknown;
  onClose: () => void;
  onStandardSend?: () => Promise<void> | void;
  onCustomSend?: (payload: SendProviderEmailPayload) => Promise<SendProviderEmailResponse>;
  onStandardSent?: () => void;
  onSent?: (response: SendProviderEmailResponse, mode: 'personal') => void;
  onRefreshRequested?: () => void;
}

const languageOptions: Array<{ value: ProviderEmailLanguage; label: string }> = [
  { value: 'it', label: 'Italiano' },
  { value: 'en', label: 'Inglese' },
];

function contactLabel(contact: ProviderEmailContact): string {
  return [contact.first_name, contact.last_name].filter(Boolean).join(' ').trim() || contact.email;
}

function formatLastSent(value: string | null): string {
  if (!value) return 'Nessun invio precedente';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Data non disponibile';
  return new Intl.DateTimeFormat('it-IT', { dateStyle: 'short', timeStyle: 'short' }).format(date);
}

function formatOrderDate(value: string, language: ProviderEmailLanguage): string {
  if (!value) return '';
  const normalized = value.includes('T') ? value : value.replace(' ', 'T');
  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(language === 'it' ? 'it-IT' : 'en-GB', {
    day: '2-digit', month: language === 'it' ? '2-digit' : 'long', year: 'numeric',
  }).format(date);
}

function validate(payload: SendProviderEmailPayload): FieldErrors {
  const errors: FieldErrors = {};
  if (payload.to_contact_ids.length === 0) errors.to = 'Seleziona almeno un destinatario in A';
  if (!payload.subject) errors.subject = "Inserisci l'oggetto";
  else if (payload.subject.length > 200) errors.subject = "L'oggetto non può superare 200 caratteri";
  else if (/\r|\n/.test(payload.subject)) errors.subject = "L'oggetto deve essere su una sola riga";
  if (!payload.introduction) errors.introduction = 'Inserisci il testo introduttivo';
  if (!payload.conclusion) errors.conclusion = 'Inserisci il testo conclusivo';
  if (payload.introduction.length + payload.conclusion.length > 10_000) errors.conclusion = 'I testi non possono superare complessivamente 10.000 caratteri';
  return errors;
}

export function ProviderEmailModal({
  open,
  poId,
  preparation,
  preparationGeneration,
  preparationLoading = false,
  preparationError,
  onClose,
  onStandardSend,
  onCustomSend,
  onStandardSent,
  onSent,
  onRefreshRequested,
}: ProviderEmailModalProps) {
  const sendMutation = useSendProviderEmail();
  const { createReference } = useProviderMutations();
  const [initializedPreparationGeneration, setInitializedPreparationGeneration] = useState<number | null>(null);
  const preparationRefreshRequested = useRef(false);
  const [mode, setMode] = useState<DeliveryMode>('standard');
  const [language, setLanguage] = useState<ProviderEmailLanguage>('it');
  const [pendingLanguage, setPendingLanguage] = useState<ProviderEmailLanguage | null>(null);
  const [toIds, setToIds] = useState<number[]>([]);
  const [ccIds, setCcIds] = useState<number[]>([]);
  const [subject, setSubject] = useState('');
  const [introduction, setIntroduction] = useState('');
  const [conclusion, setConclusion] = useState('');
  const [documentIds, setDocumentIds] = useState<number[]>([]);
  const [subjectDirty, setSubjectDirty] = useState(false);
  const [introductionDirty, setIntroductionDirty] = useState(false);
  const [conclusionDirty, setConclusionDirty] = useState(false);
  const [recipientRole, setRecipientRole] = useState<ContactRole>('to');
  const [contacts, setContacts] = useState<ProviderEmailContact[]>([]);
  const [contactRole, setContactRole] = useState<ContactRole | null>(null);
  const [contactError, setContactError] = useState('');
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [sendError, setSendError] = useState<ProviderEmailMappedError | null>(null);
  const [standardPending, setStandardPending] = useState(false);
  const [personalPending, setPersonalPending] = useState(false);
  const toErrorId = useId();
  const ccErrorId = useId();
  const subjectErrorId = useId();
  const introductionErrorId = useId();
  const conclusionErrorId = useId();
  const documentsErrorId = useId();
  const globalErrorId = useId();
  const isPending = standardPending || personalPending || sendMutation.isPending || createReference.isPending;

  useEffect(() => {
    if (!open) {
      setInitializedPreparationGeneration(null);
      preparationRefreshRequested.current = false;
      return;
    }
    if (!preparation || preparationGeneration == null || initializedPreparationGeneration === preparationGeneration) return;
    setMode(preparation.state === 'CLOSED' ? 'personal' : 'standard');
    setLanguage(preparation.language);
    setPendingLanguage(null);
    setToIds(preparation.initial_to_ids);
    setCcIds(preparation.initial_cc_ids);
    setSubject(preparation.subject);
    setIntroduction(preparation.introduction);
    setConclusion(preparation.conclusion);
    setDocumentIds([]);
    setSubjectDirty(false);
    setIntroductionDirty(false);
    setConclusionDirty(false);
    setContacts(preparation.contacts);
    setContactRole(null);
    setContactError('');
    setFieldErrors({});
    setSendError(null);
    preparationRefreshRequested.current = false;
    setInitializedPreparationGeneration(preparationGeneration);
  }, [initializedPreparationGeneration, open, preparation, preparationGeneration]);

  const contactOptions = useMemo(() => contacts, [contacts]);
  const preparationMappedError = preparationError ? mapProviderEmailError(preparationError) : null;
  const currentRecipients = preparation?.initial_to_ids
    .map((id) => contacts.find((contact) => contact.id === id))
    .filter((contact): contact is ProviderEmailContact => Boolean(contact)) ?? [];

  useEffect(() => {
    if (!open || !preparationError) return;
    const mapped = mapProviderEmailError(preparationError);
    if (mapped.refreshDetail && !preparationRefreshRequested.current) {
      preparationRefreshRequested.current = true;
      onRefreshRequested?.();
    }
    if (mapped.closeModal) onClose();
  }, [onClose, onRefreshRequested, open, preparationError]);

  function clearSectionError(section: ProviderEmailErrorSection) {
    setFieldErrors((current) => ({ ...current, [section]: undefined }));
    setSendError((current) => current?.section === section ? null : current);
  }

  function applyLanguage(nextLanguage: ProviderEmailLanguage) {
    if (!preparation) return;
    const template = preparation.templates[nextLanguage];
    setLanguage(nextLanguage);
    setSubject(template.subject);
    setIntroduction(template.introduction);
    setConclusion(template.conclusion);
    setSubjectDirty(false);
    setIntroductionDirty(false);
    setConclusionDirty(false);
    setPendingLanguage(null);
    clearSectionError('subject');
    clearSectionError('introduction');
    clearSectionError('conclusion');
  }

  function requestLanguage(nextLanguage: ProviderEmailLanguage) {
    if (nextLanguage === language) return;
    if (subjectDirty || introductionDirty || conclusionDirty) setPendingLanguage(nextLanguage);
    else applyLanguage(nextLanguage);
  }

  function toggleContact(role: ContactRole, id: number, checked: boolean) {
    const setter = role === 'to' ? setToIds : setCcIds;
    const otherSetter = role === 'to' ? setCcIds : setToIds;
    setter((current) => checked ? [...new Set([...current, id])] : current.filter((item) => item !== id));
    if (checked) otherSetter((current) => current.filter((item) => item !== id));
    clearSectionError(role);
  }

  function toggleDocument(id: number, checked: boolean) {
    setDocumentIds((current) => checked ? [...new Set([...current, id])] : current.filter((item) => item !== id));
    clearSectionError('documents');
  }

  async function addContact(body: ProviderReference) {
    if (!preparation || !contactRole) return;
    setContactError('');
    try {
      const created = await createReference.mutateAsync({ providerId: preparation.provider.id, body });
      if (created.id == null) throw new Error('missing contact id');
      const contact: ProviderEmailContact = {
        id: created.id,
        first_name: created.first_name ?? body.first_name ?? '',
        last_name: created.last_name ?? body.last_name ?? '',
        email: created.email ?? body.email ?? '',
        reference_type: created.reference_type ?? body.reference_type ?? 'OTHER_REF',
      };
      setContacts((current) => [...current.filter((item) => item.id !== contact.id), contact]);
      toggleContact(contactRole, contact.id, true);
      setContactRole(null);
    } catch (error) {
      setContactError(apiErrorMessage(error, 'Salvataggio del contatto non riuscito. Controlla i dati e riprova.'));
    }
  }

  function handleMappedError(error: unknown) {
    const mapped = mapProviderEmailError(error);
    setSendError(mapped);
    if (mapped.section !== 'global') setFieldErrors((current) => ({ ...current, [mapped.section]: mapped.message }));
    if (mapped.refreshDetail) onRefreshRequested?.();
    if (mapped.closeModal) onClose();
  }

  async function sendStandard() {
    if (!onStandardSend) return;
    setSendError(null);
    setStandardPending(true);
    try {
      await onStandardSend();
      onStandardSent?.();
    } catch (error) {
      handleMappedError(error);
    } finally {
      setStandardPending(false);
    }
  }

  async function sendPersonal(event?: FormEvent) {
    event?.preventDefault();
    if (!preparation) return;
    const payload: SendProviderEmailPayload = {
      language,
      to_contact_ids: toIds,
      cc_contact_ids: ccIds,
      subject: subject.trim(),
      introduction: introduction.trim(),
      conclusion: conclusion.trim(),
      document_ids: documentIds,
    };
    const nextErrors = validate(payload);
    setFieldErrors(nextErrors);
    setSendError(null);
    if (Object.values(nextErrors).some(Boolean)) return;
    setPersonalPending(true);
    try {
      const response = onCustomSend
        ? await onCustomSend(payload)
        : await sendMutation.mutateAsync({ id: poId, body: payload });
      onSent?.(response, 'personal');
    } catch (error) {
      handleMappedError(error);
    } finally {
      setPersonalPending(false);
    }
  }

  function close() {
    if (!isPending) onClose();
  }

  const title = preparation?.state === 'CLOSED' ? 'Invia nuovamente email' : 'Invia PO al fornitore';
  const activeError = sendError ?? preparationMappedError;

  return (
    <Modal
      open={open}
      onClose={close}
      title={contactRole ? `Nuovo contatto · ${contactRole === 'to' ? 'A' : 'CC'}` : <span className={styles.modalTitle}><Icon name="mail" />{title}</span>}
      size="wide"
      dismissible={!isPending}
    >
      {preparationLoading || (!preparation && !preparationError) ? (
        <div className={styles.loading} aria-live="polite">
          <span>Preparazione email…</span>
          <Skeleton rows={5} />
        </div>
      ) : contactRole && preparation ? (
        <section className={styles.contactStep} aria-labelledby="provider-email-contact-title">
          <div className={styles.stepHeading}>
            <Button variant="ghost" leftIcon={<Icon name="arrow-left" />} disabled={createReference.isPending} onClick={() => setContactRole(null)}>
              Torna alla composizione
            </Button>
            <div>
              <h3 id="provider-email-contact-title">Aggiungi contatto</h3>
              <p>Il nuovo contatto sarà selezionato in {contactRole === 'to' ? 'A' : 'CC'}.</p>
            </div>
          </div>
          <ProviderContactForm
            saving={createReference.isPending}
            submitError={contactError}
            onCancel={() => setContactRole(null)}
            onSubmit={(body) => void addContact(body)}
          />
        </section>
      ) : preparation ? (
        <form className={styles.form} noValidate onSubmit={(event) => void sendPersonal(event)}>
          <section className={styles.summary} aria-label="Riepilogo invio">
            <div className={styles.summaryPrimary}>
              <div>
                <span className={styles.eyebrow}>Fornitore</span>
                <strong>{preparation.provider.company_name}</strong>
                <span>PO {preparation.po_code}</span>
              </div>
            </div>
            <dl className={styles.summaryFacts}>
              <div className={styles.currentRecipients}>
                <dt>Destinatari attuali</dt>
                <dd>{currentRecipients.length ? currentRecipients.map((contact) => <span key={contact.id}>{contactLabel(contact)} · {contact.email}</span>) : 'Nessuno'}</dd>
              </div>
              {preparation.accepted_count > 0 ? <>
                <div><dt>Email inviate</dt><dd>{preparation.accepted_count}</dd></div>
                <div><dt>Ultimo invio</dt><dd>{formatLastSent(preparation.last_accepted_at)}</dd></div>
              </> : null}
            </dl>
          </section>

          {preparation.state === 'PENDING_SEND' ? (
            <fieldset className={styles.modeChoice} disabled={isPending}>
              <legend>Modalità di invio</legend>
              <label className={mode === 'standard' ? styles.modeSelected : styles.modeOption}>
                <input type="radio" name="delivery-mode" checked={mode === 'standard'} onChange={() => setMode('standard')} />
                <span><strong>Invio standard</strong><small>Invia il PO con il testo di default.</small></span>
              </label>
              <label className={mode === 'personal' ? styles.modeSelected : styles.modeOption}>
                <input type="radio" name="delivery-mode" checked={mode === 'personal'} onChange={() => setMode('personal')} />
                <span><strong>Invio personalizzato</strong><small>Scegli destinatari, testo e documenti.</small></span>
              </label>
            </fieldset>
          ) : null}

          {mode === 'personal' ? (
            <div className={styles.composer}>
              <div className={styles.field}>
                <label htmlFor="provider-email-language">Lingua</label>
                <select id="provider-email-language" value={language} disabled={isPending} onChange={(event) => requestLanguage(event.target.value as ProviderEmailLanguage)}>
                  {languageOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                </select>
              </div>

              {pendingLanguage ? (
                <section className={styles.confirmation} role="alert" aria-labelledby="language-confirm-title">
                  <Icon name="triangle-alert" />
                  <div><strong id="language-confirm-title">Sostituire oggetto e messaggio?</strong><p>Il cambio lingua ripristina entrambi i testi.</p></div>
                  <div className={styles.confirmActions}>
                    <Button size="sm" variant="secondary" onClick={() => setPendingLanguage(null)}>Mantieni testo</Button>
                    <Button size="sm" onClick={() => applyLanguage(pendingLanguage)}>Cambia lingua</Button>
                  </div>
                </section>
              ) : null}

              <RecipientPicker
                contacts={contactOptions}
                toIds={toIds}
                ccIds={ccIds}
                activeRole={recipientRole}
                disabled={isPending}
                toError={fieldErrors.to}
                toErrorId={toErrorId}
                ccError={fieldErrors.cc}
                ccErrorId={ccErrorId}
                onRoleChange={setRecipientRole}
                onToggle={toggleContact}
                onAdd={(role) => { setContactError(''); setContactRole(role); }}
              />

              <div className={styles.field}>
                <label htmlFor="provider-email-subject">Oggetto <RequiredMarker /></label>
                <input
                  id="provider-email-subject"
                  value={subject}
                  required
                  maxLength={200}
                  disabled={isPending}
                  aria-invalid={fieldErrors.subject ? 'true' : undefined}
                  aria-describedby={fieldErrors.subject ? subjectErrorId : undefined}
                  onChange={(event) => { setSubject(event.target.value); setSubjectDirty(true); clearSectionError('subject'); }}
                />
                <span className={styles.counter}>{subject.length}/200</span>
                {fieldErrors.subject ? <p id={subjectErrorId} className={styles.fieldError}>{fieldErrors.subject}</p> : null}
              </div>

              <div className={styles.messageStructure}>
                <div className={styles.field}>
                  <label htmlFor="provider-email-introduction">Testo introduttivo <RequiredMarker /></label>
                  <textarea id="provider-email-introduction" value={introduction} required maxLength={10000} rows={5} disabled={isPending}
                    aria-invalid={fieldErrors.introduction ? 'true' : undefined} aria-describedby={fieldErrors.introduction ? introductionErrorId : undefined}
                    onChange={(event) => { setIntroduction(event.target.value); setIntroductionDirty(true); clearSectionError('introduction'); }} />
                  {fieldErrors.introduction ? <p id={introductionErrorId} className={styles.fieldError}>{fieldErrors.introduction}</p> : null}
                </div>
                <section className={styles.orderCardMarker} aria-label="Riepilogo ordine inserito nell'email">
                  <header><span><Icon name="file-check" /></span><strong>Riepilogo ordine</strong><Icon name="lock" /></header>
                  <dl>
                    <div><dt>{language === 'it' ? 'Numero ordine' : 'Order number'}</dt><dd>{preparation.order_summary.number}</dd></div>
                    {preparation.order_summary.date ? <div><dt>{language === 'it' ? 'Data ordine' : 'Order date'}</dt><dd>{formatOrderDate(preparation.order_summary.date, language)}</dd></div> : null}
                    {preparation.order_summary.requester ? <div><dt>{language === 'it' ? 'Referente' : 'Contact person'}</dt><dd>{preparation.order_summary.requester}</dd></div> : null}
                  </dl>
                </section>
                <div className={styles.field}>
                  <label htmlFor="provider-email-conclusion">Testo conclusivo <RequiredMarker /></label>
                  <textarea id="provider-email-conclusion" value={conclusion} required maxLength={10000} rows={4} disabled={isPending}
                    aria-invalid={fieldErrors.conclusion ? 'true' : undefined} aria-describedby={fieldErrors.conclusion ? conclusionErrorId : undefined}
                    onChange={(event) => { setConclusion(event.target.value); setConclusionDirty(true); clearSectionError('conclusion'); }} />
                  <span className={styles.counter}>{introduction.length + conclusion.length}/10.000</span>
                  {fieldErrors.conclusion ? <p id={conclusionErrorId} className={styles.fieldError}>{fieldErrors.conclusion}</p> : null}
                </div>
              </div>

              <fieldset className={styles.attachments} aria-invalid={fieldErrors.documents ? 'true' : undefined} aria-describedby={fieldErrors.documents ? documentsErrorId : undefined}>
                <legend>Allegati</legend>
                <div className={styles.requiredDocument}>
                  <span><Icon name="file-check" /></span>
                  <div><strong>{preparation.required_pdf.filename}</strong><small>PDF del PO · obbligatorio</small></div>
                  <Icon name="lock" />
                </div>
                {preparation.documents.length ? (
                  <div className={styles.documentList}>
                    {preparation.documents.map((document) => (
                      <label key={document.id} className={styles.documentOption}>
                        <input type="checkbox" checked={documentIds.includes(document.id)} disabled={isPending} onChange={(event) => toggleDocument(document.id, event.target.checked)} />
                        <span><strong>{document.filename}</strong><small>{document.attachment_type || 'Documento aggiuntivo'}</small></span>
                      </label>
                    ))}
                  </div>
                ) : <p className={styles.emptyDocuments}>Nessun documento aggiuntivo disponibile.</p>}
                {fieldErrors.documents ? <p id={documentsErrorId} className={styles.fieldError}>{fieldErrors.documents}</p> : null}
              </fieldset>
            </div>
          ) : null}

          {activeError && !activeError.closeModal ? (
            <section id={globalErrorId} className={styles.errorPanel} role="alert">
              <Icon name="triangle-alert" />
              <div><strong>{activeError.title}</strong><p>{activeError.message}</p></div>
            </section>
          ) : null}

          <div className={styles.liveStatus} aria-live="polite" aria-atomic="true">
            {isPending ? (mode === 'standard' ? 'Invio standard in corso…' : contactRole ? 'Salvataggio contatto…' : 'Invio email in corso…') : ''}
          </div>

          <footer className={styles.actions}>
            <Button variant="secondary" disabled={isPending} onClick={close}>Chiudi</Button>
            {mode === 'standard' ? (
              <Button leftIcon={<Icon name="mail" />} loading={standardPending} disabled={!onStandardSend || Boolean(preparationMappedError)} onClick={() => void sendStandard()}>
                Invia standard
              </Button>
            ) : (
              <Button type="submit" leftIcon={<Icon name="mail" />} loading={personalPending || sendMutation.isPending} disabled={Boolean(preparationMappedError)} aria-describedby={activeError ? globalErrorId : undefined}>
                {sendError?.code === 'PO_CLOSED_EMAIL_FAILED' ? 'Riprova' : 'Invia email'}
              </Button>
            )}
          </footer>
        </form>
      ) : preparationMappedError ? (
        <div className={styles.preparationError}>
          <section className={styles.errorPanel} role="alert">
            <Icon name="triangle-alert" />
            <div><strong>{preparationMappedError.title}</strong><p>{preparationMappedError.message}</p></div>
          </section>
          <div className={styles.actions}><Button variant="secondary" onClick={close}>Chiudi</Button></div>
        </div>
      ) : null}
    </Modal>
  );
}

function RequiredMarker() {
  return <><span className={styles.requiredMarker} aria-hidden="true" /><span className={styles.srOnly}> obbligatorio</span></>;
}

function RecipientPicker({
  contacts, toIds, ccIds, activeRole, disabled, toError, toErrorId, ccError, ccErrorId, onRoleChange, onToggle, onAdd,
}: {
  contacts: ProviderEmailContact[];
  toIds: number[];
  ccIds: number[];
  activeRole: ContactRole;
  disabled: boolean;
  toError?: string;
  toErrorId: string;
  ccError?: string;
  ccErrorId: string;
  onRoleChange: (role: ContactRole) => void;
  onToggle: (role: ContactRole, id: number, checked: boolean) => void;
  onAdd: (role: ContactRole) => void;
}) {
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const controlRef = useRef<HTMLDivElement>(null);
  const inputId = useId();
  const listId = useId();
  const selected = activeRole === 'to' ? toIds : ccIds;
  const other = activeRole === 'to' ? ccIds : toIds;
  const error = activeRole === 'to' ? toError : ccError;
  const errorId = activeRole === 'to' ? toErrorId : ccErrorId;
  const selectedContacts = contacts.filter((contact) => selected.includes(contact.id));
  const normalizedQuery = query.trim().toLocaleLowerCase('it');
  const results = contacts.filter((contact) => {
    const haystack = `${contactLabel(contact)} ${contact.email} ${referenceTypeLabel(contact.reference_type)}`.toLocaleLowerCase('it');
    return !selected.includes(contact.id) && (!normalizedQuery || haystack.includes(normalizedQuery));
  });

  useEffect(() => {
    if (!open) return;
    const closeOutside = (event: PointerEvent | FocusEvent) => {
      if (!controlRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('pointerdown', closeOutside);
    document.addEventListener('focusin', closeOutside);
    return () => {
      document.removeEventListener('pointerdown', closeOutside);
      document.removeEventListener('focusin', closeOutside);
    };
  }, [open]);

  function choose(contact: ProviderEmailContact) {
    onToggle(activeRole, contact.id, true);
    setQuery('');
    setOpen(false);
  }

  return (
    <fieldset className={styles.recipientPicker} aria-invalid={error ? 'true' : undefined} aria-describedby={error ? errorId : undefined}>
      <legend>Destinatari</legend>
      <div className={styles.recipientTabs} aria-label="Tipo destinatario">
        <button type="button" className={activeRole === 'to' ? styles.recipientTabActive : styles.recipientTab} onClick={() => { setOpen(false); onRoleChange('to'); }} disabled={disabled}>A <RequiredMarker /> <span>{toIds.length}</span></button>
        <button type="button" className={activeRole === 'cc' ? styles.recipientTabActive : styles.recipientTab} onClick={() => { setOpen(false); onRoleChange('cc'); }} disabled={disabled}>CC <span>{ccIds.length}</span></button>
      </div>
      <div className={styles.recipientControl} ref={controlRef}>
        <div className={styles.recipientChips}>
          {selectedContacts.map((contact) => (
            <span className={styles.recipientChip} key={contact.id} title={contact.email}>
              <span><strong>{contactLabel(contact)}</strong><small>{contact.email}</small></span>
              <button type="button" disabled={disabled} aria-label={`Rimuovi ${contactLabel(contact)} da ${activeRole === 'to' ? 'A' : 'CC'}`} onClick={() => onToggle(activeRole, contact.id, false)}><Icon name="x" /></button>
            </span>
          ))}
          <input id={inputId} value={query} disabled={disabled} role="combobox" aria-expanded={open} aria-controls={listId} aria-autocomplete="list"
            placeholder={selectedContacts.length ? 'Aggiungi destinatario…' : `Cerca contatto per ${activeRole === 'to' ? 'A' : 'CC'}…`}
            onFocus={() => setOpen(true)} onChange={(event) => { setQuery(event.target.value); setOpen(true); }}
            onKeyDown={(event) => {
              if (event.key === 'Escape' && open) {
                event.preventDefault();
                event.stopPropagation();
                setOpen(false);
              }
              if (event.key === 'Enter' && results[0]) { event.preventDefault(); choose(results[0]); }
            }} />
        </div>
        {open ? (
          <div className={styles.recipientMenu} id={listId} role="listbox">
            {results.length ? results.map((contact) => {
              const inOther = other.includes(contact.id);
              return <button type="button" role="option" aria-selected="false" key={contact.id} onMouseDown={(event) => event.preventDefault()} onClick={() => choose(contact)}>
                <span><strong>{contactLabel(contact)}</strong><small>{contact.email}</small></span>
                <span className={styles.contactType}>{referenceTypeLabel(contact.reference_type)}</span>
                {inOther ? <em>Sposta da {activeRole === 'to' ? 'CC' : 'A'}</em> : null}
              </button>;
            }) : <p>Nessun contatto trovato.</p>}
            <button type="button" className={styles.createContactAction} onMouseDown={(event) => event.preventDefault()} onClick={() => { setOpen(false); onAdd(activeRole); }}><Icon name="plus" /> Crea nuovo contatto</button>
          </div>
        ) : null}
      </div>
      <p className={styles.recipientHint}>{activeRole === 'to' ? "Destinatari principali dell'email." : 'Destinatari in copia.'}</p>
      {error ? <p id={errorId} className={styles.fieldError}>{error}</p> : null}
      <span className={styles.srOnly} aria-live="polite">{selectedContacts.length} destinatari selezionati in {activeRole === 'to' ? 'A' : 'CC'}</span>
    </fieldset>
  );
}
