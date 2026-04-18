import { ApiError } from '@mrsmith/api-client';
import { useState } from 'react';
import { useGenerateQuote } from '../api/queries';
import type { CalculationResult, ResourceKey, TierCode } from '../api/types';
import { buildQuotePayload } from '../features/iaas/buildQuotePayload';
import { calculateQuote, normalizeQuantityValue, resetQuoteForm } from '../features/iaas/calculateQuote';
import {
  createQuantityFormValues,
  resourceCatalog,
  resourceGroups,
  resourceOrder,
} from '../features/iaas/resourceCatalog';
import { downloadBlob, formatMoneyEUR, formatRateEUR } from '../features/iaas/format';
import { pricingTiers, tierOptions } from '../features/iaas/pricing';
import styles from './shared.module.css';

function isErrorBody(value: unknown): value is { error?: string } {
  return typeof value === 'object' && value !== null && 'error' in value;
}

function resolveQuoteError(error: unknown): string {
  if (
    error instanceof ApiError &&
    error.status === 503 &&
    isErrorBody(error.body) &&
    error.body.error === 'simulatori_vendita_pdf_not_configured'
  ) {
    return 'Generazione PDF non disponibile. Il servizio di esportazione non e configurato.';
  }

  return 'Non e stato possibile generare il PDF.';
}

