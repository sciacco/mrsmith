import type { CoverageProfile, CoverageResult } from '../types';

/* ────────────────────────────────
 * Operator & technology constants
 * ──────────────────────────────── */

const OP_TIM = 1;
const OP_OPEN_FIBER = 3;
const OP_OPEN_FIBER_CD = 4;

export type TierKey =
  | 'premium_dedicated' // Tier 1a — FTTO TIM, BEA Open Fiber (preferred)
  | 'other_dedicated' // Tier 1b — GEA TIM, other FIBRA DEDICATA
  | 'shared_fiber' // Tier 2 — FTTH, XGSPON
  | 'copper_fiber' // Tier 3 — VDSL, EVDSL, VULA_* (distance sensitive)
  | 'fwa' // Tier 3b — FWA
  | 'last_resort'; // Tier 4 — ADSL, SHDSL

export type DistancePerf = 'ottimali' | 'buone' | 'degradate' | 'inutilizzabili';

export const TIER_LABEL: Record<TierKey, string> = {
  premium_dedicated: 'Fibra dedicata premium',
  other_dedicated: 'Fibra dedicata',
  shared_fiber: 'Fibra condivisa',
  copper_fiber: 'Fibra misto rame',
  fwa: 'Wireless',
  last_resort: 'Rame',
};

export const DISTANCE_LABEL: Record<DistancePerf, string> = {
  ottimali: 'Prestazioni ottimali',
  buone: 'Prestazioni buone',
  degradate: 'Prestazioni degradate',
  inutilizzabili: 'Inutilizzabile',
};

/* ────────────────────────────────
 * Speed parsing
 * ──────────────────────────────── */

const SPEED_PAIR_RE = /(\d+(?:\.\d+)?)\s*(G)?\s*\/\s*(\d+(?:\.\d+)?)\s*(G)?/i;
const SPEED_SINGLE_RE = /(\d+(?:\.\d+)?)\s*(G)?/i;

export interface ProfileSpeeds {
  down: number | null;
  up: number | null;
}

function parseProfileSpeeds(profileName: string): ProfileSpeeds {
  const pair = profileName.match(SPEED_PAIR_RE);
  if (pair && pair[1] !== undefined && pair[3] !== undefined) {
    let down = parseFloat(pair[1]);
    let up = parseFloat(pair[3]);
    const matchEnd = (pair.index ?? 0) + pair[0].length;
    const trailingG = /^\s*G\b/i.test(profileName.slice(matchEnd));
    // Any G anywhere in/after the pair means the whole pair is in Gbps.
    const inGbps = pair[2] !== undefined || pair[4] !== undefined || trailingG;
    if (inGbps) {
      down *= 1000;
      up *= 1000;
    }
    return { down, up };
  }
  const single = profileName.match(SPEED_SINGLE_RE);
  if (single && single[1] !== undefined) {
    let down = parseFloat(single[1]);
    if (single[2] !== undefined) down *= 1000;
    return { down, up: null };
  }
  return { down: null, up: null };
}

function findDetail(result: CoverageResult, needle: string): string | null {
  const lower = needle.toLowerCase();
  const found = result.details.find((d) => d.type_name.toLowerCase().includes(lower));
  return found?.value ?? null;
}

function parseNumericDetail(raw: string | null): number | null {
  if (raw === null) return null;
  const match = raw.match(/(\d+(?:\.\d+)?)/);
  if (match && match[1] !== undefined) return parseFloat(match[1]);
  return null;
}

/* ────────────────────────────────
 * Tier classification
 * ──────────────────────────────── */

function classifyTier(tech: string, operatorId: number, profiles: CoverageProfile[]): TierKey {
  const t = tech.trim().toUpperCase();

  if (t === 'FTTO') return 'premium_dedicated';

  if (t === 'FIBRA DEDICATA' || t === 'FIBRA_DEDICATA') {
    // BEA (Open Fiber) = premium; GEA (TIM) and others = secondary
    const isBEA =
      operatorId === OP_OPEN_FIBER || profiles.some((p) => /^\s*BEA\b/i.test(p.name));
    return isBEA ? 'premium_dedicated' : 'other_dedicated';
  }

  if (t === 'FTTH' || t === 'XGSPON') return 'shared_fiber';

  if (
    t === 'VDSL' ||
    t === 'EVDSL' ||
    t === 'VULA_VDSL' ||
    t === 'VULA_EVDSL' ||
    t === 'FTTC'
  ) {
    return 'copper_fiber';
  }

  if (t === 'FWA') return 'fwa';

  // ADSL, SHDSL, unknown
  return 'last_resort';
}

