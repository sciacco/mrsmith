import { useState } from 'react';
import { ApiError } from '@mrsmith/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Modal, useToast } from '@mrsmith/ui';
import { useApiClient } from '../../api/client';
import type { MACompanyFact, MACompanyFactKind, MACompanyRegistry } from '../../api/types';
import styles from './CompanyRegistrySection.module.css';

function errorLabel(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 400) return 'Richiesta non valida.';
    if (error.status === 403) return 'Non hai accesso a Binocolo.';
    if (error.status === 503) return 'Servizio non configurato in questo ambiente.';
    return `Richiesta non riuscita (${error.status}).`;
  }
  return 'Richiesta non riuscita.';
}

const FACT_KIND_LABEL: Record<MACompanyFactKind, string> = {
  non_vende: 'Non vende',
  in_trattativa_altrui: 'In trattativa con altri',
  da_evitare: 'Da evitare',
  gia_cliente: 'Già cliente',
  partner: 'Partner',
};

const FACT_KIND_WARN = new Set<MACompanyFactKind>(['non_vende', 'in_trattativa_altrui', 'da_evitare']);

function factBadgeClass(kind: MACompanyFactKind): string {
  return FACT_KIND_WARN.has(kind) ? styles.badgeWarn ?? '' : styles.badgeInfo ?? '';
}

function dateFmt(value: string): string {
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleDateString('it-IT', { day: '2-digit', month: 'short', year: 'numeric' });
}

function whoWhen(email: string | undefined, createdAt: string): string {
  const parts = [email, dateFmt(createdAt)].filter(Boolean);
  return parts.join(' · ');
}

