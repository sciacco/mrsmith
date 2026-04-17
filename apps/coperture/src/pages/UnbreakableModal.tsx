import { useEffect, useMemo, useState } from 'react';
import { Modal, TabNav } from '@mrsmith/ui';
import type { CoverageResult } from '../types';
import type { RankedCoverage } from './ranking';
import {
  UNBREAKABLE_TIER_DESC,
  UNBREAKABLE_TIER_LABEL,
  isDeprecatedTech,
  type UnbreakableCombo,
  type UnbreakableCombos,
  type UnbreakableTier,
} from './unbreakable';
import styles from './UnbreakableModal.module.css';

interface UnbreakableModalProps {
  open: boolean;
  onClose: () => void;
  combos: UnbreakableCombos;
  address: string;
}

function operatorInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase() || 'OP';
}

function formatMbps(value: number | null): string {
  if (value === null) return '—';
  if (value >= 100) return Math.round(value).toString();
  return value.toFixed(1).replace(/\.0$/, '');
}

function normalizeTech(tech: string): string {
  const key = tech.trim().toUpperCase();
  if (key === 'FTTO') return 'FIBRA DEDICATA';
  return tech;
}

function OperatorLogo({ result }: { result: CoverageResult }) {
  const [failed, setFailed] = useState(false);
  if (result.logo_url && !failed) {
    return (
      <img
        className={styles.logo}
        src={result.logo_url}
        alt=""
        aria-hidden="true"
        onError={() => setFailed(true)}
      />
    );
  }
  return <span className={`${styles.logo} ${styles.logoFallback}`}>{operatorInitials(result.operator_name)}</span>;
}

function CircuitSide({ ranked, label }: { ranked: RankedCoverage; label: string }) {
  const { result, metrics } = ranked;
  const profileName = metrics.selectedProfile?.profile.name;
  const deprecated = isDeprecatedTech(result.tech);
  return (
    <div className={styles.side}>
      <span className={styles.sideLabel}>{label}</span>
      <div className={styles.sideHeader}>
        <OperatorLogo result={result} />
        <div className={styles.sideHeaderText}>
          <span className={styles.sideOperator}>{result.operator_name}</span>
          <div className={styles.sideTechRow}>
            <span className={styles.sideTech}>{normalizeTech(result.tech)}</span>
            {deprecated && <span className={styles.sideDeprecated}>Deprecata</span>}
          </div>
        </div>
      </div>
      <div className={styles.sideSpeed}>
        <span className={styles.sideSpeedValue}>
          {formatMbps(metrics.maxDown)} <span className={styles.sideSpeedSlash}>/</span>{' '}
          {formatMbps(metrics.maxUp)}
        </span>
        <span className={styles.sideSpeedUnit}>Mbps · Down/Up</span>
      </div>
      {profileName && <span className={styles.sideProfile}>{profileName}</span>}
    </div>
  );
}

function ComboRow({ combo, index }: { combo: UnbreakableCombo; index: number }) {
  return (
    <article
      className={`${styles.combo} ${combo.optimal ? styles.comboOptimal : styles.comboWarn}`}
      style={{ animationDelay: `${Math.min(index * 40, 240)}ms` }}
    >
      <div className={styles.comboBody}>
        <CircuitSide ranked={combo.a} label="Linea A" />
        <div className={styles.comboJoin} aria-hidden="true">
          <span className={styles.comboJoinDot} />
          <span className={styles.comboJoinLine} />
          <span className={styles.comboJoinDot} />
        </div>
        <CircuitSide ranked={combo.b} label="Linea B" />
      </div>
      <div className={styles.comboFooter}>
        {combo.optimal ? (
          <span className={`${styles.badge} ${styles.badgeOk}`}>Combinazione ottimale</span>
        ) : (
          <>
            <span className={`${styles.badge} ${styles.badgeWarn}`}>Fallback</span>
            {combo.warnings.map((w) => (
              <span key={w} className={styles.warning}>
                {w}
              </span>
            ))}
          </>
        )}
      </div>
    </article>
  );
}

function TierBody({
  tier,
  combos,
}: {
  tier: UnbreakableTier;
  combos: UnbreakableCombo[];
}) {
  return (
    <section className={`${styles.tier} ${styles[`tier_${tier}`] ?? ''}`}>
      <header className={styles.tierHeader}>
        <div>
          <span className={styles.tierEyebrow}>Unbreakable</span>
          <h3 className={styles.tierTitle}>{UNBREAKABLE_TIER_LABEL[tier]}</h3>
          <p className={styles.tierDesc}>{UNBREAKABLE_TIER_DESC[tier]}</p>
        </div>
        <span className={styles.tierCount}>
          {combos.length} combinazion{combos.length === 1 ? 'e' : 'i'}
        </span>
      </header>

      {combos.length === 0 ? (
        <div className={styles.emptyTier}>
          Nessuna combinazione disponibile a questo indirizzo per il livello{' '}
          {UNBREAKABLE_TIER_LABEL[tier]}.
        </div>
      ) : (
        <div className={styles.comboList}>
          {combos.map((combo, i) => (
            <ComboRow
              key={`${tier}-${combo.a.result.coverage_id}-${combo.b.result.coverage_id}-${i}`}
              combo={combo}
              index={i}
            />
          ))}
        </div>
      )}
    </section>
  );
}

const TIER_ORDER: UnbreakableTier[] = ['extreme', 'core', 'essence'];

export function UnbreakableModal({ open, onClose, combos, address }: UnbreakableModalProps) {
  const total = combos.extreme.length + combos.core.length + combos.essence.length;

  const defaultTier = useMemo<UnbreakableTier>(() => {
    const firstWithCombos = TIER_ORDER.find((t) => combos[t].length > 0);
    return firstWithCombos ?? 'extreme';
  }, [combos]);

  const [activeTier, setActiveTier] = useState<UnbreakableTier>(defaultTier);

  useEffect(() => {
    if (open) setActiveTier(defaultTier);
  }, [open, defaultTier]);

  const tabItems = TIER_ORDER.map((t) => ({
    key: t,
    label: `${UNBREAKABLE_TIER_LABEL[t]} (${combos[t].length})`,
  }));

  return (
    <Modal open={open} onClose={onClose} title="Unbreakable — Connettività ridondata" size="wide">
      <div className={styles.root}>
        <div className={styles.intro}>
          <span className={styles.introAddress}>{address}</span>
          <span className={styles.introCount}>
            {total} combinazion{total === 1 ? 'e' : 'i'} totali
          </span>
        </div>

        <div className={styles.tabs}>
          <TabNav
            items={tabItems}
            activeKey={activeTier}
            onTabChange={(key) => setActiveTier(key as UnbreakableTier)}
          />
        </div>

        <div className={styles.scroll}>
          <TierBody tier={activeTier} combos={combos[activeTier]} />
        </div>
      </div>
    </Modal>
  );
}
