import { useState } from 'react';
import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import type { MACompanyContact } from '../../../api/types';
import { useCompanyContacts } from '../../../hooks/useCompanyContacts';
import { ContactEditorModal } from './ContactEditorModal';
import { ContactRemoveModal } from './ContactRemoveModal';
import styles from './contacts.module.css';

export function CompanyContactsPanel({ companyKey, compact = false }: { companyKey: string; compact?: boolean }) {
  const query = useCompanyContacts(companyKey);
  const { toast } = useToast();
  const [editor, setEditor] = useState<MACompanyContact | 'new' | null>(null);
  const [removing, setRemoving] = useState<MACompanyContact | null>(null);
  const [copyError, setCopyError] = useState('');
  const contacts = query.data ?? [];

  const copy = async (text: string) => {
    setCopyError('');
    try { await navigator.clipboard.writeText(text); toast('Recapito copiato.', 'success'); }
    catch { setCopyError('Copia non riuscita. Seleziona il testo e copialo manualmente.'); }
  };

  return (
    <section className={`${styles.panel} ${compact ? styles.compact : ''}`}>
      <div className={styles.heading}><div><h3>Interlocutori</h3><p>Rubrica condivisa dell’azienda</p></div><Button size="sm" variant="secondary" leftIcon={<Icon name="plus" size={16} />} onClick={() => setEditor('new')}>Aggiungi interlocutore</Button></div>
      {query.isLoading ? <Skeleton rows={2} /> : query.isError ? (
        <div className={styles.errorState} role="alert"><p>Rubrica non disponibile.</p><Button size="sm" variant="secondary" onClick={() => void query.refetch()}>Riprova</Button></div>
      ) : contacts.length === 0 ? (
        <p className={styles.empty}>Nessun interlocutore</p>
      ) : (
        <div className={styles.list}>{contacts.map((contact, index) => (
          <details className={styles.contact} key={contact.id} open={contact.isPrimary || (!contacts.some((item) => item.isPrimary) && index === 0 && !compact)}>
            <summary><span><strong>{contact.name}</strong>{contact.relationship ? <small>{contact.relationship}</small> : null}</span>{contact.contactDetails ? <span className={styles.contactPreview}>{contact.contactDetails.split('\n').filter((line) => line.trim()).join(' · ')}</span> : null}{contact.isPrimary ? <em>Principale</em> : null}<Icon name="chevron-down" size={16} /></summary>
            <div className={styles.contactBody}>
              {contact.contactDetails ? <div className={styles.details}><div className={styles.detailsHeading}><span>Recapiti</span><button type="button" onClick={() => void copy(contact.contactDetails)}>Copia tutti i recapiti</button></div>{contact.contactDetails.split('\n').filter((line) => line.trim().length > 0).map((line, lineIndex) => <div className={styles.detailLine} key={`${contact.id}-${lineIndex}`}><span>{line}</span><button type="button" aria-label={`Copia recapito: ${line}`} onClick={() => void copy(line)}><Icon name="copy" size={15} /></button></div>)}</div> : <p className={styles.muted}>Nessun recapito registrato.</p>}
              {contact.note ? <p className={styles.note}>{contact.note}</p> : null}
              <div className={styles.contactActions}><Button size="sm" variant="ghost" onClick={() => setEditor(contact)}>Modifica</Button><Button size="sm" variant="ghost" onClick={() => setRemoving(contact)}>Rimuovi</Button></div>
            </div>
          </details>
        ))}</div>
      )}
      {copyError ? <p className={styles.copyError} role="alert">{copyError}</p> : null}
      {editor ? <ContactEditorModal companyKey={companyKey} open onClose={() => setEditor(null)} contact={editor === 'new' ? undefined : editor} /> : null}
      <ContactRemoveModal companyKey={companyKey} contact={removing} onClose={() => setRemoving(null)} />
    </section>
  );
}
