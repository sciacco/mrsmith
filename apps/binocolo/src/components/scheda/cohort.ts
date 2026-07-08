// Coorte di navigazione della Scheda azienda (Staffetta).
//
// Gli entry point verso la scheda aprono in `target="_blank" rel="noopener"`:
// `location.state` non attraversa il nuovo tab e nemmeno `sessionStorage`
// (la copia nel tab figlio avviene solo per contesti auxiliary, che noopener
// esclude). Il canale è quindi `localStorage`, condiviso tra i tab della
// stessa origin, con timestamp e TTL per non far sopravvivere coorti stantie
// oltre la sessione di lavoro. L'entry point scrive l'elenco ordinato dei
// companyKey così come l'utente lo vede in quel momento; la scheda (S2) lo
// legge e lo usa solo se coerente con la lente attiva.

export type SchedaLensType = 'ricerca' | 'iniziativa';

export type SchedaCohort = {
  lensType: SchedaLensType;
  lensId: string;
  companyKeys: string[];
};

const COHORT_STORAGE_KEY = 'binocolo.scheda.cohort';

// Cap difensivo: una coorte non serializza più di questo numero di chiavi
// (evita di saturare lo storage con liste patologicamente lunghe).
const MAX_COHORT_KEYS = 500;

// Oltre questa età la coorte è considerata stantia e ignorata (localStorage
// persiste tra sessioni browser, a differenza di sessionStorage).
const COHORT_TTL_MS = 12 * 60 * 60 * 1000;

function isSchedaCohort(value: unknown): value is SchedaCohort {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Record<string, unknown>;
  if (candidate.lensType !== 'ricerca' && candidate.lensType !== 'iniziativa') return false;
  if (typeof candidate.lensId !== 'string') return false;
  if (!Array.isArray(candidate.companyKeys)) return false;
  return candidate.companyKeys.every((key) => typeof key === 'string');
}

// Scrittura best-effort: storage pieno o non disponibile non deve mai
// rompere la navigazione. Da chiamare in un handler onClick/onAuxClick, prima
// che il tab si apra.
export function writeCohort(cohort: SchedaCohort): void {
  try {
    const payload = {
      lensType: cohort.lensType,
      lensId: cohort.lensId,
      companyKeys: cohort.companyKeys.slice(0, MAX_COHORT_KEYS),
      savedAt: Date.now(),
    };
    localStorage.setItem(COHORT_STORAGE_KEY, JSON.stringify(payload));
  } catch {
    // best-effort
  }
}

// Lettura difensiva: undefined su assenza, JSON invalido, shape non conforme
// o coorte più vecchia del TTL.
export function readCohort(): SchedaCohort | undefined {
  try {
    const raw = localStorage.getItem(COHORT_STORAGE_KEY);
    if (!raw) return undefined;
    const parsed: unknown = JSON.parse(raw);
    if (!isSchedaCohort(parsed)) return undefined;
    const savedAt = (parsed as { savedAt?: unknown }).savedAt;
    if (typeof savedAt !== 'number' || Date.now() - savedAt > COHORT_TTL_MS) return undefined;
    return parsed;
  } catch {
    return undefined;
  }
}

// Posizione dell'azienda corrente nella coorte (Staffetta).
export type CohortPosition = {
  lensType: SchedaLensType;
  lensId: string;
  total: number;
  // 1-based, per la copy «azienda N di M».
  position: number;
  prevKey?: string;
  nextKey?: string;
};

// Risolve la staffetta solo se la coorte è coerente con la lente attiva e il
// companyKey corrente è nella lista. Ritorna undefined (degradazione elegante)
// per lente diversa/globale, deep-link, reload, o coorte a un solo elemento
// (M===1: nessuna staffetta — è il caso del dossier iniziativa).
export function resolveCohortPosition(
  cohort: SchedaCohort | undefined,
  lensType: SchedaLensType | 'globale',
  lensId: string | undefined,
  companyKey: string | undefined,
): CohortPosition | undefined {
  if (!cohort || !companyKey) return undefined;
  if (cohort.lensType !== lensType || cohort.lensId !== lensId) return undefined;
  if (cohort.companyKeys.length <= 1) return undefined;
  const index = cohort.companyKeys.indexOf(companyKey);
  if (index === -1) return undefined;
  return {
    lensType: cohort.lensType,
    lensId: cohort.lensId,
    total: cohort.companyKeys.length,
    position: index + 1,
    prevKey: index > 0 ? cohort.companyKeys[index - 1] : undefined,
    nextKey: index < cohort.companyKeys.length - 1 ? cohort.companyKeys[index + 1] : undefined,
  };
}
