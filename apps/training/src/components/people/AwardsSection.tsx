// Conseguimenti della scheda persona (#162, §Persone 1): certificazione,
// esito, data, scadenza con stato di validità corrente, fonte e attestato
// (scarica/carica/convalida). L'attestato ammette solo PDF (vincolo
// backend); il conseguimento nasce con employeeId precompilato.

import { type ChangeEvent, useRef, useState } from 'react';
import { Button, Icon, Modal, SingleSelect, StatusBadge, VisuallyHidden, useToast } from '@mrsmith/ui';
import {
  useCreateAward,
  useTrainingCertifications,
  useUpdateAward,
  useUploadAwardDocument,
  useValidateDocument,
} from '../../api/queries';
import { useApiClient } from '../../api/client';
import type { AwardInput, AwardUpdateInput, PersonAwardRef, PersonEnrollmentRef } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly } from '../events/eventFormat';
import { AWARD_OUTCOME_LABELS, AWARD_STATUS_LABELS, VALIDATION_SOURCE_LABELS } from '../../lib/labels';
import { awardStatusVariant, downloadFile } from './awardHelpers';
import cardStyles from '../requests/drawerShared.module.css';
import formStyles from '../requests/requestShared.module.css';
import listStyles from '../../pages/RequestsPage/listPage.module.css';
import styles from './AwardsSection.module.css';

const OUTCOME_OPTIONS = Object.entries(AWARD_OUTCOME_LABELS).map(([value, label]) => ({ value, label }));
const SOURCE_OPTIONS = Object.entries(VALIDATION_SOURCE_LABELS).map(([value, label]) => ({ value, label }));

interface AwardsSectionProps {
  personId: string;
  awards: PersonAwardRef[];
  enrollments: PersonEnrollmentRef[];
}

