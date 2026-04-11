import { useCallback, useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useHSStatus, usePublishPrecheck, useQuote, useUpdateQuote } from '../api/queries';
import type { Quote } from '../api/types';
import { StatusBadge } from '../components/StatusBadge';
import { DirtyBanner } from '../components/DirtyBanner';
import { HeaderTab } from '../components/HeaderTab';
import { ContactsTab } from '../components/ContactsTab';
import { NotesTab } from '../components/NotesTab';
import { KitsTab } from '../components/KitsTab';
import { PublishModal } from '../components/PublishModal';
import { useDirtyState } from '../hooks/useDirtyState';
import { buildQuoteSavePayload, prepareQuoteForDetail } from '../utils/quoteRules';
import styles from './QuoteDetailPage.module.css';

const tabs = [
  { key: 'header', label: 'Intestazione' },
  { key: 'kits', label: 'Kit e Prodotti' },
  { key: 'notes', label: 'Note e Condizioni' },
  { key: 'contacts', label: 'Contatti' },
] as const;

type TabKey = (typeof tabs)[number]['key'];

export function QuoteDetailPage() {
  const { id } = useParams<{ id: string }>();
  const quoteId = Number(id);
  const navigate = useNavigate();
  const { data: quote, isLoading } = useQuote(quoteId);
  const { data: hsStatus } = useHSStatus(quoteId);
  const publishPrecheck = usePublishPrecheck(quoteId);
  const updateQuote = useUpdateQuote();
  const { isDirty, dirtyTabs, markDirty, markClean, setSnapshot } = useDirtyState();

  const [activeTab, setActiveTab] = useState<TabKey>('header');
  const [localQuote, setLocalQuote] = useState<Quote | null>(null);
  const [saveFlash, setSaveFlash] = useState(false);
  const [showPublish, setShowPublish] = useState(false);

  // Sync server data to local state
  useEffect(() => {
    if (quote && !localQuote) {
      const prepared = prepareQuoteForDetail(quote);
      setLocalQuote(prepared);
      setSnapshot(prepared);
    }
  }, [quote, localQuote, setSnapshot]);

  const handleChange = useCallback((field: string, value: string | number) => {
    setLocalQuote(prev => {
      if (!prev) return prev;
      return { ...prev, [field]: value };
    });
    markDirty(field.startsWith('rif_') ? 'contacts' : 'header');
  }, [markDirty]);

  const handleSave = useCallback(async () => {
    if (!localQuote) return;
    try {
      const saved = await updateQuote.mutateAsync({ id: quoteId, data: buildQuoteSavePayload(localQuote) });
      const prepared = prepareQuoteForDetail(saved);
      setLocalQuote(prepared);
      markClean();
      setSnapshot(prepared);
      setSaveFlash(true);
      setTimeout(() => setSaveFlash(false), 600);
    } catch {
      // Error handled by mutation
    }
  }, [localQuote, quoteId, updateQuote, markClean, setSnapshot]);

  // Cmd+S keyboard shortcut
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        if (isDirty) void handleSave();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [isDirty, handleSave]);

  if (isLoading || !localQuote) {
    return <div className={styles.loading}>Caricamento...</div>;
  }

  return (
    <div className={styles.page}>
      {/* Header bar */}
      <div className={styles.headerBar}>
        <button className={styles.backBtn} onClick={() => navigate('/quotes')}>
          &#x2190;
        </button>
        <span className={styles.quoteNumber}>{localQuote.quote_number}</span>
        <StatusBadge status={localQuote.status} />

        <div className={styles.actions}>
          {hsStatus?.hs_quote_id && (
            <>
              {hsStatus.quote_url && (
                <a
                  className={styles.hsLink}
                  href={hsStatus.quote_url}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Apri su HS
                </a>
              )}
              {hsStatus.pdf_url && hsStatus.pdf_url !== hsStatus.quote_url && (
                <a
                  className={styles.hsLink}
                  href={hsStatus.pdf_url}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  PDF HS
                </a>
              )}
            </>
          )}
          <button
            className={styles.publishBtn}
            disabled={isDirty}
            title={isDirty ? 'Salva prima di pubblicare' : undefined}
            onClick={() => setShowPublish(true)}
          >
            {hsStatus?.hs_quote_id ? 'Ripubblica' : 'Pubblica su HubSpot'}
          </button>
          <button
            className={`${styles.saveBtn} ${saveFlash ? styles.saveSuccess : ''}`}
            disabled={!isDirty || updateQuote.isPending}
            onClick={() => void handleSave()}
          >
            {updateQuote.isPending ? 'Salvataggio...' : 'Salva'}
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className={styles.tabs}>
        {tabs.map(t => (
          <button
            key={t.key}
            className={`${styles.tab} ${activeTab === t.key ? styles.tabActive : ''}`}
            onClick={() => setActiveTab(t.key)}
          >
            {t.label}
            {dirtyTabs[t.key] && <span className={styles.tabDot} />}
          </button>
        ))}
      </div>

      {/* Dirty banner */}
      {isDirty && (
        <div className={styles.dirtyWrap}>
          <DirtyBanner />
        </div>
      )}

      {/* Tab content */}
      <div className={styles.tabContent} key={activeTab}>
        {activeTab === 'header' && (
          <HeaderTab quote={localQuote} onChange={handleChange} />
        )}
        {activeTab === 'kits' && (
          <KitsTab quoteId={quoteId} documentType={localQuote.document_type} />
        )}
        {activeTab === 'notes' && (
          <NotesTab quote={localQuote} onChange={handleChange} />
        )}
        {activeTab === 'contacts' && (
          <ContactsTab quote={localQuote} onChange={handleChange} />
        )}
      </div>

      {showPublish && (
        <PublishModal
          quoteId={quoteId}
          isRepublish={!!hsStatus?.hs_quote_id}
          hsStatus={hsStatus ?? null}
          precheck={publishPrecheck.data ?? null}
          onClose={() => setShowPublish(false)}
        />
      )}
    </div>
  );
}
