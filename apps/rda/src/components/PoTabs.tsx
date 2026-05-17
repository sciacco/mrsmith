import { useEffect, useMemo, useState } from 'react';
import type { PoDetail, ProviderSummary } from '../api/types';
import type { TabBadgeModel } from '../lib/po-detail-view-model';
import { AttachmentsTab } from './AttachmentsTab';
import { ProviderRefTable } from './ProviderRefTable';
import { RowsTab } from './RowsTab';

type TabID = 'rows' | 'attachments' | 'contacts';

const tabs: { id: TabID; label: string }[] = [
  { id: 'rows', label: 'Righe PO' },
  { id: 'attachments', label: 'Allegati' },
  { id: 'contacts', label: 'Contatti' },
];

export function PoTabs({
  po,
  provider,
  rowsEditable,
  attachmentsEditable,
  contactsEditable,
  contactsSaving,
  badges,
  onRecipientSelectionChange,
  onSaveRecipients,
}: {
  po: PoDetail;
  provider?: ProviderSummary;
  rowsEditable: boolean;
  attachmentsEditable: boolean;
  contactsEditable: boolean;
  contactsSaving?: boolean;
  badges: TabBadgeModel;
  onRecipientSelectionChange?: (ids: number[]) => void;
  onSaveRecipients: (ids: number[]) => void;
}) {
  const [active, setActive] = useState<TabID>('rows');
  const visibleTabs = useMemo(
    () => (po.type === 'ECOMMERCE' ? tabs.filter((tab) => tab.id !== 'contacts') : tabs),
    [po.type],
  );

  useEffect(() => {
    if (visibleTabs.some((tab) => tab.id === active)) return;
    setActive(visibleTabs[0]?.id ?? 'rows');
  }, [active, visibleTabs]);

  return (
    <section className="surface">
      <div className="tabs">
        {visibleTabs.map((tab) => (
          <button key={tab.id} className={`tab ${active === tab.id ? 'active' : ''}`} type="button" onClick={() => setActive(tab.id)}>
            <span>{tab.label}</span>
            {tab.id === 'attachments' ? <span className="tabBadge">{badges.attachments}</span> : null}
            {tab.id === 'rows' ? <span className="tabBadge">{badges.rows}</span> : null}
            {tab.id === 'contacts' ? <span className="tabBadge">{badges.contacts}</span> : null}
          </button>
        ))}
      </div>
      <div className="tabBody">
        {active === 'rows' ? <RowsTab po={po} editable={rowsEditable} /> : null}
        {active === 'attachments' ? <AttachmentsTab po={po} editable={attachmentsEditable} /> : null}
        {active === 'contacts' && po.type !== 'ECOMMERCE' ? (
          <ProviderRefTable
            po={po}
            provider={provider}
            editable={contactsEditable}
            savingRecipients={contactsSaving}
            onSelectionChange={onRecipientSelectionChange}
            onSaveRecipients={onSaveRecipients}
          />
        ) : null}
      </div>
    </section>
  );
}
