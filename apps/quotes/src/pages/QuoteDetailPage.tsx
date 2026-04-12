import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Button, Icon, TabNav, Tooltip, type TabNavDotIndicator } from '@mrsmith/ui';
import { useHSStatus, usePublishPrecheck, useQuote, useUpdateQuote } from '../api/queries';
import type { HSStatus, PublishPrecheck, Quote } from '../api/types';
import { StatusBadge } from '../components/StatusBadge';
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
  { key: 'contacts', label: 'Riferimenti' },
] as const;

type TabKey = (typeof tabs)[number]['key'];

const tabKeys: readonly TabKey[] = tabs.map(t => t.key);

function isTabKey(value: string): value is TabKey {
  return (tabKeys as readonly string[]).includes(value);
}

interface PublishBlocker {
  key: string;
  label: string;
}

function computePublishBlockers(
  isDirty: boolean,
  hsStatus: HSStatus | null | undefined,
  precheck: PublishPrecheck | null | undefined,
): PublishBlocker[] {
  const blockers: PublishBlocker[] = [];
  if (isDirty) {
    blockers.push({ key: 'dirty', label: 'Salva le modifiche prima di pubblicare' });
  }
  if (hsStatus?.sign_status === 'ESIGN_COMPLETED') {
    blockers.push({ key: 'esign', label: 'La proposta è già firmata su HubSpot' });
  }
  if (precheck?.has_missing_required_products) {
    const count = precheck.invalid_required_groups;
    blockers.push({
      key: 'required',
      label: count
        ? `${count} grupp${count === 1 ? 'o' : 'i'} prodotto obbligator${count === 1 ? 'io' : 'i'} da configurare`
        : 'Gruppi prodotto obbligatori da configurare',
    });
  }
  return blockers;
}