const TIER_BASE: Record<TierKey, number> = {
  premium_dedicated: 1000,
  other_dedicated: 900,
  shared_fiber: 700,
  copper_fiber: 500, // starting base; distance adjusts within copper
  fwa: 200,
  last_resort: 100,
};

/* ────────────────────────────────
 * Distance → performance band (copper-fiber mix only)
 * 0–500 m: ottimali; 500–1000 m: buone; 1000–1500 m: degradate; >1500 m: inutilizzabili
 * ──────────────────────────────── */

function distancePerf(distance: number | null): DistancePerf | null {
  if (distance === null) return null;
  if (distance <= 500) return 'ottimali';
  if (distance <= 1000) return 'buone';
  if (distance <= 1500) return 'degradate';
  return 'inutilizzabili';
}

const COPPER_DISTANCE_BONUS: Record<DistancePerf, number> = {
  ottimali: 100,
  buone: 40,
  degradate: -50,
  inutilizzabili: -700, // pushes below last_resort
};

/* ────────────────────────────────
 * Profile selection within a result
 * FTTH/XGSPON: sweet-spot 2500 preferred; 10G deprioritised (still sellable but not headline).
 * Others: highest downstream wins.
 * ──────────────────────────────── */

export interface SelectedProfile {
  profile: CoverageProfile;
  speeds: ProfileSpeeds;
}

function selectBestProfile(profiles: CoverageProfile[], tech: string): SelectedProfile | null {
  if (profiles.length === 0) return null;
  const parsed: SelectedProfile[] = profiles.map((p) => ({
    profile: p,
    speeds: parseProfileSpeeds(p.name),
  }));

  const t = tech.trim().toUpperCase();
  const isDedicated = t === 'FTTO' || t === 'FIBRA DEDICATA' || t === 'FIBRA_DEDICATA';

  if (isDedicated) {
    // Dedicated: always suggest 1000 (cap) — never the 10G profile.
    const capped = parsed.filter(
      (x) => x.speeds.down !== null && x.speeds.down <= 1000,
    );
    const pool =
      capped.length > 0
        ? capped
        : parsed.filter((x) => x.speeds.down !== null && x.speeds.down < 10000);
    if (pool.length > 0) {
      return pool.reduce((a, b) =>
        (a.speeds.down ?? 0) >= (b.speeds.down ?? 0) ? a : b,
      );
    }
  }

  if (t === 'FTTH' || t === 'XGSPON') {
    const sweet = parsed.find((x) => x.speeds.down === 2500);
    if (sweet) return sweet;
    const nonTenG = parsed.filter(
      (x) => x.speeds.down !== null && x.speeds.down < 10000,
    );
    if (nonTenG.length > 0) {
      return nonTenG.reduce((a, b) =>
        (a.speeds.down ?? 0) >= (b.speeds.down ?? 0) ? a : b,
      );
    }
  }

  return parsed.reduce((a, b) => ((a.speeds.down ?? 0) >= (b.speeds.down ?? 0) ? a : b));
}

/* ────────────────────────────────
 * Sellability (Open Fiber CD) + status modifiers
 * ──────────────────────────────── */

export interface Sellability {
  sellable: boolean;
  reason: string | null;
}

function evaluateSellability(result: CoverageResult): Sellability {
  if (result.operator_id === OP_OPEN_FIBER_CD) {
    const stato = (findDetail(result, 'stato copertura') ?? '').toLowerCase();
    if (!stato.includes('openstream')) {
      return {
        sellable: false,
        reason: 'Infrastruttura presente, nessun servizio di trasporto attivo',
      };
    }
  }
  return { sellable: true, reason: null };
}

/**
 * TIM coverage fascia A-F: A/B/C commercial-friendly, D/E/F require project.
 * Open Fiber (op 3) state: 110 (RFA) > 104 (RFC) > 102 (RFC Bassa Densità).
 */
