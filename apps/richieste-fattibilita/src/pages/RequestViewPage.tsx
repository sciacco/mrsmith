import { Button, Skeleton } from '@mrsmith/ui';
import { hasAnyRole } from '@mrsmith/auth-client';
import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAnalysis, useAnalysisJSON, useRichiestaFull, useRichiestaPdf } from '../api/queries';
import { MANAGER_ROLES, budgetLabel, copyErrorMessage, formatCounts, formatDate, parsePositiveId } from '../lib/format';
import { useOptionalAuth } from '../hooks/useOptionalAuth';
import styles from './Workspace.module.css';

type TabKey = 'riepilogo' | 'analisi' | 'azioni' | 'pdf';

export function RequestViewPage() {
  const navigate = useNavigate();
  const params = useParams();
  const { user } = useOptionalAuth();
  const richiestaId = parsePositiveId(params.id);
  const canManage = hasAnyRole(user?.roles, MANAGER_ROLES);
  const [activeTab, setActiveTab] = useState<TabKey>('riepilogo');
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);

  const richiesta = useRichiestaFull(richiestaId);
  const analysisText = useAnalysis(richiestaId, activeTab === 'analisi');
  const analysisJSON = useAnalysisJSON(richiestaId, activeTab === 'azioni');
  const pdf = useRichiestaPdf(richiestaId, activeTab === 'pdf');

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
      <section className={styles.emptyCard}>
        <h3>Identificativo non valido</h3>
        <p className={styles.muted}>Il dettaglio richiesto non puo essere aperto con questo URL.</p>
      </section>
    );
  }

  return (
    <section className={styles.page}>
      {richiesta.isLoading ? (
        <div className={styles.panel}>
          <Skeleton rows={8} />
        </div>
      ) : richiesta.error || !richiesta.data ? (
        <div className={styles.emptyCard}>
          <h3>Vista non disponibile</h3>
          <p className={styles.muted}>{copyErrorMessage(richiesta.error, 'Impossibile caricare la richiesta.')}</p>
        </div>
      ) : (
        <>
          <div className={styles.pageHeader}>
            <div>
              <h1 className={styles.pageTitle}>Visualizza RDF</h1>
              <p className={styles.pageSubtitle}>
                Consulta il riepilogo, l’analisi AI, il PDF condivisibile e le azioni raccomandate per la richiesta selezionata.
              </p>
            </div>
            <div className={styles.headerActions}>
              <Button variant="secondary" onClick={() => navigate('/richieste')}>
                Torna alla consultazione
              </Button>
              {canManage && (
                <Button onClick={() => navigate(`/richieste/${richiestaId}`)}>
                  Gestisci
                </Button>
              )}
            </div>
          </div>

          <div className={styles.heroCard}>
            <div className={styles.heroTop}>
              <div>
                <div className={styles.summaryCode}>{richiesta.data.codice_deal || `RDF #${richiesta.data.id}`}</div>
                <div className={styles.pageTitle} style={{ fontSize: '1.55rem' }}>
                  {richiesta.data.company_name ?? 'Cliente non disponibile'}
                </div>
                <p className={styles.pageSubtitle}>{richiesta.data.deal_name ?? 'Deal non disponibile'}</p>
              </div>
              <span className={styles.pill} data-status={richiesta.data.stato}>
                {richiesta.data.stato}
              </span>
            </div>

            <div className={styles.heroMeta}>
              <div>
                <p className={styles.small}>Indirizzo</p>
                <p>{richiesta.data.indirizzo}</p>
              </div>
              <div>
                <p className={styles.small}>Richiedente</p>
                <p>{richiesta.data.created_by ?? 'Non disponibile'}</p>
              </div>
              <div>
                <p className={styles.small}>Contatori</p>
                <p>{formatCounts(richiesta.data.counts)}</p>
              </div>
            </div>
          </div>

          <div className={styles.tabRow}>
            {(['riepilogo', 'analisi', 'azioni', 'pdf'] as TabKey[]).map((tab) => (
              <button
                key={tab}
                type="button"
                className={`${styles.tabButton} ${activeTab === tab ? styles.tabButtonActive : ''}`}
                onClick={() => setActiveTab(tab)}
              >
                {tab === 'riepilogo' ? 'Riepilogo' : tab === 'analisi' ? 'Analisi' : tab === 'azioni' ? 'Azioni' : 'PDF'}
              </button>
            ))}
          </div>

          {activeTab === 'riepilogo' && (
            <div className={styles.sectionSpacer}>
              <div className={styles.metaGrid}>
                <div className={styles.metaCard}>
                  <div className={styles.sectionLabel}>Data richiesta</div>
                  <div className={styles.metaCardValue}>{formatDate(richiesta.data.data_richiesta)}</div>
                </div>
                <div className={styles.metaCard}>
                  <div className={styles.sectionLabel}>Owner deal</div>
                  <div className={styles.metaCardValue}>{richiesta.data.owner_email ?? 'Non disponibile'}</div>
                </div>
                <div className={styles.metaCard}>
                  <div className={styles.sectionLabel}>Preferenze fornitori</div>
                  <div className={styles.metaCardValue}>
                    {richiesta.data.preferred_supplier_names?.join(', ') || 'Nessuna preferenza'}
                  </div>
                </div>
              </div>

              <div className={styles.panel}>
                <div className={styles.panelHeader}>
                  <div>
                    <h2 className={styles.panelTitle}>Descrizione richiesta</h2>
                  </div>
                </div>
                <p>{richiesta.data.descrizione}</p>
              </div>

              <div className={styles.valuationList}>
                {richiesta.data.fattibilita.map((item) => (
                  <article key={item.id} className={styles.valuationCard}>
                    <div className={styles.listItemTop}>
                      <div>
                        <div className={styles.listHeading}>{item.fornitore_nome}</div>
                        <p className={styles.small}>{item.tecnologia_nome}</p>
                      </div>
                      <span className={styles.pill} data-status={item.stato}>
                        {item.stato}
                      </span>
                    </div>
                    <div className={styles.listItemBottom}>
                      <span className={styles.small}>Budget: {budgetLabel(item.aderenza_budget)}</span>
                      <span className={styles.small}>Copertura: {item.copertura ? 'SI' : 'NO'}</span>
                      <span className={styles.small}>Esito: {formatDate(item.esito_ricevuto_il)}</span>
                    </div>
                    {item.annotazioni && <p className={styles.muted}>{item.annotazioni}</p>}
                  </article>
                ))}
              </div>
            </div>
          )}

          {activeTab === 'analisi' && (
            <div className={styles.panel}>
              {analysisText.isLoading ? (
                <Skeleton rows={10} />
              ) : analysisText.error ? (
                <div className={styles.emptyCard}>
                  <h3>Analisi non disponibile</h3>
                  <p className={styles.muted}>{copyErrorMessage(analysisText.error, 'Il servizio AI non e disponibile in questo momento.')}</p>
                </div>
              ) : (
                <div className={styles.analysisBlock}>{analysisText.data}</div>
              )}
            </div>
          )}

          {activeTab === 'azioni' && (
            <div className={styles.sectionSpacer}>
              {analysisJSON.isLoading ? (
                <div className={styles.panel}><Skeleton rows={10} /></div>
              ) : analysisJSON.error || !analysisJSON.data ? (
                <div className={styles.emptyCard}>
                  <h3>Azioni non disponibili</h3>
                  <p className={styles.muted}>{copyErrorMessage(analysisJSON.error, 'Il riepilogo strutturato non e disponibile.')}</p>
                </div>
              ) : (
                <>
                  <div className={styles.panel}>
                    <div className={styles.panelHeader}>
                      <div>
                        <h2 className={styles.panelTitle}>Azioni raccomandate</h2>
                      </div>
                    </div>
                    <div className={styles.actionList}>
                      {analysisJSON.data.azioni_raccomandate.map((item, index) => (
                        <article key={`${item.azione}-${index}`} className={styles.actionCard}>
                          <div className={styles.listHeading}>{item.azione}</div>
                          <p>{item.fornitore}{item.tecnologia ? ` / ${item.tecnologia}` : ''}</p>
                          <p className={styles.muted}>{item.motivo}</p>
                        </article>
                      ))}
                    </div>
                  </div>

                  <div className={styles.panel}>
                    <div className={styles.panelHeader}>
                      <div>
                        <h2 className={styles.panelTitle}>Valutazioni</h2>
                      </div>
                    </div>
                    <div className={styles.valuationList}>
                      {analysisJSON.data.valutazioni.map((item, index) => (
                        <article key={`${item.fornitore}-${item.tecnologia}-${index}`} className={styles.valuationCard}>
                          <div className={styles.listItemTop}>
                            <div>
                              <div className={styles.listHeading}>{item.fornitore}</div>
                              <p className={styles.small}>{item.tecnologia}</p>
                            </div>
                            <span className={styles.pill} data-status={item.stato}>
                              {item.stato}
                            </span>
                          </div>
                          <div className={styles.listItemBottom}>
                            {item.copertura && <span className={styles.small}>Copertura: {item.copertura}</span>}
                            {item.aderenza_budget && <span className={styles.small}>Budget: {item.aderenza_budget}</span>}
                            {typeof item.durata_mesi === 'number' && <span className={styles.small}>Durata: {item.durata_mesi} mesi</span>}
                          </div>
                          {item.criticita && <p className={styles.muted}>{item.criticita}</p>}
                        </article>
                      ))}
                    </div>
                  </div>
                </>
              )}
            </div>
          )}

          {activeTab === 'pdf' && (
            <div className={styles.panel}>
              {pdf.isLoading ? (
                <Skeleton rows={8} />
              ) : pdf.error || !pdfUrl ? (
                <div className={styles.emptyCard}>
                  <h3>PDF non disponibile</h3>
                  <p className={styles.muted}>{copyErrorMessage(pdf.error, 'Il render PDF non e disponibile in questo momento.')}</p>
                </div>
              ) : (
                <div className={styles.sectionSpacer}>
                  <div className={styles.actionsRow}>
                    <Button variant="secondary" onClick={() => window.open(pdfUrl, '_blank', 'noopener,noreferrer')}>
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
                  <div className={styles.iframeWrap}>
                    <iframe title="Anteprima PDF RDF" src={pdfUrl} />
                  </div>
                </div>
              )}
            </div>
          )}
        </>
      )}
    </section>
  );
}