export function QuoteDetailPage() {
  const { id } = useParams<{ id: string }>();
  const quoteId = Number(id);
  const navigate = useNavigate();
  const { data: quote, isLoading } = useQuote(quoteId);
  const { data: hsStatus, refetch: refetchHSStatus } = useHSStatus(quoteId);
  const publishPrecheck = usePublishPrecheck(quoteId);
  const updateQuote = useUpdateQuote();
  const { isDirty, dirtyTabs, markDirty, markClean, setSnapshot } = useDirtyState();

  const [activeTab, setActiveTab] = useState<TabKey>('header');
  const [localQuote, setLocalQuote] = useState<Quote | null>(null);
  const [showPublish, setShowPublish] = useState(false);

  const dotIndicator = useMemo<Record<string, TabNavDotIndicator>>(
    () => ({
      header: dirtyTabs.header ? 'warning' : null,
      kits: dirtyTabs.kits ? 'warning' : null,
      notes: dirtyTabs.notes ? 'warning' : null,
      contacts: dirtyTabs.contacts ? 'warning' : null,
    }),
    [dirtyTabs],
  );

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
    } catch {
      // Error handled by mutation
    }
  }, [localQuote, quoteId, updateQuote, markClean, setSnapshot]);

  const handleOpenPublish = useCallback(async () => {
    await refetchHSStatus();
    setShowPublish(true);
  }, [refetchHSStatus]);

  const refreshHSStatus = useCallback(async () => {
    const result = await refetchHSStatus();
    return result.data ?? null;
  }, [refetchHSStatus]);

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

  const publishBlockers = useMemo(
    () => computePublishBlockers(isDirty, hsStatus, publishPrecheck.data),
    [isDirty, hsStatus, publishPrecheck.data],
  );
  const publishBlocked = publishBlockers.length > 0;
  const isRepublish = !!hsStatus?.hs_quote_id;
  const pdfAction = useMemo(() => {
    if (!hsStatus?.hs_quote_id) {
      return {
        enabled: false,
        href: null as string | null,
        message: 'Pubblica prima la proposta su HubSpot per generare il PDF.',
      };
    }
    if (!hsStatus.pdf_url) {
      return {
        enabled: false,
        href: null as string | null,
        message: 'HubSpot non ha ancora generato il PDF.',
      };
    }
    if (hsStatus.quote_url && hsStatus.pdf_url === hsStatus.quote_url) {
      return {
        enabled: false,
        href: null as string | null,
        message: 'Il link PDF non è ancora disponibile separatamente.',
      };
    }
    return {
      enabled: true,
      href: hsStatus.pdf_url,
      message: 'Scarica il PDF della proposta da HubSpot.',
    };
  }, [hsStatus]);
  const handleOpenPdf = useCallback(() => {
    if (!pdfAction.enabled || !pdfAction.href) return;
    window.open(pdfAction.href, '_blank', 'noopener,noreferrer');
  }, [pdfAction.enabled, pdfAction.href]);

  if (isLoading || !localQuote) {
    return <div className={styles.loading}>Caricamento...</div>;
  }

  return (
    <div className={styles.page}>
      {/* Sticky action bar */}
      <div className={styles.actionBar}>
        <div className={styles.actionBarInner}>
          <button
            type="button"
            className={styles.backBtn}
            onClick={() => navigate('/quotes')}
            aria-label="Torna all'elenco"
          >
            <Icon name="arrow-left" size={18} />
          </button>
          <span className={styles.quoteNumber}>{localQuote.quote_number}</span>
          <StatusBadge status={localQuote.status} />

          {isDirty && (
            <span className={styles.dirtyHint}>
              <Icon name="triangle-alert" size={14} />
              Hai modifiche non salvate
            </span>
          )}

          <div className={styles.actionBarSpacer} />

          {hsStatus?.hs_quote_id && hsStatus.quote_url && (
            <a
              className={styles.hsLink}
              href={hsStatus.quote_url}
              target="_blank"
              rel="noopener noreferrer"
            >
              <Icon name="external-link" size={14} />
              Apri su HS
            </a>
          )}
          <Tooltip content={<span>{pdfAction.message}</span>} placement="bottom">
            <span className={styles.hsLinkWrap}>
              <button
                type="button"
                className={`${styles.hsLinkButton} ${!pdfAction.enabled ? styles.hsLinkButtonDisabled : ''}`}
                onClick={handleOpenPdf}
                disabled={!pdfAction.enabled}
                aria-label="Scarica PDF"
              >
                <Icon name="download" size={14} />
                PDF
              </button>
            </span>
          </Tooltip>

          <div className={styles.saveWrap}>
            <Button
              variant="primary"
              onClick={() => void handleSave()}
              disabled={!isDirty}
              loading={updateQuote.isPending}
            >
              Salva
            </Button>
            {isDirty && !updateQuote.isPending && <span className={styles.attentionDot} aria-hidden="true" />}
          </div>

          <Tooltip
            content={
              publishBlocked ? (
                <div className={styles.publishBlockers}>
                  <div className={styles.publishBlockersTitle}>Impossibile pubblicare</div>
                  <ul className={styles.publishBlockersList}>
                    {publishBlockers.map(b => (
                      <li key={b.key}>{b.label}</li>
                    ))}
                  </ul>
                </div>
              ) : (
                <span>
                  {hsStatus?.hs_locked
                    ? "HubSpot riportera l'offerta in bozza prima di aggiornarla"
                    : 'Pubblica la proposta su HubSpot'}
                </span>
              )
            }
            placement="bottom"
          >
            <Button
              variant="secondary"
              onClick={() => void handleOpenPublish()}
              disabled={publishBlocked}
              leftIcon={<Icon name="external-link" size={16} />}
            >
              {isRepublish ? 'Ripubblica' : 'Pubblica'}
            </Button>
          </Tooltip>
        </div>
      </div>

      <div className={styles.contentWrap}>
        {/* Tabs */}
        <div className={styles.tabs}>
          <TabNav
            items={tabs.map(t => ({ key: t.key, label: t.label }))}
            activeKey={activeTab}
            onTabChange={(key) => { if (isTabKey(key)) setActiveTab(key); }}
            dotIndicator={dotIndicator}
          />
        </div>

        {/* Tab content */}
        <div className={styles.tabContent} key={activeTab}>
          {activeTab === 'header' && (
            <HeaderTab quote={localQuote} onChange={handleChange} />
          )}
          {activeTab === 'kits' && (
            <KitsTab
              quoteId={quoteId}
              documentType={localQuote.document_type}
              onDirtyChange={dirty => (dirty ? markDirty('kits') : markClean('kits'))}
            />
          )}
          {activeTab === 'notes' && (
            <NotesTab quote={localQuote} onChange={handleChange} />
          )}
          {activeTab === 'contacts' && (
            <ContactsTab quote={localQuote} onChange={handleChange} />
          )}
        </div>
      </div>

        <PublishModal
          open={showPublish}
          quoteId={quoteId}
          isRepublish={isRepublish}
          hsStatus={hsStatus ?? null}
          precheck={publishPrecheck.data ?? null}
          refreshHSStatus={refreshHSStatus}
          onClose={() => setShowPublish(false)}
        />
      </div>
  );
}
