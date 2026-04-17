import { Button, Drawer, Icon, Skeleton, Tooltip } from '@mrsmith/ui';
import { hasAnyRole } from '@mrsmith/auth-client';
import { useEffect, useState, type KeyboardEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  useAnalysis,
  useAnalysisJSON,
  useRichiestaFull,
  useRichiestaPdf,
} from '../api/queries';
import type { Fattibilita } from '../api/types';
import { StatusPill, statusTone } from '../components/StatusPill';
import {
  MANAGER_ROLES,
  budgetLabel,
  compactAddress,
  copyErrorMessage,
  formatCountsBreakdown,
  formatDate,
  parsePositiveId,
  stripCompanyPrefix,
} from '../lib/format';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import shared from './shared.module.css';
import styles from './RequestViewPage.module.css';

type TabKey = 'riepilogo' | 'analisi' | 'pdf';

const TAB_DEFS: { key: TabKey; label: string }[] = [
  { key: 'riepilogo', label: 'Riepilogo' },
  { key: 'analisi', label: 'Analisi AI' },
  { key: 'pdf', label: 'PDF' },
];

function formatCurrency(value: number | null): string {
  if (value === null || value === undefined) return '—';
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 2,
  }).format(value);
}

function formatDays(value: number | null): string {
  if (value === null || value === undefined) return '—';
  return `${value} gg`;
}

function formatMonths(value: number | null): string {
  if (value === null || value === undefined) return '—';
  return `${value} mesi`;
}