export function CalcolatoreIaaSPage() {
  const [tierCode, setTierCode] = useState<TierCode>('diretta');
  const [formValues, setFormValues] = useState(createQuantityFormValues());
  const [calculation, setCalculation] = useState<CalculationResult | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const generateQuote = useGenerateQuote();

  const activeTier = pricingTiers[tierCode];

  function setDraftValue(key: ResourceKey, nextValue: string) {
    if (!/^\d*$/.test(nextValue)) return;

    setFormValues((current) => ({
      ...current,
      [key]: nextValue,
    }));
    setCalculation(null);
    setActionError(null);
  }

  function normalizeField(key: ResourceKey) {
    setFormValues((current) => ({
      ...current,
      [key]: normalizeQuantityValue(key, current[key]),
    }));
  }

  function handleTierChange(nextTierCode: TierCode) {
    setTierCode(nextTierCode);
    setCalculation(null);
    setActionError(null);
  }

  function handleCalculate() {
    const nextCalculation = calculateQuote(formValues, activeTier.rates);
    setFormValues(nextCalculation.normalizedFormValues);
    setCalculation(nextCalculation);
    setActionError(null);
  }

  function handleReset() {
    setTierCode('diretta');
    setFormValues(resetQuoteForm());
    setCalculation(null);
    setActionError(null);
    generateQuote.reset();
  }

  async function handleGeneratePdf() {
    const nextCalculation = calculateQuote(formValues, activeTier.rates);
    setFormValues(nextCalculation.normalizedFormValues);
    setCalculation(nextCalculation);
    setActionError(null);

    try {
      const blob = await generateQuote.mutateAsync(
        buildQuotePayload(nextCalculation, activeTier.rates),
      );
      downloadBlob(blob, 'calcolatore-iaas.pdf');
    } catch (error) {
      setActionError(resolveQuoteError(error));
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titleBlock}>
          <h1 className={styles.title}>Calcolatore IaaS</h1>
          <p className={styles.subtitle}>
            Simula gli addebiti giornalieri e mensili delle risorse IaaS con il listino Diretta o Indiretta e genera il PDF del preventivo.
          </p>
        </div>
      </div>

      <div className={styles.workspace}>
        <section className={`${styles.card} ${styles.summaryCard}`}>
          <div className={styles.cardHeader}>
            <div>
              <h2 className={styles.cardTitle}>Addebiti giornalieri risorse</h2>
              <p className={styles.cardSubtitle}>Listino attivo {activeTier.label}</p>
            </div>
            <div className={styles.segmented} aria-label="Canale di vendita">
              {tierOptions.map((tier) => (
                <button
                  key={tier.code}
                  type="button"
                  className={`${styles.segmentedItem} ${
                    tier.code === tierCode ? styles.segmentedItemActive : ''
                  }`}
                  onClick={() => handleTierChange(tier.code)}
                >
                  {tier.label}
                </button>
              ))}
            </div>
          </div>

          <table className={styles.rateTable}>
            <thead>
              <tr>
                <th>Risorsa</th>
                <th>Tariffa / giorno</th>
              </tr>
            </thead>
            <tbody>
              {resourceOrder.map((resourceKey) => (
                <tr key={resourceKey}>
                  <td className={styles.rateName}>{resourceCatalog[resourceKey].label}</td>
                  <td className={styles.rateValue}>{formatRateEUR(activeTier.rates[resourceKey])}</td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className={styles.infoBlock}>
            <p className={styles.infoLabel}>Inclusioni</p>
            <p className={styles.infoText}>Public IP, VPC, Firewall, rete 1Gbps</p>
          </div>

          {calculation === null ? (
            <section className={styles.statePanel}>
              <p className={styles.stateEyebrow}>Riepilogo</p>
              <h3 className={styles.stateTitle}>Totali in attesa</h3>
              <p className={styles.stateMessage}>
                Compila le quantita e seleziona Calcola per aggiornare il riepilogo giornaliero e mensile.
              </p>
            </section>
          ) : (
            <>
              <div className={styles.breakdown}>
                <div className={styles.breakdownRow}>
                  <span className={styles.breakdownLabel}>Computing</span>
                  <span className={styles.breakdownValue}>
                    {formatMoneyEUR(calculation.dailyTotals.computing)}
                  </span>
                </div>
                <div className={styles.breakdownRow}>
                  <span className={styles.breakdownLabel}>Storage</span>
                  <span className={styles.breakdownValue}>
                    {formatMoneyEUR(calculation.dailyTotals.storage)}
                  </span>
                </div>
                <div className={styles.breakdownRow}>
                  <span className={styles.breakdownLabel}>Sicurezza</span>
                  <span className={styles.breakdownValue}>
                    {formatMoneyEUR(calculation.dailyTotals.sicurezza)}
                  </span>
                </div>
                <div className={styles.breakdownRow}>
                  <span className={styles.breakdownLabel}>Add On</span>
                  <span className={styles.breakdownValue}>
                    {formatMoneyEUR(calculation.dailyTotals.addon)}
                  </span>
                </div>
              </div>

              <div className={styles.totalsGrid}>
                <div className={styles.totalCard}>
                  <p className={styles.totalLabel}>Totale giornaliero</p>
                  <p className={styles.totalValue}>
                    {formatMoneyEUR(calculation.dailyTotals.totale)}
                  </p>
                </div>
                <div className={`${styles.totalCard} ${styles.totalCardMuted}`}>
                  <p className={styles.totalLabel}>Totale mensile</p>
                  <p className={styles.totalValue}>
                    {formatMoneyEUR(calculation.monthlyTotal)}
                  </p>
                </div>
              </div>
            </>
          )}
        </section>

        <section className={`${styles.card} ${styles.formCard}`}>
          <div className={styles.cardHeader}>
            <div>
              <h2 className={styles.cardTitle}>Quantita risorse</h2>
              <p className={styles.cardSubtitle}>
                Inserisci i volumi richiesti e usa Calcola per aggiornare il riepilogo.
              </p>
            </div>
          </div>

          <div className={styles.groupStack}>
            {resourceGroups.map((group) => (
              <section key={group.id} className={styles.groupCard}>
                <div className={styles.groupHeader}>
                  <h3 className={styles.groupTitle}>{group.title}</h3>
                  <p className={styles.groupSubtitle}>Quantita giornaliere richieste per il simulatore.</p>
                </div>

                <div className={styles.fieldGrid}>
                  {group.keys.map((resourceKey) => {
                    const resource = resourceCatalog[resourceKey];
                    const hintParts = [`Min ${resource.min}`];
                    if (resource.max !== undefined) {
                      hintParts.push(`Max ${resource.max}`);
                    }

                    return (
                      <div key={resourceKey} className={styles.field}>
                        <label htmlFor={resourceKey}>{resource.label}</label>
                        <input
                          id={resourceKey}
                          type="number"
                          min={resource.min}
                          max={resource.max}
                          step={resource.step}
                          value={formValues[resourceKey]}
                          onChange={(event) => setDraftValue(resourceKey, event.target.value)}
                          onBlur={() => normalizeField(resourceKey)}
                        />
                        <span className={styles.fieldHint}>{hintParts.join(' - ')}</span>
                      </div>
                    );
                  })}
                </div>
              </section>
            ))}
          </div>

          {actionError ? (
            <section className={`${styles.statePanel} ${styles.statePanelError}`}>
              <p className={styles.stateEyebrow}>Errore</p>
              <h3 className={styles.stateTitle}>PDF non disponibile</h3>
              <p className={styles.stateMessage}>{actionError}</p>
            </section>
          ) : null}

          <div className={styles.actions}>
            <div className={styles.actionsGroup}>
              <button type="button" className={styles.btnPrimary} onClick={handleCalculate}>
                Calcola
              </button>
              <button type="button" className={styles.btnSecondary} onClick={handleReset}>
                Azzera
              </button>
              <button
                type="button"
                className={styles.btnGhost}
                onClick={handleGeneratePdf}
                disabled={generateQuote.isPending}
              >
                {generateQuote.isPending ? 'Generazione PDF...' : 'Genera PDF'}
              </button>
            </div>

            <p className={styles.formFootnote}>
              Il totale mensile usa il moltiplicatore fisso di 30 giorni.
            </p>
          </div>
        </section>
      </div>
    </div>
  );
}