function modifierFromStatus(result: CoverageResult): { modifier: number; note: string | null } {
  let modifier = 0;
  let note: string | null = null;

  if (result.operator_id === OP_TIM) {
    const fascia = (findDetail(result, 'fascia copertura') ?? '').trim().toUpperCase();
    if (fascia === 'A') modifier += 20;
    else if (fascia === 'B') modifier += 10;
    else if (fascia === 'C') modifier += 0;
    else if (fascia === 'D') {
      modifier -= 30;
      note = 'Fascia D — richiede valutazione';
    } else if (fascia === 'E') {
      modifier -= 60;
      note = 'Fascia E — richiede progetto';
    } else if (fascia === 'F') {
      modifier -= 90;
      note = 'Fascia F — richiede progetto';
    }
  }

  if (result.operator_id === OP_OPEN_FIBER) {
    const stato = (findDetail(result, 'stato copertura') ?? '').toLowerCase();
    if (stato.includes('110') || stato.includes('rfa') || stato.includes('ready for activation')) {
      modifier += 20;
    } else if (stato.includes('104') || stato.includes('ready for connect')) {
      modifier += 10;
    } else if (stato.includes('102') || stato.includes('bassa densità') || stato.includes('bassa densita')) {
      modifier -= 30;
      note = 'Area a bassa densità';
    }
  }

  return { modifier, note };
}

/* ────────────────────────────────
 * Public API
 * ──────────────────────────────── */

export interface CoverageMetrics {
  tier: TierKey;
  tierLabel: string;
  selectedProfile: SelectedProfile | null;
  maxDown: number | null;
  maxUp: number | null;
  stato: string | null;
  fascia: string | null;
  distanza: number | null;
  distancePerf: DistancePerf | null;
  sellable: boolean;
  sellabilityNote: string | null;
  statusNote: string | null;
  score: number;
}

export interface RankedCoverage {
  result: CoverageResult;
  metrics: CoverageMetrics;
}

export function extractMetrics(result: CoverageResult): CoverageMetrics {
  const tier = classifyTier(result.tech, result.operator_id, result.profiles);
  const selectedProfile = selectBestProfile(result.profiles, result.tech);

  const distanza = parseNumericDetail(findDetail(result, 'distanza armadio'));
  const perf = tier === 'copper_fiber' ? distancePerf(distanza) : null;

  // For copper-based (tier 3 + tier 4 + fwa if detail present) prefer measured speed.
  const measuredDown = parseNumericDetail(findDetail(result, 'velocità down'));
  const measuredUp = parseNumericDetail(findDetail(result, 'velocità up'));

  const useMeasured =
    tier === 'copper_fiber' || tier === 'last_resort' || tier === 'fwa';

  const maxDown = useMeasured
    ? measuredDown ?? selectedProfile?.speeds.down ?? null
    : selectedProfile?.speeds.down ?? measuredDown;
  const maxUp = useMeasured
    ? measuredUp ?? selectedProfile?.speeds.up ?? null
    : selectedProfile?.speeds.up ?? measuredUp;

  const sellability = evaluateSellability(result);
  const statusMod = modifierFromStatus(result);

  // Score: tier base + status modifier + distance bonus (copper only) +
  // small profile-speed tiebreak. 10G FTTH gets a mild penalty so the
  // headline profile defaults to the sweet spot.
  let score = TIER_BASE[tier] + statusMod.modifier;

  if (tier === 'copper_fiber' && perf !== null) {
    score += COPPER_DISTANCE_BONUS[perf];
  }

  const profileDown = selectedProfile?.speeds.down ?? 0;
  score += profileDown / 100;

  if (tier === 'shared_fiber' && profileDown >= 10000) {
    score -= 10;
  }

  if (!sellability.sellable) {
    score = -1000; // non-sellable always bottom
  }

  return {
    tier,
    tierLabel: TIER_LABEL[tier],
    selectedProfile,
    maxDown,
    maxUp,
    stato: findDetail(result, 'stato copertura'),
    fascia: findDetail(result, 'fascia copertura'),
    distanza,
    distancePerf: perf,
    sellable: sellability.sellable,
    sellabilityNote: sellability.reason,
    statusNote: statusMod.note,
    score,
  };
}

export function rankCoverage(results: CoverageResult[]): RankedCoverage[] {
  const ranked = results.map((result) => ({ result, metrics: extractMetrics(result) }));
  ranked.sort((a, b) => b.metrics.score - a.metrics.score);
  return ranked;
}
