import { ApiError } from '@mrsmith/api-client';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useGenerateQuote } from '../api/queries';
import type { ResourceKey, TierCode } from '../api/types';
import { buildQuotePayload } from '../features/iaas/buildQuotePayload';
import { calculateQuote, normalizeQuantityValue, resetQuoteForm } from '../features/iaas/calculateQuote';
import { downloadBlob, formatMoneyEUR, formatRateEUR } from '../features/iaas/format';
import { pricingTiers, tierOptions } from '../features/iaas/pricing';
import {
  createQuantityFormValues,
  resourceCatalog,
  resourceGroups,
} from '../features/iaas/resourceCatalog';
import styles from './CalcolatoreIaaSPage.module.css';

type CategoryKey = 'computing' | 'storage' | 'sicurezza' | 'addon';

const CATEGORY_META: Record<CategoryKey, { label: string; color: string }> = {
  computing: { label: 'Computing', color: '#6366f1' },
  storage: { label: 'Storage', color: '#8b5cf6' },
  sicurezza: { label: 'Sicurezza', color: '#10b981' },
  addon: { label: 'Add On', color: '#f59e0b' },
};

function groupToCategory(groupId: string): CategoryKey {
  const id = groupId.toLowerCase();
  if (id === 'add on') return 'addon';
  if (id === 'computing' || id === 'storage' || id === 'sicurezza') return id;
  return 'computing';
}

const INCLUSIONI = ['Public IP', 'VPC', 'Firewall', 'Rete 1 Gbps'];

