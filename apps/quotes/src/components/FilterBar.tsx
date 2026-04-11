import { useCallback, useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { SearchInput } from '@mrsmith/ui';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { useOwners } from '../api/queries';
import styles from './FilterBar.module.css';

const statusFilters = [
  { value: '', label: 'Tutte' },
  { value: 'DRAFT', label: 'Bozza' },
  { value: 'PENDING_APPROVAL', label: 'In approvazione' },
  { value: 'APPROVED', label: 'Approvate' },
] as const;

export function FilterBar() {
  const [params, setParams] = useSearchParams();
  const currentStatus = params.get('status') ?? '';
  const currentOwner = params.get('owner') ?? '';
  const currentSearch = params.get('q') ?? '';

  const { user } = useOptionalAuth();
  const { data: owners } = useOwners();

  const [searchValue, setSearchValue] = useState(currentSearch);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  // Debounced search
  const handleSearch = useCallback((value: string) => {
    setSearchValue(value);
    clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setParams(prev => {
        const next = new URLSearchParams(prev);
        if (value) {
          next.set('q', value);
        } else {
          next.delete('q');
        }
        next.set('page', '1');
        return next;
      });
    }, 300);
  }, [setParams]);

  useEffect(() => () => clearTimeout(debounceRef.current), []);

  const setFilter = useCallback((key: string, value: string) => {
    setParams(prev => {
      const next = new URLSearchParams(prev);
      if (value) {
        next.set(key, value);
      } else {
        next.delete(key);
      }
      next.set('page', '1');
      return next;
    });
  }, [setParams]);

  // "Le mie proposte" preset — match user email against owners
  const matchedOwner = owners?.find(o => o.email === user?.email);
  const isMyQuotesActive = currentOwner === matchedOwner?.id;

  const handleMyQuotes = () => {
    if (isMyQuotesActive) {
      setFilter('owner', '');
    } else if (matchedOwner) {
      setFilter('owner', matchedOwner.id);
    }
  };

  // "Recenti" preset — last 30 days
  const isRecentActive = params.get('date_from') !== null;
  const handleRecent = () => {
    setParams(prev => {
      const next = new URLSearchParams(prev);
      if (isRecentActive) {
        next.delete('date_from');
      } else {
        const d = new Date();
        d.setDate(d.getDate() - 30);
        next.set('date_from', d.toISOString().split('T')[0] ?? '');
      }
      next.set('page', '1');
      return next;
    });
  };

  return (
    <div className={styles.filterBar}>
      <div className={styles.topRow}>
        <div className={styles.pills}>
          {statusFilters.map(sf => (
            <button
              key={sf.value}
              className={`${styles.pill} ${currentStatus === sf.value ? styles.pillActive : ''}`}
              onClick={() => setFilter('status', sf.value)}
            >
              {sf.label}
            </button>
          ))}
        </div>

        <div className={styles.searchWrap}>
          <SearchInput
            value={searchValue}
            onChange={handleSearch}
            placeholder="Cerca proposte..."
          />
        </div>

        <div className={styles.presets}>
          <button
            className={`${styles.presetBtn} ${isMyQuotesActive ? styles.presetActive : ''}`}
            onClick={handleMyQuotes}
            disabled={!matchedOwner}
            title={!matchedOwner ? 'Utente non collegato a HubSpot' : undefined}
          >
            Le mie proposte
          </button>
          <button
            className={`${styles.presetBtn} ${isRecentActive ? styles.presetActive : ''}`}
            onClick={handleRecent}
          >
            Recenti
          </button>
        </div>
      </div>
    </div>
  );
}
