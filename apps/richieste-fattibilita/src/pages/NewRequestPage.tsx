import { Button, Icon, MultiSelect, SearchInput, Skeleton, useToast } from '@mrsmith/ui';
import { useDeferredValue, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useCreateRichiesta, useDeals, useFornitori } from '../api/queries';
import { copyErrorMessage } from '../lib/format';
import styles from './shared.module.css';

export function NewRequestPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const [search, setSearch] = useState('');
  const [cliente, setCliente] = useState('');
  const [page, setPage] = useState(1);
  const [selectedDeal, setSelectedDeal] = useState<{
    id: number;
    codice: string;
    deal_name: string;
    company_name: string | null;
    owner_email: string | null;
  } | null>(null);
  const [indirizzo, setIndirizzo] = useState('');
  const [descrizione, setDescrizione] = useState('');
  const [fornitoriPreferiti, setFornitoriPreferiti] = useState<number[]>([]);

  const deferredSearch = useDeferredValue(search);
  const deferredCliente = useDeferredValue(cliente);
  const deals = useDeals({ q: deferredSearch || undefined, cliente: deferredCliente || undefined, page, page_size: 8 });
  const fornitori = useFornitori();
  const createRichiesta = useCreateRichiesta();

  const canSubmit = selectedDeal !== null && indirizzo.trim() !== '' && descrizione.trim() !== '';

  async function handleSubmit() {
    if (!selectedDeal) return;
    createRichiesta.mutate(
      {
        deal_id: selectedDeal.id,
        indirizzo: indirizzo.trim(),
        descrizione: descrizione.trim(),
        fornitori_preferiti: fornitoriPreferiti,
      },
      {
        onSuccess: () => {
          toast('Richiesta inserita');
          navigate('/richieste');
        },
        onError: (error) => {
          toast(copyErrorMessage(error, 'Impossibile creare la richiesta.'), 'error');
        },
      },
    );
  }

  function resetForm() {
    setSelectedDeal(null);
    setIndirizzo('');
    setDescrizione('');
    setFornitoriPreferiti([]);
  }

  return (
    <section className={styles.page}>
      <div className={styles.pageHeader}>
        <div>
          <h1 className={styles.pageTitle}>Nuova RDF</h1>
          <p className={styles.pageSubtitle}>
            Cerca il deal corretto, seleziona i fornitori preferiti e inserisci il contesto operativo della richiesta.
          </p>
        </div>
      </div>

      <div className={styles.newGrid}>
        <div className={styles.panel}>
          <div className={styles.panelHeader}>
            <div>
              <h2 className={styles.panelTitle}>Selezione deal</h2>
              <p className={styles.small}>Cerca per codice, nome deal, cliente o owner.</p>
            </div>
          </div>

          <div className={styles.filterStack}>
            <SearchInput value={search} onChange={(value) => { setSearch(value); setPage(1); }} placeholder="Cerca deal..." />
            <input
              className={styles.inlineInput}
              value={cliente}
              onChange={(event) => { setCliente(event.target.value); setPage(1); }}
              placeholder="Filtra per cliente"
              aria-label="Filtra per cliente"
            />
          </div>

          <div className={styles.dealList}>
            {deals.isLoading ? (
              <Skeleton rows={6} />
            ) : deals.error ? (
              <div className={styles.emptyCard}>
                <div className={styles.emptyIconDanger}><Icon name="triangle-alert" /></div>
                <h3>Deal non disponibili</h3>
                <p className={styles.muted}>{copyErrorMessage(deals.error, 'Impossibile caricare i deal.')}</p>
              </div>
            ) : !deals.data || deals.data.items.length === 0 ? (
              <div className={styles.emptyCard}>
                <div className={styles.emptyIcon}><Icon name="search" /></div>
                <h3>Nessun deal trovato</h3>
                <p className={styles.muted}>Affina la ricerca o il filtro cliente per trovare il deal corretto.</p>
              </div>
            ) : (
              <>
                {deals.data.items.map((deal) => {
                  const selected = selectedDeal?.id === deal.id;
                  return (
                    <button
                      key={deal.id}
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      className={`${styles.dealCard} ${selected ? styles.dealCardSelected : ''}`}
                      onClick={() => setSelectedDeal(deal)}
                    >
                      <div className={styles.listItemTop}>
                        <div>
                          <div className={styles.summaryCode}>{deal.codice}</div>
                          <div className={styles.listHeading}>{deal.company_name ?? 'Cliente non disponibile'}</div>
                        </div>
                        <div className={styles.small}>{deal.stage_label ?? 'Stage non disponibile'}</div>
                      </div>
                      <p>{deal.deal_name}</p>
                      <p className={styles.small}>{deal.owner_email ?? 'Owner non disponibile'}</p>
                    </button>
                  );
                })}
                {deals.data.total > deals.data.page_size && (
                  <div className={styles.actionsRow}>
                    <Button variant="secondary" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>
                      Pagina precedente
                    </Button>
                    <Button
                      variant="secondary"
                      disabled={page >= Math.ceil(deals.data.total / deals.data.page_size)}
                      onClick={() => setPage((current) => current + 1)}
                    >
                      Pagina successiva
                    </Button>
                  </div>
                )}
              </>
            )}
          </div>
        </div>

        <div className={styles.formCard}>
          <div className={styles.panelHeader}>
            <div>
              <h2 className={styles.panelTitle}>Dati richiesta</h2>
              <p className={styles.small}>Conferma il deal scelto e prepara il contesto per il team carrier.</p>
            </div>
          </div>

          <div className={styles.formGrid}>
            <div className={styles.heroCard}>
              {selectedDeal ? (
                <div className={styles.sectionSpacer}>
                  <div className={styles.summaryCode}>{selectedDeal.codice}</div>
                  <div className={styles.contextTitle}>{selectedDeal.company_name ?? 'Cliente non disponibile'}</div>
                  <p>{selectedDeal.deal_name}</p>
                  <p className={styles.small}>{selectedDeal.owner_email ?? 'Owner non disponibile'}</p>
                </div>
              ) : (
                <div className={styles.emptyCard}>
                  <div className={styles.emptyIcon}><Icon name="package" /></div>
                  <h3>Seleziona un deal</h3>
                  <p className={styles.muted}>La richiesta può essere inserita solo dopo aver scelto un deal eleggibile.</p>
                </div>
              )}
            </div>

            <div className={styles.fieldGroup}>
              <label htmlFor="indirizzo">Indirizzo</label>
              <input
                id="indirizzo"
                className={styles.fieldInput}
                value={indirizzo}
                onChange={(event) => setIndirizzo(event.target.value)}
                placeholder="Indirizzo del circuito o sede"
              />
            </div>

            <div className={styles.fieldGroup}>
              <label htmlFor="descrizione">Descrizione</label>
              <textarea
                id="descrizione"
                className={styles.textArea}
                value={descrizione}
                onChange={(event) => setDescrizione(event.target.value)}
                placeholder="Dettagli utili per la fattibilità, vincoli e obiettivi attesi"
              />
            </div>

            <div className={styles.fieldGroup}>
              <label htmlFor="fornitori-preferiti">Fornitori preferiti</label>
              <MultiSelect
                options={(fornitori.data ?? []).map((item) => ({ value: item.id, label: item.nome }))}
                selected={fornitoriPreferiti}
                onChange={setFornitoriPreferiti}
                placeholder={fornitori.isLoading ? 'Caricamento fornitori...' : 'Seleziona fornitori'}
              />
            </div>

            <div className={styles.actionsRow}>
              <Button onClick={handleSubmit} loading={createRichiesta.isPending} disabled={!canSubmit}>
                Inserisci RDF
              </Button>
              <Button variant="secondary" onClick={resetForm}>
                Reset
              </Button>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
