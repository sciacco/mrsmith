import { useState, useMemo, useRef, useEffect } from 'react';
import { Skeleton, SearchInput, TableToolbar, useTableFilter } from '@mrsmith/ui';
import { isUpstreamAuthFailed } from '../../api/errors';
import { useDomainStatus } from '../../api/queries';
import { DomainStatusTable } from './DomainStatusTable';
import { ExportButtons } from '../../components/ExportButtons';
import styles from './DomainsPage.module.css';

const TABS = [
  { key: 'blocked', label: 'Bloccati' },
  { key: 'released', label: 'Rilasciati' },
] as const;

type TabKey = (typeof TABS)[number]['key'];

export function DomainsPage() {
  const [activeTab, setActiveTab] = useState<TabKey>('blocked');
  const [searchQuery, setSearchQuery] = useState('');
  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const [indicatorStyle, setIndicatorStyle] = useState({ left: 0, width: 0 });

  const { data: domains, isLoading, error } = useDomainStatus();

  useEffect(() => {
    const el = tabRefs.current[activeTab];
    if (el) setIndicatorStyle({ left: el.offsetLeft, width: el.offsetWidth });
  }, [activeTab]);

  const tabFiltered = useMemo(() => {
    if (!domains) return undefined;
    return domains.filter((d) =>
      activeTab === 'blocked' ? d.block_count > d.release_count : d.block_count <= d.release_count,
    );
  }, [domains, activeTab]);

  const { filtered: filteredDomains } = useTableFilter({
    data: tabFiltered,
    searchQuery,
    searchFields: ['domain'],
  });

  return (
    <div className={styles.page}>
      <div className={styles.toolbar}>
        <div>
          <h1 className={styles.pageTitle}>Stato domini</h1>
          <p className={styles.pageSubtitle}>Visualizza lo stato corrente di blocco/rilascio dei domini</p>
        </div>
      </div>

      <TableToolbar>
        <SearchInput value={searchQuery} onChange={setSearchQuery} placeholder="Cerca dominio..." />
        <ExportButtons basePath="/compliance/domains" params={{ status: activeTab, search: searchQuery }} />
      </TableToolbar>

      <div className={styles.tabBar}>
        {TABS.map((tab) => (
          <button
            key={tab.key}
            ref={(el) => { tabRefs.current[tab.key] = el; }}
            className={`${styles.tab} ${activeTab === tab.key ? styles.tabActive : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.label}
          </button>
        ))}
        <div className={styles.tabIndicator} style={indicatorStyle} />
      </div>

      <div className={styles.tableCard}>
        {isLoading ? (
          <div className={styles.tableBody}><Skeleton rows={8} /></div>
        ) : isUpstreamAuthFailed(error) ? (
          <div className={styles.emptyState}>
            <p className={styles.emptyTitle}>Servizio temporaneamente non disponibile</p>
            <p className={styles.emptyText}>Lo stato dei domini non puo essere caricato in questo momento.</p>
          </div>
        ) : (
          <DomainStatusTable domains={filteredDomains} />
        )}
      </div>
    </div>
  );
}