function useCountUp(value: number, durationMs = 380) {
  const [display, setDisplay] = useState(value);
  const fromRef = useRef(value);
  const startRef = useRef<number | null>(null);
  const frameRef = useRef<number | null>(null);

  useEffect(() => {
    fromRef.current = display;
    startRef.current = null;
    const target = value;

    function tick(now: number) {
      if (startRef.current === null) startRef.current = now;
      const elapsed = now - startRef.current;
      const t = Math.min(1, elapsed / durationMs);
      const eased = 1 - Math.pow(1 - t, 3);
      setDisplay(fromRef.current + (target - fromRef.current) * eased);
      if (t < 1) frameRef.current = requestAnimationFrame(tick);
    }

    frameRef.current = requestAnimationFrame(tick);
    return () => {
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  return display;
}

function resolveQuoteError(error: unknown): string {
  if (
    error instanceof ApiError &&
    error.status === 503 &&
    typeof error.body === 'object' &&
    error.body !== null &&
    'error' in error.body &&
    (error.body as { error?: string }).error === 'simulatori_vendita_pdf_not_configured'
  ) {
    return 'Servizio PDF offline. Riprova tra qualche minuto.';
  }
  return 'Non e stato possibile generare il PDF.';
}

export function CalcolatoreIaaSPage() {
  const [tierCode, setTierCode] = useState<TierCode>('diretta');
  const [formValues, setFormValues] = useState(createQuantityFormValues());
  const [actionError, setActionError] = useState<string | null>(null);
  const generateQuote = useGenerateQuote();

  const activeTier = pricingTiers[tierCode];
  const calculation = useMemo(
    () => calculateQuote(formValues, activeTier.rates),
    [formValues, activeTier],
  );
  const animatedMonthly = useCountUp(calculation.monthlyTotal);

  const categories: Array<{ key: CategoryKey; value: number }> = [
    { key: 'computing', value: calculation.dailyTotals.computing },
    { key: 'storage', value: calculation.dailyTotals.storage },
    { key: 'sicurezza', value: calculation.dailyTotals.sicurezza },
    { key: 'addon', value: calculation.dailyTotals.addon },
  ];

  function setDraftValue(key: ResourceKey, nextValue: string) {
    if (!/^\d*$/.test(nextValue)) return;
    setFormValues((current) => ({ ...current, [key]: nextValue }));
    setActionError(null);
  }

  function stepBy(key: ResourceKey, delta: number) {
    setFormValues((current) => {
      const resource = resourceCatalog[key];
      const raw = Number(current[key]);
      let next = (Number.isFinite(raw) ? raw : resource.defaultValue) + delta;
      if (next < resource.min) next = resource.min;
      if (resource.max !== undefined && next > resource.max) next = resource.max;
      return { ...current, [key]: String(next) };
    });
    setActionError(null);
  }

  function normalizeField(key: ResourceKey) {
    setFormValues((current) => ({
      ...current,
      [key]: normalizeQuantityValue(key, current[key]),
    }));
  }

  function handleReset() {
    setTierCode('diretta');
    setFormValues(resetQuoteForm());
    setActionError(null);
    generateQuote.reset();
  }

  async function handleGeneratePdf() {
    setActionError(null);
    try {
      const blob = await generateQuote.mutateAsync(
        buildQuotePayload(calculation, activeTier.rates),
      );
      downloadBlob(blob, `calcolatore-iaas-${tierCode}.pdf`);
    } catch (error) {
      setActionError(resolveQuoteError(error));
    }
  }

  return (
    <div className={styles.page}>
      <header className={styles.toolbar}>
        <div className={styles.tierToggle} role="tablist" aria-label="Listino applicato">
          {tierOptions.map((tier) => (
            <button
              key={tier.code}
              type="button"
              role="tab"
              aria-selected={tier.code === tierCode}
              className={`${styles.tierBtn} ${
                tier.code === tierCode ? styles.tierBtnActive : ''
              }`}
              onClick={() => setTierCode(tier.code)}
            >
              {tier.label}
            </button>
          ))}
        </div>
        <div className={styles.included} aria-label="Inclusi gratuitamente nel listino">
          <span className={styles.includedLabel}>
            <span aria-hidden className={styles.includedIcon}>✓</span>
            Inclusi gratuitamente
          </span>
          <ul className={styles.includedList}>
            {INCLUSIONI.map((item, idx) => (
              <li key={item} className={styles.includedItem}>
                {item}
                {idx < INCLUSIONI.length - 1 ? (
                  <span aria-hidden className={styles.includedSep}>·</span>
                ) : null}
              </li>
            ))}
          </ul>
        </div>
      </header>

      <div className={styles.workspace}>
        <section className={styles.form}>
          {resourceGroups.map((group, idx) => {
            const catKey = groupToCategory(group.id);
            const accent = CATEGORY_META[catKey].color;
            return (
              <div
                key={group.id}
                className={`${styles.section} ${idx > 0 ? styles.sectionDivider : ''}`}
              >
                <header className={styles.sectionHeader}>
                  <span
                    aria-hidden
                    className={styles.sectionDot}
                    style={{ background: accent }}
                  />
                  <h2 className={styles.sectionTitle}>{group.title}</h2>
                </header>
                <div className={styles.fieldGrid}>
                  {group.keys.map((resourceKey) => {
                    const resource = resourceCatalog[resourceKey];
                    const rate = activeTier.rates[resourceKey];
                    const qty = Number(formValues[resourceKey]) || 0;
                    const disableMinus = qty <= resource.min;
                    const disablePlus =
                      resource.max !== undefined && qty >= resource.max;
                    return (
                      <div key={resourceKey} className={styles.field}>
                        <div className={styles.fieldText}>
                          <label htmlFor={`iaas-${resourceKey}`} className={styles.fieldLabel}>
                            {resource.label}
                          </label>
                          <span className={styles.fieldRate}>
                            {formatRateEUR(rate)} / giorno
                          </span>
                        </div>
                        <div className={styles.stepper}>
                          <button
                            type="button"
                            className={styles.stepBtn}
                            onClick={() => stepBy(resourceKey, -1)}
                            disabled={disableMinus}
                            aria-label={`Diminuisci ${resource.label}`}
                          >
                            −
                          </button>
                          <input
                            id={`iaas-${resourceKey}`}
                            type="text"
                            inputMode="numeric"
                            value={formValues[resourceKey]}
                            onChange={(e) => setDraftValue(resourceKey, e.target.value)}
                            onBlur={() => normalizeField(resourceKey)}
                            className={styles.stepInput}
                          />
                          <button
                            type="button"
                            className={styles.stepBtn}
                            onClick={() => stepBy(resourceKey, 1)}
                            disabled={disablePlus}
                            aria-label={`Aumenta ${resource.label}`}
                          >
                            +
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </section>

        <aside className={styles.rail} aria-live="polite">
          <section className={styles.railBlock}>
            <div className={styles.railHeader}>
              <p className={styles.railEyebrow}>Totale preventivo</p>
              <p className={styles.railTierBadge}>{activeTier.label}</p>
            </div>
            <p className={styles.railPrice}>
              <span>{formatMoneyEUR(animatedMonthly)}</span>
              <span className={styles.railPriceUnit}>/ mese</span>
            </p>
            <p className={styles.railMeta}>
              {formatMoneyEUR(calculation.dailyTotals.totale)} al giorno ·{' '}
              {formatMoneyEUR(calculation.monthlyTotal * 12)} all&apos;anno
            </p>
          </section>

          <section className={styles.railBlock}>
            <div className={styles.breakdownHead}>
              <span className={styles.breakdownColLabel}>Categoria</span>
              <span className={styles.breakdownColNum}>Giorno</span>
              <span className={styles.breakdownColNum}>Mese</span>
            </div>
            <ul className={styles.breakdownList}>
              {categories.map((cat) => (
                <li key={cat.key} className={styles.breakdownRow}>
                  <span
                    aria-hidden
                    className={styles.dot}
                    style={{ background: CATEGORY_META[cat.key].color }}
                  />
                  <span className={styles.breakdownLabel}>
                    {CATEGORY_META[cat.key].label}
                  </span>
                  <span className={styles.breakdownValue}>{formatMoneyEUR(cat.value)}</span>
                  <span className={styles.breakdownValueMuted}>
                    {formatMoneyEUR(cat.value * 30)}
                  </span>
                </li>
              ))}
            </ul>
          </section>

          <section className={styles.railBlock}>
            <div className={styles.railActions}>
              <button
                type="button"
                className={styles.btnPrimary}
                onClick={handleGeneratePdf}
                disabled={generateQuote.isPending}
              >
                {generateQuote.isPending ? 'Generazione PDF…' : 'Scarica preventivo'}
              </button>
              <button type="button" className={styles.btnGhost} onClick={handleReset}>
                Azzera
              </button>
            </div>
            {actionError ? (
              <p className={styles.railError} role="alert">
                {actionError}
              </p>
            ) : null}
            <p className={styles.railFootnote}>
              Stima su 30 giorni/mese. Importi IVA esclusa.
            </p>
          </section>
        </aside>
      </div>
    </div>
  );
}
