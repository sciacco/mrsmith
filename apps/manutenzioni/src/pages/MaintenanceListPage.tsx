import { Button, Icon, SearchInput, Skeleton } from '@mrsmith/ui';
import { useDeferredValue, useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useMaintenances, useReferenceData } from '../api/queries';
import { Pagination } from '../components/Pagination';
import { StatusPill, statusTone } from '../components/StatusPill';
import {
  STATUS_OPTIONS,
  errorMessage,
  formatDateTime,
  minutesLabel,
  noticesSummary,
  statusLabel,
} from '../lib/format';
import shared from './shared.module.css';

export function MaintenanceListPage() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const reference = useReferenceData();

  const pageValue = Number(params.get('page') ?? '1');
  const page = Number.isFinite(pageValue) && pageValue > 0 ? pageValue : 1;
  const q = params.get('q') ?? '';
  const deferredQ = useDeferredValue(q);
  const selectedStatuses = useMemo(() => {
    const raw = params.get('status');
    return raw ? raw.split(',').filter(Boolean) : [];
  }, [params]);

  const query = useMaintenances({
    q: deferredQ || undefined,
    status: selectedStatuses,
    scheduled_from: params.get('scheduled_from') ?? undefined,
    scheduled_to: params.get('scheduled_to') ?? undefined,
    technical_domain_id: params.get('technical_domain_id') ?? undefined,
    maintenance_kind_id: params.get('maintenance_kind_id') ?? undefined,
    customer_scope_id: params.get('customer_scope_id') ?? undefined,
    site_id: params.get('site_id') ?? undefined,
    page,
    page_size: 20,
  });

  const hasFilters =
    q !== '' ||
    selectedStatuses.length > 0 ||
    Boolean(params.get('scheduled_from')) ||
    Boolean(params.get('scheduled_to')) ||
    Boolean(params.get('technical_domain_id')) ||
    Boolean(params.get('maintenance_kind_id')) ||
    Boolean(params.get('customer_scope_id')) ||
    Boolean(params.get('site_id'));

  function updateParam(key: string, value: string) {
    setParams((current) => {
      const next = new URLSearchParams(current);
      if (value) next.set(key, value);
      else next.delete(key);
      next.set('page', '1');
      return next;
    });
  }

  function updatePage(nextPage: number) {
    setParams((current) => {
      const next = new URLSearchParams(current);
      next.set('page', String(nextPage));
      return next;
    });
  }

  function clearFilters() {
    setParams({ page: '1' });
  }

  return (
    <section className={shared.page}>
      <div className={shared.header}>
        <div className={shared.titleBlock}>
          <h1 className={shared.pageTitle}>Manutenzioni</h1>
          <p className={shared.pageSubtitle}>
            Consulta le manutenzioni, apri il dettaglio operativo e aggiorna finestre, impatto e comunicazioni.
          </p>
        </div>
        <div className={shared.headerActions}>
          <Button
            variant="secondary"
            onClick={() => query.refetch()}
            loading={query.isFetching && !query.isLoading}
            leftIcon={<Icon name="loader" size={16} />}
          >
            Aggiorna
          </Button>
          <Button onClick={() => navigate('/manutenzioni/new')} leftIcon={<Icon name="plus" size={16} />}>
            Nuova manutenzione
          </Button>
        </div>
      </div>

      <div className={shared.filterBar}>
        <SearchInput
          value={q}
          onChange={(value) => updateParam('q', value)}
          placeholder="Cerca per codice, titolo, motivo, servizio o target..."
        />
        <select
          className={shared.select}
          value={selectedStatuses.join(',')}
          onChange={(event) => updateParam('status', event.target.value)}
          aria-label="Stato"
        >
          <option value="">Tutti gli stati</option>
          {STATUS_OPTIONS.map((status) => (
            <option key={status} value={status}>
              {statusLabel(status)}
            </option>
          ))}
        </select>
        <input
          className={shared.field}
          type="date"
          value={params.get('scheduled_from') ?? ''}
          onChange={(event) => updateParam('scheduled_from', event.target.value)}
          aria-label="Finestra dal"
        />
        <input
          className={shared.field}
          type="date"
          value={params.get('scheduled_to') ?? ''}
          onChange={(event) => updateParam('scheduled_to', event.target.value)}
          aria-label="Finestra al"
        />
        <select
          className={shared.select}
          value={params.get('technical_domain_id') ?? ''}
          onChange={(event) => updateParam('technical_domain_id', event.target.value)}
          aria-label="Dominio"
        >
          <option value="">Tutti i domini</option>
          {reference.data?.technical_domains.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name_it}
            </option>
          ))}
        </select>
        <select
          className={shared.select}
          value={params.get('maintenance_kind_id') ?? ''}
          onChange={(event) => updateParam('maintenance_kind_id', event.target.value)}
          aria-label="Tipo"
        >
          <option value="">Tutti i tipi</option>
          {reference.data?.maintenance_kinds.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name_it}
            </option>
          ))}
        </select>
        {hasFilters && (
          <Button variant="ghost" onClick={clearFilters}>
            Cancella filtri
          </Button>
        )}
      </div>

      {query.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={7} />
        </div>
      ) : query.error ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Registro non disponibile</h3>
          <p>{errorMessage(query.error, 'Impossibile caricare le manutenzioni.')}</p>
        </div>
      ) : !query.data || query.data.items.length === 0 ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIcon}>
            <Icon name="search" />
          </div>
          <h3>{hasFilters ? 'Nessuna manutenzione trovata' : 'Nessuna manutenzione'}</h3>
          <p>
            {hasFilters
              ? 'Non ci sono manutenzioni che corrispondono ai filtri selezionati.'
              : 'Crea una nuova manutenzione per iniziare il registro.'}
          </p>
        </div>
      ) : (
        <>
          <div className={shared.tableCard}>
            <div className={shared.tableScroll}>
              <table className={shared.table}>
                <thead>
                  <tr>
                    <th>Codice</th>
                    <th>Titolo</th>
                    <th>Stato</th>
                    <th>Dominio</th>
                    <th>Tipo</th>
                    <th>Ambito clienti</th>
                    <th>Sito</th>
                    <th>Finestra corrente</th>
                    <th>Downtime</th>
                    <th>Impatto / servizio</th>
                    <th>Comunicazioni</th>
                    <th className={shared.actionsCell}>Azioni</th>
                  </tr>
                </thead>
                <tbody>
                  {query.data.items.map((item) => (
                    <tr key={item.maintenance_id}>
                      <td className={shared.mono}>{item.code || `MNT #${item.maintenance_id}`}</td>
                      <td>
                        <div className={shared.rowTitle}>
                          <strong>{item.title_it}</strong>
                          {item.title_en && <span className={shared.small}>{item.title_en}</span>}
                        </div>
                      </td>
                      <td>
                        <StatusPill tone={statusTone(item.status)}>{statusLabel(item.status)}</StatusPill>
                      </td>
                      <td>{item.technical_domain.name_it}</td>
                      <td>{item.maintenance_kind.name_it}</td>
                      <td>{item.customer_scope.name_it}</td>
                      <td>{item.site?.name_it ?? '-'}</td>
                      <td>{item.current_window ? formatDateTime(item.current_window.scheduled_start_at) : '-'}</td>
                      <td>{minutesLabel(item.current_window?.expected_downtime_minutes)}</td>
                      <td>{item.primary_impact_label ?? '-'}</td>
                      <td>{noticesSummary(item.notice_statuses)}</td>
                      <td className={shared.actionsCell}>
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => navigate(`/manutenzioni/${item.maintenance_id}`)}
                        >
                          Apri
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
          <Pagination
            page={query.data.page}
            pageSize={query.data.page_size}
            total={query.data.total}
            onPageChange={updatePage}
          />
        </>
      )}
    </section>
  );
}
