import { useState } from 'react';
import { Button, Icon, Tooltip } from '@mrsmith/ui';
import type { MATarget } from '../../api/types';
import { ContactEditorModal } from './contacts/ContactEditorModal';
import styles from './CompanyPanels.module.css';

type ShareholderDetail = {
  name?: string;
  surname?: string;
  companyName?: string;
  percentShare?: number;
  taxCode?: string;
};

function EmptyState({ title, text }: { title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyStateIcon} aria-hidden="true">
        <Icon name="file-text" size={28} />
      </span>
      <h2>{title}</h2>
      <p>{text}</p>
    </div>
  );
}

export function extractShareholdersDetail(target?: MATarget): ShareholderDetail[] {
  const rawPayload = target?.vendorPayload;
  if (!rawPayload) return [];
  if (Array.isArray(rawPayload.shareHolders)) return rawPayload.shareHolders;
  if (Array.isArray(rawPayload.shareholders)) {
    const list: ShareholderDetail[] = [];
    for (const item of rawPayload.shareholders) {
      const percent = item.percentShare ?? 0;
      const info = item.shareholdersInformation;
      if (Array.isArray(info) && info.length > 0) {
        for (const sub of info) {
          list.push({
            name: sub.name,
            surname: sub.surname,
            companyName: sub.companyName,
            percentShare: sub.percentShare ?? percent,
            taxCode: sub.taxCode,
          });
        }
      } else {
        list.push({
          name: item.name,
          surname: item.surname,
          companyName: item.companyName,
          percentShare: percent,
          taxCode: item.taxCode,
        });
      }
    }
    return list;
  }
  return [];
}

export function hasShareholdersDetail(target?: MATarget): boolean {
  return extractShareholdersDetail(target).length > 0;
}

export function ShareholdersDetail({ target, companyKey }: { target: MATarget; companyKey: string }) {
  const shareholders = extractShareholdersDetail(target);
  const [contactName, setContactName] = useState<string | null>(null);

  if (shareholders.length === 0) {
    return <EmptyState title="Soci non disponibili" text="Nessun dato relativo ai soci presente per questo target." />;
  }

  return (
    <div className={styles.shGrid}>
      {shareholders.map((shareholder, index) => {
        const displayName = [shareholder.name, shareholder.surname].filter(Boolean).join(' ') || shareholder.companyName || 'Socio sconosciuto';
        const percent = shareholder.percentShare ?? 0;
        return (
          <div key={`${displayName}-${shareholder.taxCode ?? index}`} className={styles.shCard}>
            <div className={styles.shName}>{displayName}</div>
            {shareholder.taxCode ? <div className={styles.shTaxCode}>{shareholder.taxCode}</div> : null}
            <div className={styles.shShare}>
              <span>Quota societaria:</span>
              <strong>{percent > 0 ? `${percent.toLocaleString('it-IT')}%` : 'n.d.'}</strong>
            </div>
            <Tooltip content="Aggiungi ai contatti">
              <Button className={styles.shContactButton} size="sm" variant="ghost" aria-label={`Aggiungi ${displayName} ai contatti`} onClick={() => setContactName(displayName)}>
                <span aria-hidden="true">+</span><Icon name="user" size={16} />
              </Button>
            </Tooltip>
          </div>
        );
      })}
      {contactName ? <ContactEditorModal companyKey={companyKey} open onClose={() => setContactName(null)} initial={{ name: contactName, relationship: 'Socio' }} /> : null}
    </div>
  );
}
