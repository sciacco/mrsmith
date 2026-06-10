import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { SearchInput, SingleSelect, Icon } from '@mrsmith/ui';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import { useOwners, useCustomers } from '../api/queries';
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
  const currentCustomer = params.get('customer_id') ?? '';
  const currentSearch = params.get('q') ?? '';
  const dateFrom = params.get('date_from') ?? '';
  const dateTo = params.get('date_to') ?? '';

  const { user } = useOptionalAuth();
  const { data: owners } = useOwners();
  const { data: customers } = useCustomers();

  const [searchValue, setSearchValue] = useState(currentSearch);
  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  // Sync search input with search param when search param changes externally (e.g. on clear)
  useEffect(() => {
    setSearchValue(currentSearch);
  }, [currentSearch]);

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

  const handleResetAdvanced = useCallback(() => {
    setParams(prev => {
      const next = new URLSearchParams(prev);
      next.delete('owner');
      next.delete('customer_id');
      next.delete('date_from');
      next.delete('date_to');
      next.set('page', '1');
      return next;
    });
  }, [setParams]);

  const customerOptions = useMemo(
    () =>
      (customers ?? []).map(c => ({
        value: String(c.id),
        label: c.name,
      })),
    [customers],
  );

  const ownerOptions = useMemo(
    () =>
      (owners ?? []).map(o => ({
        value: String(o.id),
        label: [o.firstname, o.lastname].filter(Boolean).join(' '),
      })),
    [owners],
  );

  const activeAdvancedCount = [currentOwner, currentCustomer, dateFrom, dateTo].filter(Boolean).length;
  const isAdvancedActive = activeAdvancedCount > 0;

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
          <button
            className={`${styles.presetBtn} ${isAdvancedActive ? styles.presetActive : ''} ${isAdvancedOpen ? styles.presetOpen : ''}`}
            onClick={() => setIsAdvancedOpen(prev => !prev)}
          >
            <Icon name="filter" size={14} />
            Filtri {activeAdvancedCount > 0 && `(${activeAdvancedCount})`}
            <Icon name={isAdvancedOpen ? 'chevron-up' : 'chevron-down'} size={14} />
          </button>
        </div>
      </div>

      {isAdvancedOpen && (
        <div className={styles.advancedPanel}>
          <div className={styles.grid}>
            <div className={styles.field}>
              <label className={styles.fieldLabel}>Cliente</label>
              <SingleSelect<string>
                options={customerOptions}
                selected={currentCustomer || null}
                onChange={v => setFilter('customer_id', v ?? '')}
                placeholder="Tutti i clienti"
                allowClear
              />
            </div>

            <div className={styles.field}>
              <label className={styles.fieldLabel}>Owner</label>
              <SingleSelect<string>
                options={ownerOptions}
                selected={currentOwner || null}
                onChange={v => setFilter('owner', v ?? '')}
                placeholder="Tutti gli owner"
                allowClear
              />
            </div>

            <div className={styles.field}>
              <label className={styles.fieldLabel}>Periodo proposta</label>
              <div className={styles.dateRange}>
                <div className={styles.dateInputWrap}>
                  <span className={styles.dateLabel}>Da</span>
                  <input
                    type="date"
                    className={styles.dateInput}
                    value={dateFrom}
                    onChange={e => setFilter('date_from', e.target.value)}
                  />
                </div>
                <div className={styles.dateInputWrap}>
                  <span className={styles.dateLabel}>A</span>
                  <input
                    type="date"
                    className={styles.dateInput}
                    value={dateTo}
                    onChange={e => setFilter('date_to', e.target.value)}
                  />
                </div>
              </div>
            </div>
          </div>

          {activeAdvancedCount > 0 && (
            <div className={styles.panelActions}>
              <button onClick={handleResetAdvanced} className={styles.resetLink}>
                Azzera filtri avanzati
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
