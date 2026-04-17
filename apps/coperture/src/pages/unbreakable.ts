import type { RankedCoverage } from './ranking';

export type UnbreakableTier = 'extreme' | 'core' | 'essence';

export const UNBREAKABLE_TIER_LABEL: Record<UnbreakableTier, string> = {
  extreme: 'Extreme',
  core: 'Core',
  essence: 'Essence',
};

export const UNBREAKABLE_TIER_DESC: Record<UnbreakableTier, string> = {
  extreme: 'Due linee dedicate di vendor differenti',
  core: 'Una fibra dedicata più una condivisa, preferibilmente di vendor differenti',
  essence: 'Due linee condivise, preferibilmente di vendor e tecnologie differenti',
};

type CircuitRole = 'shared' | 'dedicated' | 'excluded';

/**
 * Classify a result for Unbreakable pairing:
 * - dedicated: FTTO, FIBRA DEDICATA (fibra pregiata)
 * - excluded: SHDSL (rame dedicato, fuori dal catalogo Unbreakable)
 * - shared: everything else (FTTH, XGSPON, VDSL, EVDSL, FTTC, VULA_*, FWA, ADSL)
 */
function classifyForUnbreakable(tech: string): CircuitRole {
  const t = tech.trim().toUpperCase();
  if (t === 'SHDSL') return 'excluded';
  if (t === 'FTTO' || t === 'FIBRA DEDICATA' || t === 'FIBRA_DEDICATA') return 'dedicated';
  return 'shared';
}

/**
 * Shared-tier preference order per the product rules:
 * FTTH > VDSL/EVDSL/FTTC > FWA > XGSPON > ADSL.
 * XGSPON is demoted (eccessivo), ADSL is deprecated (ultima spiaggia).
 */
function sharedTechRank(tech: string): number {
  const t = tech.trim().toUpperCase();
  if (t === 'FTTH') return 5;
  if (t === 'VDSL' || t === 'EVDSL' || t === 'FTTC' || t === 'VULA_VDSL' || t === 'VULA_EVDSL') return 4;
  if (t === 'FWA') return 3;
  if (t === 'XGSPON') return 2;
  if (t === 'ADSL') return 1;
  return 0;
}

export function isDeprecatedTech(tech: string): boolean {
  const t = tech.trim().toUpperCase();
  return t === 'ADSL';
}

export interface UnbreakableCombo {
  a: RankedCoverage;
  b: RankedCoverage;
  optimal: boolean;
  warnings: string[];
}

export interface UnbreakableCombos {
  extreme: UnbreakableCombo[];
  core: UnbreakableCombo[];
  essence: UnbreakableCombo[];
}

/**
 * Build all valid Unbreakable combinations from sellable coverage results.
 * Non-sellable (OF CD only-passivo) and SHDSL are excluded.
 */
export function buildUnbreakableCombos(sellable: RankedCoverage[]): UnbreakableCombos {
  const pool = sellable.filter(
    (r) => classifyForUnbreakable(r.result.tech) !== 'excluded',
  );

  const dedicated = pool.filter(
    (r) => classifyForUnbreakable(r.result.tech) === 'dedicated',
  );
  const shared = pool.filter(
    (r) => classifyForUnbreakable(r.result.tech) === 'shared',
  );

  const extreme: UnbreakableCombo[] = [];
  for (let i = 0; i < dedicated.length; i++) {
    for (let j = i + 1; j < dedicated.length; j++) {
      const a = dedicated[i];
      const b = dedicated[j];
      if (!a || !b) continue;
      if (a.result.operator_id === b.result.operator_id) continue;
      extreme.push({ a, b, optimal: true, warnings: [] });
    }
  }

  const core: UnbreakableCombo[] = [];
  for (const d of dedicated) {
    for (const s of shared) {
      const warnings: string[] = [];
      if (d.result.operator_id === s.result.operator_id) {
        warnings.push('Stesso vendor per entrambe le linee');
      }
      if (isDeprecatedTech(s.result.tech)) {
        warnings.push('Include una tecnologia deprecata (ADSL)');
      }
      core.push({ a: d, b: s, optimal: warnings.length === 0, warnings });
    }
  }

  const essence: UnbreakableCombo[] = [];
  for (let i = 0; i < shared.length; i++) {
    for (let j = i + 1; j < shared.length; j++) {
      const a = shared[i];
      const b = shared[j];
      if (!a || !b) continue;
      const sameOp = a.result.operator_id === b.result.operator_id;
      const sameTech = a.result.tech.trim().toUpperCase() === b.result.tech.trim().toUpperCase();
      // Two circuits from same vendor AND same technology share failure mode → not redundant.
      if (sameOp && sameTech) continue;
      const warnings: string[] = [];
      if (sameOp) warnings.push('Stesso vendor per entrambe le linee');
      if (sameTech) warnings.push('Stessa tecnologia per entrambe le linee');
      if (isDeprecatedTech(a.result.tech) || isDeprecatedTech(b.result.tech)) {
        warnings.push('Include una tecnologia deprecata (ADSL)');
      }
      essence.push({ a, b, optimal: warnings.length === 0, warnings });
    }
  }

  // Sort combos: optimal first, then by combined shared-tech rank (for essence/core),
  // then by max downstream of the pair.
  function comboScore(c: UnbreakableCombo): number {
    let s = c.optimal ? 1000 : 0;
    s += sharedTechRank(c.a.result.tech) * 10;
    s += sharedTechRank(c.b.result.tech) * 10;
    s += (c.a.metrics.maxDown ?? 0) / 200;
    s += (c.b.metrics.maxDown ?? 0) / 200;
    return s;
  }

  extreme.sort((x, y) => comboScore(y) - comboScore(x));
  core.sort((x, y) => comboScore(y) - comboScore(x));
  essence.sort((x, y) => comboScore(y) - comboScore(x));

  return { extreme, core, essence };
}