export function RequestViewPage() {
  const navigate = useNavigate();
  const params = useParams();
  const { user } = useOptionalAuth();
  const richiestaId = parsePositiveId(params.id);
  const canManage = hasAnyRole(user?.roles, MANAGER_ROLES);
  const [activeTab, setActiveTab] = useState<TabKey>('riepilogo');
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState<Fattibilita | null>(null);
  const [drawerTab, setDrawerTab] = useState<'dettaglio' | 'commenti'>('dettaglio');

  const richiesta = useRichiestaFull(richiestaId);
  const analysisText = useAnalysis(richiestaId, activeTab === 'analisi');
  const analysisJSON = useAnalysisJSON(richiestaId, activeTab === 'analisi');
  const pdf = useRichiestaPdf(richiestaId, activeTab === 'pdf');

  useEffect(() => {
    if (detailOpen) setDrawerTab('dettaglio');
  }, [detailOpen?.id]);

  useEffect(() => {
    if (!pdf.data) return;
    const nextUrl = URL.createObjectURL(pdf.data);
    setPdfUrl((current) => {
      if (current) URL.revokeObjectURL(current);
      return nextUrl;
    });
    return () => {
      URL.revokeObjectURL(nextUrl);
    };
  }, [pdf.data]);

  if (richiestaId === null) {
    return (
      <section className={shared.emptyCard}>
        <div className={shared.emptyIconDanger}>
          <Icon name="triangle-alert" />
        </div>
        <h3>Identificativo non valido</h3>
        <p className={shared.muted}>Il dettaglio richiesto non può essere aperto con questo URL.</p>
      </section>
    );
  }

  function handleRowKeyDown(event: KeyboardEvent<HTMLTableRowElement>, item: Fattibilita) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      setDetailOpen(item);
    }
  }

  return (
    <section className={shared.page}>
      {richiesta.isLoading ? (
        <div className={shared.panel}>
          <Skeleton rows={8} />
        </div>
      ) : richiesta.error || !richiesta.data ? (
        <div className={shared.emptyCard}>
          <div className={shared.emptyIconDanger}>
            <Icon name="triangle-alert" />
          </div>
          <h3>Vista non disponibile</h3>
          <p className={shared.muted}>
            {copyErrorMessage(richiesta.error, 'Impossibile caricare la richiesta.')}
          </p>
        </div>
      ) : (
        <>
          <div>
            <button
              type="button"
              className={styles.backLink}
              onClick={() => navigate('/richieste')}
              aria-label="Torna all'elenco richieste"
            >
              <Icon name="chevron-left" size={16} />
              Torna all’elenco
            </button>
            <div className={styles.titleRow}>
              <h1 className={styles.titleMain}>
                <span className={styles.titleCode}>
                  {richiesta.data.codice_deal || `RDF #${richiesta.data.id}`}
                </span>
                <span className={styles.titleSep}>·</span>
                <span>{richiesta.data.company_name ?? 'Cliente non disponibile'}</span>
              </h1>
              <StatusPill tone={statusTone(richiesta.data.stato)} aria-label={`Stato ${richiesta.data.stato}`}>
                {richiesta.data.stato}
              </StatusPill>
              <div className={styles.titleActions}>
                {canManage && (
                  <Button onClick={() => navigate(`/richieste/${richiestaId}`)}>Gestisci</Button>
                )}
              </div>
            </div>
            {(() => {
              const subtitle = stripCompanyPrefix(richiesta.data.deal_name, richiesta.data.company_name);
              if (!subtitle) return null;
              return <p className={styles.dealSubtitle}>{subtitle}</p>;
            })()}
          </div>

          <div className={styles.infoBar}>
            <Tooltip content={richiesta.data.indirizzo || '—'}>
              <span className={`${styles.infoItem} ${styles.infoAddress}`}>
                <Icon name="box" size={14} className={styles.infoIcon} />
                {compactAddress(richiesta.data.indirizzo)}
              </span>
            </Tooltip>
            <span className={styles.infoItem}>
              <Icon name="calendar" size={14} className={styles.infoIcon} />
              {formatDate(richiesta.data.data_richiesta)}
            </span>
            {richiesta.data.created_by && (
              <span className={styles.infoItem}>
                <Icon name="user" size={14} className={styles.infoIcon} />
                <span className={styles.infoLabel}>Richiedente:</span>
                <span className={styles.infoValue}>{richiesta.data.created_by}</span>
              </span>
            )}
            {richiesta.data.owner_email && richiesta.data.owner_email !== richiesta.data.created_by && (
              <span className={styles.infoItem}>
                <Icon name="mail" size={14} className={styles.infoIcon} />
                <span className={styles.infoLabel}>Owner deal:</span>
                <span className={styles.infoValue}>{richiesta.data.owner_email}</span>
              </span>
            )}
            {richiesta.data.preferred_supplier_names && richiesta.data.preferred_supplier_names.length > 0 && (
              <span className={styles.infoItem}>
                <Icon name="settings" size={14} className={styles.infoIcon} />
                <span className={styles.infoLabel}>Preferenze:</span>
                <span className={styles.infoValue}>
                  {richiesta.data.preferred_supplier_names.join(', ')}
                </span>
              </span>
            )}
            <span className={styles.infoItem}>
              <Tooltip content={formatCountsBreakdown(richiesta.data.counts)}>
                <span className={styles.countsBadge}>
                  {richiesta.data.counts.completata}/{richiesta.data.counts.totale} completate
                </span>
              </Tooltip>
            </span>
          </div>

          <div className={shared.tabRow} role="tablist" aria-label="Sezioni RDF">
            {TAB_DEFS.map(({ key, label }) => {
              const active = activeTab === key;
              return (
                <button
                  key={key}
                  type="button"
                  role="tab"
                  id={`tab-${key}`}
                  aria-selected={active}
                  aria-controls={`tabpanel-${key}`}
                  tabIndex={active ? 0 : -1}
                  className={`${shared.tabButton} ${active ? shared.tabButtonActive : ''}`}
                  onClick={() => setActiveTab(key)}
                >
                  {label}
                </button>
              );
            })}
          </div>

          {activeTab === 'riepilogo' && (
            <div
              className={shared.sectionSpacer}
              role="tabpanel"
              id="tabpanel-riepilogo"
              aria-labelledby="tab-riepilogo"
            >
              <div className={shared.panel}>
                <h2 className={shared.panelTitle}>Descrizione richiesta</h2>
                <p style={{ marginTop: '0.6rem', whiteSpace: 'pre-wrap', lineHeight: 1.6 }}>
                  {richiesta.data.descrizione || '—'}
                </p>
              </div>

              {richiesta.data.fattibilita.length === 0 ? (
                <div className={shared.emptyCard}>
                  <div className={shared.emptyIcon}>
                    <Icon name="list" />
                  </div>
                  <h3>Nessuna fattibilità registrata</h3>
                  <p className={shared.muted}>
                    Non sono ancora state aggiunte valutazioni per questa richiesta.
                  </p>
                </div>
              ) : (
                <div className={styles.tableWrap}>
                  <table className={styles.table}>
                    <thead>
                      <tr>
                        <th>Fornitore</th>
                        <th>Tecnologia</th>
                        <th>Stato</th>
                        <th>Copertura</th>
                        <th>Budget</th>
                        <th>Esito</th>
                        <th className={styles.numCell}>Rilascio</th>
                      </tr>
                    </thead>
                    <tbody>
                      {richiesta.data.fattibilita.map((item) => (
                        <tr
                          key={item.id}
                          className={styles.row}
                          role="button"
                          tabIndex={0}
                          onClick={() => setDetailOpen(item)}
                          onKeyDown={(event) => handleRowKeyDown(event, item)}
                          aria-label={`Dettaglio ${item.fornitore_nome} ${item.tecnologia_nome}`}
                        >
                          <td className={styles.fornitoreCell}>{item.fornitore_nome}</td>
                          <td className={styles.tecnologiaCell}>{item.tecnologia_nome}</td>
                          <td>
                            <StatusPill tone={statusTone(item.stato)}>{item.stato}</StatusPill>
                          </td>
                          <td>{item.copertura ? 'Sì' : 'No'}</td>
                          <td>{budgetLabel(item.aderenza_budget)}</td>
                          <td>
                            {item.esito_ricevuto_il ? formatDate(item.esito_ricevuto_il) : '—'}
                          </td>
                          <td className={styles.numCell}>{formatDays(item.giorni_rilascio)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {activeTab === 'analisi' && (
            <div
              className={styles.aiStack}
              role="tabpanel"
              id="tabpanel-analisi"
              aria-labelledby="tab-analisi"
            >
              <div className={shared.panel}>
                <h2 className={shared.panelTitle}>Azioni raccomandate</h2>
                {analysisJSON.isLoading ? (
                  <Skeleton rows={4} />
                ) : analysisJSON.error || !analysisJSON.data ? (
                  <p className={shared.muted}>
                    {copyErrorMessage(analysisJSON.error, 'Raccomandazioni non disponibili.')}
                  </p>
                ) : analysisJSON.data.azioni_raccomandate.length === 0 ? (
                  <p className={shared.muted}>Nessuna azione consigliata.</p>
                ) : (
                  <div className={styles.actionList} style={{ marginTop: '0.6rem' }}>
                    {analysisJSON.data.azioni_raccomandate.map((azione, index) => (
                      <article key={`${azione.azione}-${index}`} className={styles.actionItem}>
                        <div className={styles.actionTitle}>{azione.azione}</div>
                        <div className={styles.actionTarget}>
                          {azione.fornitore}
                          {azione.tecnologia ? ` · ${azione.tecnologia}` : ''}
                        </div>
                        <p className={styles.actionReason}>{azione.motivo}</p>
                      </article>
                    ))}
                  </div>
                )}
              </div>

              {analysisJSON.data && analysisJSON.data.valutazioni.length > 0 && (
                <div className={shared.panel}>
                  <h2 className={shared.panelTitle}>Valutazioni sintetiche</h2>
                  <div className={styles.tableWrap} style={{ marginTop: '0.6rem' }}>
                    <table className={styles.table}>
                      <thead>
                        <tr>
                          <th>Fornitore</th>
                          <th>Tecnologia</th>
                          <th>Stato</th>
                          <th>Copertura</th>
                          <th>Budget</th>
                          <th className={styles.numCell}>Durata</th>
                          <th className={styles.numCell}>Rilascio</th>
                        </tr>
                      </thead>
                      <tbody>
                        {analysisJSON.data.valutazioni.map((valutazione, index) => (
                          <tr key={`${valutazione.fornitore}-${valutazione.tecnologia}-${index}`}>
                            <td className={styles.fornitoreCell}>{valutazione.fornitore}</td>
                            <td className={styles.tecnologiaCell}>{valutazione.tecnologia}</td>
                            <td>
                              <StatusPill tone={statusTone(valutazione.stato)}>
                                {valutazione.stato}
                              </StatusPill>
                            </td>
                            <td>{valutazione.copertura ?? '—'}</td>
                            <td>{valutazione.aderenza_budget ?? '—'}</td>
                            <td className={styles.numCell}>{formatMonths(valutazione.durata_mesi ?? null)}</td>
                            <td className={styles.numCell}>
                              {formatDays(valutazione.giorni_rilascio ?? null)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              <div className={shared.panel}>
                <h2 className={shared.panelTitle}>Riassunto</h2>
                {analysisText.isLoading ? (
                  <Skeleton rows={6} />
                ) : analysisText.error ? (
                  <p className={shared.muted}>
                    {copyErrorMessage(analysisText.error, 'Riassunto non disponibile.')}
                  </p>
                ) : (
                  <div className={shared.analysisBlock} style={{ marginTop: '0.6rem' }}>
                    {analysisText.data}
                  </div>
                )}
              </div>
            </div>
          )}

          {activeTab === 'pdf' && (
            <div
              className={shared.panel}
              role="tabpanel"
              id="tabpanel-pdf"
              aria-labelledby="tab-pdf"
            >
              {pdf.isLoading ? (
                <Skeleton rows={8} />
              ) : pdf.error || !pdfUrl ? (
                <div className={shared.emptyCard}>
                  <div className={shared.emptyIconDanger}>
                    <Icon name="triangle-alert" />
                  </div>
                  <h3>PDF non disponibile</h3>
                  <p className={shared.muted}>
                    {copyErrorMessage(pdf.error, 'Il PDF non è disponibile in questo momento.')}
                  </p>
                </div>
              ) : (
                <div className={shared.sectionSpacer}>
                  <div className={shared.actionsRow}>
                    <Button
                      variant="secondary"
                      onClick={() => window.open(pdfUrl, '_blank', 'noopener,noreferrer')}
                    >
                      Apri in nuova scheda
                    </Button>
                    <Button
                      onClick={() => {
                        const link = document.createElement('a');
                        link.href = pdfUrl;
                        link.download = `rdf-${richiestaId}.pdf`;
                        link.click();
                      }}
                    >
                      Scarica PDF
                    </Button>
                  </div>
                  <div className={shared.iframeWrap}>
                    <iframe title="Anteprima PDF RDF" src={pdfUrl} />
                  </div>
                </div>
              )}
            </div>
          )}
        </>
      )}

      <Drawer
        open={detailOpen !== null}
        onClose={() => setDetailOpen(null)}
        size="lg"
        title={detailOpen ? `${detailOpen.fornitore_nome} · ${detailOpen.tecnologia_nome}` : ''}
        subtitle={
          detailOpen ? (
            <StatusPill tone={statusTone(detailOpen.stato)}>{detailOpen.stato}</StatusPill>
          ) : undefined
        }
      >
        {detailOpen && (
          <>
            <div className={styles.drawerTabs} role="tablist" aria-label="Sezioni dettaglio fattibilità">
              <button
                type="button"
                role="tab"
                aria-selected={drawerTab === 'dettaglio'}
                className={styles.drawerTab}
                onClick={() => setDrawerTab('dettaglio')}
              >
                Dettaglio
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={drawerTab === 'commenti'}
                className={styles.drawerTab}
                disabled
                title="Disponibile presto"
              >
                Commenti
                <span className={styles.drawerTabBadge}>Presto</span>
              </button>
            </div>

            {drawerTab === 'dettaglio' && (
              <div className={styles.drawerBody} role="tabpanel">
                <div className={styles.drawerSectionGroup}>
                  <div className={styles.drawerSectionHeading}>Esito</div>
                  <div className={styles.drawerGrid3}>
                    <div className={styles.drawerField}>
                      <span className={styles.drawerLabel}>Copertura</span>
                      <span className={styles.drawerValue}>{detailOpen.copertura ? 'Sì' : 'No'}</span>
                    </div>
                    <div className={styles.drawerField}>
                      <span className={styles.drawerLabel}>Esito ricevuto</span>
                      <span className={styles.drawerValue}>
                        {detailOpen.esito_ricevuto_il ? formatDate(detailOpen.esito_ricevuto_il) : '—'}
                      </span>
                    </div>
                    <div className={styles.drawerField}>
                      <span className={styles.drawerLabel}>Giorni rilascio</span>
                      <span className={styles.drawerValue}>{formatDays(detailOpen.giorni_rilascio)}</span>
                    </div>
                  </div>
                </div>

                <div className={styles.drawerSectionGroup}>
                  <div className={styles.drawerSectionHeading}>Commerciale</div>
                  <div className={styles.drawerSubgrid}>
                    <div className={styles.drawerGrid3}>
                      <div className={styles.drawerField}>
                        <span className={styles.drawerLabel}>Budget</span>
                        <span className={styles.drawerValue}>{budgetLabel(detailOpen.aderenza_budget)}</span>
                      </div>
                      <div className={styles.drawerField}>
                        <span className={styles.drawerLabel}>Durata</span>
                        <span className={styles.drawerValue}>{formatMonths(detailOpen.durata_mesi)}</span>
                      </div>
                      <div className={styles.drawerField}>
                        <span className={styles.drawerLabel}>Da ordinare</span>
                        <span className={styles.drawerValue}>{detailOpen.da_ordinare ? 'Sì' : 'No'}</span>
                      </div>
                    </div>
                    {canManage && (
                      <div className={styles.drawerGrid}>
                        <div className={styles.drawerField}>
                          <span className={styles.drawerLabel}>NRC</span>
                          <span className={styles.drawerValue}>{formatCurrency(detailOpen.nrc)}</span>
                        </div>
                        <div className={styles.drawerField}>
                          <span className={styles.drawerLabel}>MRC</span>
                          <span className={styles.drawerValue}>{formatCurrency(detailOpen.mrc)}</span>
                        </div>
                      </div>
                    )}
                  </div>
                </div>

                <div className={styles.drawerSectionGroup}>
                  <div className={styles.drawerSectionHeading}>Fornitore</div>
                  <div className={styles.drawerGrid}>
                    <div className={styles.drawerField}>
                      <span className={styles.drawerLabel}>Profilo</span>
                      <span className={styles.drawerValue}>{detailOpen.profilo_fornitore || '—'}</span>
                    </div>
                    <div className={styles.drawerField}>
                      <span className={styles.drawerLabel}>Data richiesta</span>
                      <span className={styles.drawerValue}>{formatDate(detailOpen.data_richiesta)}</span>
                    </div>
                    <div className={styles.drawerField}>
                      <span className={styles.drawerLabel}>Contatto</span>
                      <span className={styles.drawerValue}>{detailOpen.contatto_fornitore || '—'}</span>
                    </div>
                    <div className={styles.drawerField}>
                      <span className={styles.drawerLabel}>Riferimento</span>
                      <span className={styles.drawerValue}>{detailOpen.riferimento_fornitore || '—'}</span>
                    </div>
                  </div>
                </div>

                <div className={styles.drawerSectionGroup}>
                  <div className={styles.drawerSectionHeading}>Note</div>
                  <div className={styles.drawerGrid}>
                    <div className={`${styles.drawerField} ${styles.drawerFull}`}>
                      <span className={styles.drawerLabel}>Descrizione</span>
                      <div className={styles.drawerText}>{detailOpen.descrizione || '—'}</div>
                    </div>
                    <div className={`${styles.drawerField} ${styles.drawerFull}`}>
                      <span className={styles.drawerLabel}>Annotazioni</span>
                      <div className={styles.drawerText}>{detailOpen.annotazioni || '—'}</div>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {drawerTab === 'commenti' && (
              <div className={styles.drawerEmpty} role="tabpanel">
                <Icon name="mail" size={32} />
                <h4>Sezione commenti in arrivo</h4>
                <p className={shared.muted}>
                  Qui potrai scambiare messaggi tra richiedente e carrier manager sulla fattibilità.
                </p>
              </div>
            )}
          </>
        )}
      </Drawer>
    </section>
  );
}
