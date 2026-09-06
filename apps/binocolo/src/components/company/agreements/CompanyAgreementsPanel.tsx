import { useState } from 'react';
import { formatLocalDate } from '@mrsmith/format';
import { Button, Icon, Skeleton, StatusBadge } from '@mrsmith/ui';
import type { MACompanyAgreement } from '../../../api/types';
import { useCompanyAgreements } from '../../../hooks/useCompanyAgreements';
import { AgreementEditorModal } from './AgreementEditorModal';
import { AgreementRemoveModal } from './AgreementRemoveModal';
import styles from './agreements.module.css';

const AGREEMENT_KIND_LABELS: Record<string, string> = { nda: 'NDA' };
const dateLabel = (value?: string | null) => formatLocalDate(value) ?? '';

function todayIso(): string {
  const now = new Date();
  const year = now.getFullYear();
  const month = String(now.getMonth() + 1).padStart(2, '0');
  const day = String(now.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function ExpiryStatus({ expiresOn }: { expiresOn?: string }) {
  if (!expiresOn) return <StatusBadge value="Senza scadenza" variant="neutral" dot={false} />;
  const expired = expiresOn < todayIso();
  if (expired) {
    return (
      <>
        <StatusBadge value="Scaduto" variant="warning" dot={false} />
        <span>Scaduto il {dateLabel(expiresOn)}</span>
      </>
    );
  }
  return (
    <>
      <StatusBadge value="In vigore" variant="success" dot={false} />
      <span>Scade il {dateLabel(expiresOn)}</span>
    </>
  );
}

export function CompanyAgreementsPanel({ companyKey }: { companyKey: string }) {
  const query = useCompanyAgreements(companyKey);
  const [editor, setEditor] = useState<MACompanyAgreement | 'new' | null>(null);
  const [removing, setRemoving] = useState<MACompanyAgreement | null>(null);
  const agreements = query.data ?? [];

  return (
    <section className={styles.panel}>
      <div className={styles.heading}>
        <div>
          <h3>Accordi sottoscritti</h3>
          <p>Condivisi tra le iniziative dell’azienda.</p>
        </div>
        <Button size="sm" variant="secondary" leftIcon={<Icon name="plus" size={16} />} onClick={() => setEditor('new')}>
          Registra accordo
        </Button>
      </div>
      {query.isLoading ? (
        <Skeleton rows={2} />
      ) : query.isError ? (
        <div className={styles.errorState} role="alert">
          <p>Accordi non disponibili.</p>
          <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>Riprova</Button>
        </div>
      ) : agreements.length === 0 ? (
        <p className={styles.empty}>Nessun accordo registrato.</p>
      ) : (
        <ul className={styles.list}>
          {agreements.map((agreement) => (
            <li className={styles.row} key={agreement.id}>
              <div className={styles.rowInfo}>
                <span className={styles.rowKind}>
                  <strong>{AGREEMENT_KIND_LABELS[agreement.kind] ?? agreement.kind}</strong>
                </span>
                <span className={styles.rowDates}>Sottoscritto il {dateLabel(agreement.signedOn)}</span>
                <span className={styles.rowStatus}><ExpiryStatus expiresOn={agreement.expiresOn} /></span>
              </div>
              <div className={styles.rowActions}>
                <Button size="sm" variant="ghost" onClick={() => setEditor(agreement)}>Modifica</Button>
                <Button size="sm" variant="ghost" onClick={() => setRemoving(agreement)}>Rimuovi</Button>
              </div>
            </li>
          ))}
        </ul>
      )}
      {editor ? (
        <AgreementEditorModal companyKey={companyKey} open onClose={() => setEditor(null)} agreement={editor === 'new' ? undefined : editor} />
      ) : null}
      <AgreementRemoveModal companyKey={companyKey} agreement={removing} onClose={() => setRemoving(null)} />
    </section>
  );
}