export function AwardsSection({ personId, awards, enrollments }: AwardsSectionProps) {
  const { toast } = useToast();
  const api = useApiClient();
  const validateDocument = useValidateDocument();
  const [editing, setEditing] = useState<PersonAwardRef | 'create' | null>(null);
  const [docError, setDocError] = useState<string | null>(null);
  const [pendingDownload, setPendingDownload] = useState<string | null>(null);

  async function download(documentId: string, filename: string) {
    setDocError(null);
    setPendingDownload(documentId);
    try {
      await downloadFile(api, `/training/v1/documents/${documentId}/download`, filename);
    } catch (e) {
      setDocError(describeApiError(e, "Scaricamento dell'attestato non riuscito"));
    } finally {
      setPendingDownload(null);
    }
  }

  async function validate(documentId: string) {
    setDocError(null);
    try {
      await validateDocument.mutateAsync(documentId);
      toast('Attestato convalidato');
    } catch (e) {
      setDocError(describeApiError(e, 'Convalida non riuscita'));
    }
  }

  return (
    <section className={cardStyles.card}>
      <header className={listStyles.header}>
        <h3 className={cardStyles.cardTitle}>Conseguimenti</h3>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={() => setEditing('create')}>
          Registra conseguimento
        </Button>
      </header>
      <ErrorPanel message={docError} onDismiss={() => setDocError(null)} />
      {awards.length === 0 ? (
        <p>Nessun conseguimento registrato.</p>
      ) : (
        <div className={listStyles.tableWrap}>
          <table className={listStyles.table}>
            <thead>
              <tr>
                <th>Certificazione</th>
                <th>Esito</th>
                <th>Conseguito il</th>
                <th>Scadenza</th>
                <th>Fonte</th>
                <th>Attestato</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {awards.map((a) => (
                <tr key={a.awardId}>
                  <td>{a.certificationName}</td>
                  <td>{AWARD_OUTCOME_LABELS[a.outcome] ?? a.outcome}</td>
                  <td>{formatDateOnly(a.awardedOn)}</td>
                  <td>
                    {a.expiresOn ? formatDateOnly(a.expiresOn) : '—'}{' '}
                    <StatusBadge
                      value={a.currentStatus}
                      label={AWARD_STATUS_LABELS[a.currentStatus] ?? a.currentStatus}
                      variant={awardStatusVariant(a.currentStatus)}
                    />
                  </td>
                  <td>{VALIDATION_SOURCE_LABELS[a.validationSource] ?? a.validationSource}</td>
                  <td>
                    <DocumentCell
                      awardId={a.awardId}
                      document={a.document}
                      pendingDownload={pendingDownload}
                      onDownload={download}
                      onValidate={validate}
                      validatePending={validateDocument.isPending}
                      onUploadError={setDocError}
                    />
                  </td>
                  <td>
                    <Button variant="ghost" size="sm" onClick={() => setEditing(a)}>
                      Correggi
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {editing && (
        <AwardEditorModal
          personId={personId}
          award={editing === 'create' ? null : editing}
          enrollments={enrollments}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  );
}

function DocumentCell({
  awardId,
  document,
  pendingDownload,
  onDownload,
  onValidate,
  validatePending,
  onUploadError,
}: {
  awardId: string;
  document: PersonAwardRef['document'];
  pendingDownload: string | null;
  onDownload: (documentId: string, filename: string) => void;
  onValidate: (documentId: string) => void;
  validatePending: boolean;
  onUploadError: (message: string) => void;
}) {
  const { toast } = useToast();
  const upload = useUploadAwardDocument();
  const inputRef = useRef<HTMLInputElement>(null);

  async function handleFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) return;
    try {
      await upload.mutateAsync({ awardId, file });
      toast('Attestato caricato');
    } catch (e) {
      onUploadError(describeApiError(e, "Caricamento dell'attestato non riuscito"));
    }
  }

  return (
    <div className={styles.docCell}>
      {document ? (
        <>
          <span className={styles.docName}>{document.filename}</span>
          <StatusBadge
            value={document.isValidated ? 'validated' : 'pending'}
            label={document.isValidated ? 'Validato' : 'Da validare'}
            variant={document.isValidated ? 'success' : 'warning'}
          />
          <Button
            variant="ghost"
            size="sm"
            leftIcon={<Icon name="download" size={14} />}
            loading={pendingDownload === document.id}
            onClick={() => onDownload(document.id, document.filename)}
          >
            Scarica
          </Button>
          {!document.isValidated && (
            <Button variant="ghost" size="sm" loading={validatePending} onClick={() => onValidate(document.id)}>
              Convalida
            </Button>
          )}
          <Button
            variant="ghost"
            size="sm"
            leftIcon={<Icon name="file-up" size={14} />}
            loading={upload.isPending}
            onClick={() => inputRef.current?.click()}
          >
            Sostituisci attestato
          </Button>
        </>
      ) : (
        <Button
          variant="ghost"
          size="sm"
          leftIcon={<Icon name="file-up" size={14} />}
          loading={upload.isPending}
          onClick={() => inputRef.current?.click()}
        >
          Carica attestato
        </Button>
      )}
      <input
        ref={inputRef}
        type="file"
        accept="application/pdf"
        className={styles.hiddenFileInput}
        onChange={handleFile}
      />
    </div>
  );
}

function AwardEditorModal({
  personId,
  award,
  enrollments,
  onClose,
}: {
  personId: string;
  award: PersonAwardRef | null;
  enrollments: PersonEnrollmentRef[];
  onClose: () => void;
}) {
  const { toast } = useToast();
  const certifications = useTrainingCertifications();
  const createAward = useCreateAward();
  const updateAward = useUpdateAward();

  const [certificationId, setCertificationId] = useState(award?.certificationId ?? '');
  const [enrollmentId, setEnrollmentId] = useState('');
  const [outcome, setOutcome] = useState(award?.outcome ?? 'passed_exam');
  const [awardedOn, setAwardedOn] = useState(award?.awardedOn ?? '');
  const [expiresOn, setExpiresOn] = useState(award?.expiresOn ?? '');
  const [validationSource, setValidationSource] = useState(award?.validationSource ?? '');
  const [externalCredentialId, setExternalCredentialId] = useState('');
  const [externalCredentialUrl, setExternalCredentialUrl] = useState('');
  const [notes, setNotes] = useState('');
  const [error, setError] = useState<string | null>(null);

  const pending = createAward.isPending || updateAward.isPending;
  const canSubmit = (award ? true : certificationId !== '') && outcome !== '' && awardedOn !== '';

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    try {
      if (award) {
        const input: AwardUpdateInput = {
          outcome,
          awardedOn,
          expiresOn: expiresOn || undefined,
          validationSource: validationSource || undefined,
          notes: notes.trim() || undefined,
        };
        await updateAward.mutateAsync({ id: award.awardId, input });
        toast('Conseguimento aggiornato');
      } else {
        const input: AwardInput = {
          employeeId: personId,
          certificationId,
          enrollmentId: enrollmentId || undefined,
          outcome,
          awardedOn,
          expiresOn: expiresOn || undefined,
          validationSource: validationSource || undefined,
          externalCredentialId: externalCredentialId.trim() || undefined,
          externalCredentialUrl: externalCredentialUrl.trim() || undefined,
          notes: notes.trim() || undefined,
        };
        await createAward.mutateAsync(input);
        toast('Conseguimento registrato');
      }
      onClose();
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <Modal open onClose={onClose} title={award ? 'Correggi conseguimento' : 'Registra conseguimento'} size="md">
      <div className={formStyles.body}>
        {award ? (
          <p className={formStyles.hint}>Certificazione: {award.certificationName}</p>
        ) : (
          <label className={formStyles.field}>
            <span className={formStyles.labelHead}>
              Certificazione
              <span className={formStyles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <SingleSelect
              options={(certifications.data ?? []).filter((c) => c.active).map((c) => ({ value: c.id, label: c.name }))}
              selected={certificationId || null}
              onChange={(v) => setCertificationId(v ?? '')}
              placeholder="Seleziona certificazione..."
              searchable
            />
          </label>
        )}

        <div className={formStyles.row}>
          <label className={formStyles.field}>
            Esito
            <SingleSelect options={OUTCOME_OPTIONS} selected={outcome} onChange={(v) => setOutcome(v ?? 'passed_exam')} />
          </label>
          <label className={formStyles.field}>
            Fonte
            <SingleSelect
              options={SOURCE_OPTIONS}
              selected={validationSource || null}
              onChange={(v) => setValidationSource(v ?? '')}
              placeholder="Documento verificato"
              allowClear
            />
          </label>
        </div>

        <div className={formStyles.row}>
          <label className={formStyles.field}>
            <span className={formStyles.labelHead}>
              Conseguito il
              <span className={formStyles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input type="date" className={formStyles.input} value={awardedOn} onChange={(e) => setAwardedOn(e.target.value)} />
          </label>
          <label className={formStyles.field}>
            Scadenza
            <input type="date" className={formStyles.input} value={expiresOn} onChange={(e) => setExpiresOn(e.target.value)} />
          </label>
        </div>

        {!award && enrollments.length > 0 && (
          <label className={formStyles.field}>
            Iscrizione d'origine
            <SingleSelect
              options={enrollments.map((e) => ({ value: e.enrollmentId, label: e.courseTitle }))}
              selected={enrollmentId || null}
              onChange={(v) => setEnrollmentId(v ?? '')}
              placeholder="Nessuna"
              allowClear
            />
          </label>
        )}

        {!award && (
          <div className={formStyles.row}>
            <label className={formStyles.field}>
              ID credenziale esterna
              <input
                className={formStyles.input}
                value={externalCredentialId}
                onChange={(e) => setExternalCredentialId(e.target.value)}
              />
            </label>
            <label className={formStyles.field}>
              URL credenziale esterna
              <input
                className={formStyles.input}
                value={externalCredentialUrl}
                onChange={(e) => setExternalCredentialUrl(e.target.value)}
              />
            </label>
          </div>
        )}

        <label className={formStyles.field}>
          Note
          <textarea className={formStyles.textarea} value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} />
          {award && (
            <span className={formStyles.hint}>Lasciando vuoto questo campo, l'eventuale nota esistente (non mostrata qui) resta invariata.</span>
          )}
        </label>

        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={pending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={pending} disabled={!canSubmit} onClick={submit}>
            {award ? 'Salva modifiche' : 'Registra'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