// CompanyRegistrySection porta il "registro azienda" (fatti tipizzati +
// note) dal vecchio modale TargetPage / CompanyDossierPage in un componente
// condiviso. ATTENZIONE: la companyKey passata deve essere la companyKey
// normalizzata (vendor_id > vat_code > tax_code > company_name), NON la
// P.IVA — su /azienda il bug storico era passare vatCode come companyKey,
// per cui il registro risultava sempre vuoto.
export function CompanyRegistrySection({
  companyKey,
  vatCode,
  companyName,
  readOnly = false,
}: {
  companyKey: string;
  vatCode?: string;
  companyName?: string;
  readOnly?: boolean;
}) {
  const api = useApiClient();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [showHistory, setShowHistory] = useState(false);
  const [factModalOpen, setFactModalOpen] = useState(false);
  const [factKind, setFactKind] = useState<MACompanyFactKind>('non_vende');
  const [factNote, setFactNote] = useState('');
  const [revokeTarget, setRevokeTarget] = useState<MACompanyFact | null>(null);
  const [revokeNote, setRevokeNote] = useState('');

  const registryKey = ['ma-company-registry', companyKey];

  const registry = useQuery({
    queryKey: registryKey,
    queryFn: () => api.get<MACompanyRegistry>(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/registry`),
  });

  const createFact = useMutation({
    mutationFn: () =>
      api.post<MACompanyFact>(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/registry/facts`, {
        kind: factKind,
        note: factNote.trim(),
        vatCode,
        companyName,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: registryKey });
      setFactModalOpen(false);
      setFactNote('');
      setFactKind('non_vende');
    },
    onError: (err: unknown) => {
      toast(errorLabel(err), 'error');
    },
  });

  const revokeFact = useMutation({
    mutationFn: (id: string) =>
      api.post<void>(`/binocolo/v1/ma/companies/${encodeURIComponent(companyKey)}/registry/facts/${encodeURIComponent(id)}/revoke`, {
        note: revokeNote.trim(),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: registryKey });
      setRevokeTarget(null);
      setRevokeNote('');
    },
    onError: (err: unknown) => {
      toast(errorLabel(err), 'error');
    },
  });

  const facts = registry.data?.facts ?? [];
  const activeFacts = facts.filter((f) => !f.revokedAt);
  const revokedFacts = facts.filter((f) => f.revokedAt);

  return (
    <>
      <section className={styles.section}>
        <div className={styles.sectionHead}>
          <h2 id="registro" className={styles.sectionTitle}>
            Registro azienda
          </h2>
        </div>
        {activeFacts.length === 0 && !registry.isLoading ? (
          <p className={styles.hint}>Nessun fatto registrato.</p>
        ) : (
          <ul className={styles.factList}>
            {activeFacts.map((f) => (
              <li key={f.id} className={styles.factRow}>
                <span className={`${styles.factBadge} ${factBadgeClass(f.kind)}`}>{FACT_KIND_LABEL[f.kind]}</span>
                <span className={styles.factBody}>
                  {f.note}
                  <span className={styles.factWho}>{whoWhen(f.createdByEmail, f.createdAt)}</span>
                </span>
                {readOnly ? null : (
                  <button type="button" className={styles.linkBtn} onClick={() => setRevokeTarget(f)}>
                    Revoca
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}
        {revokedFacts.length > 0 ? (
          <div className={styles.historyToggle}>
            <button type="button" className={styles.linkBtn} onClick={() => setShowHistory((s) => !s)}>
              {showHistory ? 'Nascondi storico revocati' : `Storico revocati (${revokedFacts.length})`}
            </button>
            {showHistory ? (
              <ul className={styles.factList}>
                {revokedFacts.map((f) => (
                  <li key={f.id} className={`${styles.factRow} ${styles.factRevoked}`}>
                    <span className={`${styles.factBadge} ${factBadgeClass(f.kind)}`}>{FACT_KIND_LABEL[f.kind]}</span>
                    <span className={styles.factBody}>
                      {f.note}
                      <span className={styles.factWho}>{whoWhen(f.createdByEmail, f.createdAt)}</span>
                      <span className={styles.factWho}>
                        Revocato — {whoWhen(f.revokedByEmail, f.revokedAt ?? '')}
                        {f.revokeNote ? `: ${f.revokeNote}` : ''}
                      </span>
                    </span>
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        ) : null}
        {readOnly ? null : (
          <div className={styles.factActions}>
            <Button variant="secondary" size="sm" onClick={() => setFactModalOpen(true)}>
              + Registra fatto
            </Button>
          </div>
        )}
      </section>

      <Modal open={factModalOpen} onClose={() => setFactModalOpen(false)} title="Registra fatto">
        <div className={styles.factModalBody}>
          <label className={styles.formLabel}>
            Tipo
            <select
              className={styles.formSelect}
              value={factKind}
              onChange={(e) => setFactKind(e.target.value as MACompanyFactKind)}
            >
              {(Object.keys(FACT_KIND_LABEL) as MACompanyFactKind[]).map((k) => (
                <option key={k} value={k}>
                  {FACT_KIND_LABEL[k]}
                </option>
              ))}
            </select>
          </label>
          <label className={styles.formLabel}>
            Nota
            <textarea
              className={styles.formTextarea}
              value={factNote}
              onChange={(e) => setFactNote(e.target.value)}
              maxLength={500}
              rows={3}
            />
          </label>
          <div className={styles.modalActions}>
            <Button variant="secondary" onClick={() => setFactModalOpen(false)}>
              Annulla
            </Button>
            <Button loading={createFact.isPending} onClick={() => createFact.mutate()}>
              Registra
            </Button>
          </div>
        </div>
      </Modal>

      <Modal open={revokeTarget !== null} onClose={() => setRevokeTarget(null)} title="Revoca fatto">
        <div className={styles.factModalBody}>
          {revokeTarget ? (
            <p className={styles.hint}>
              {FACT_KIND_LABEL[revokeTarget.kind]} — {revokeTarget.note}
            </p>
          ) : null}
          <label className={styles.formLabel}>
            Motivo (opzionale)
            <textarea
              className={styles.formTextarea}
              value={revokeNote}
              onChange={(e) => setRevokeNote(e.target.value)}
              maxLength={500}
              rows={3}
            />
          </label>
          <div className={styles.modalActions}>
            <Button variant="secondary" onClick={() => setRevokeTarget(null)}>
              Annulla
            </Button>
            <Button loading={revokeFact.isPending} onClick={() => revokeTarget && revokeFact.mutate(revokeTarget.id)}>
              Revoca
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
}
