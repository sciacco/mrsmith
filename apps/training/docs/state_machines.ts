/**
 * CDLAN Training Management Tool — State Machines
 *
 * Specifica eseguibile delle transizioni di stato per:
 *   - enrollment
 *   - certification_award
 *   - training_request
 *
 * Pensata come riferimento applicativo. La validazione effettiva nel backend
 * Go avverrà via traduzione di queste tabelle; un trigger Postgres
 * (validate_enrollment_transition.sql) fornisce la rete di sicurezza finale.
 */

// =============================================================================
// Tipi base
// =============================================================================

export type Actor = "employee" | "manager" | "people_admin" | "system";

export interface TransitionContext {
  actor: Actor;
  /** Per transizioni che richiedono motivazione (cancel, reopen, revert) */
  reason?: string;
  /** Per la guard "piano in stato consentito" */
  planStatus?: "draft" | "open" | "frozen" | "closed";
  /** Per la guard "actual_start definito" */
  actualStart?: Date | null;
  /** Per la guard "course.leads_to_cert_id valorizzato" */
  hasLinkedCertification?: boolean;
  /** Per training_request: piano dell'anno target esiste ed è apribile */
  targetPlanIsOpenable?: boolean;
}

export type TransitionResult =
  | { ok: true }
  | { ok: false; code: string; message: string };

// =============================================================================
// Helpers
// =============================================================================

function deny(code: string, message: string): TransitionResult {
  return { ok: false, code, message };
}

function allow(): TransitionResult {
  return { ok: true };
}

function requireActor(ctx: TransitionContext, allowed: Actor[]): TransitionResult | null {
  if (!allowed.includes(ctx.actor)) {
    return deny(
      "UNAUTHORIZED_ACTOR",
      `Attore '${ctx.actor}' non autorizzato. Consentiti: ${allowed.join(", ")}`,
    );
  }
  return null;
}

function requireReason(ctx: TransitionContext): TransitionResult | null {
  if (!ctx.reason || ctx.reason.trim().length < 3) {
    return deny("REASON_REQUIRED", "Una motivazione di almeno 3 caratteri è obbligatoria");
  }
  return null;
}

// =============================================================================
// 1. ENROLLMENT
// =============================================================================

export type EnrollmentState =
  | "proposed"
  | "approved"
  | "in_progress"
  | "completed"
  | "failed"
  | "cancelled"
  | "expired";

export type EnrollmentTransition =
  | "submit"
  | "approve"
  | "revert_to_proposed"
  | "start"
  | "complete"
  | "fail"
  | "cancel"
  | "expire"
  | "reopen";

/** Mappa (stato_corrente, transizione) → stato_target */
const ENROLLMENT_TARGETS: Record<
  EnrollmentState,
  Partial<Record<EnrollmentTransition, EnrollmentState>>
> = {
  proposed:    { approve: "approved", cancel: "cancelled", expire: "expired" },
  approved:    { revert_to_proposed: "proposed", start: "in_progress",
                 cancel: "cancelled", expire: "expired" },
  in_progress: { complete: "completed", fail: "failed", cancel: "cancelled" },
  completed:   { reopen: "in_progress" },
  failed:      { reopen: "in_progress" },
  cancelled:   { reopen: "in_progress" },
  expired:     { reopen: "in_progress" },
};

const ENROLLMENT_TERMINAL_STATES: Set<EnrollmentState> = new Set([
  "completed",
  "failed",
  "cancelled",
  "expired",
]);

export function isEnrollmentTerminal(s: EnrollmentState): boolean {
  return ENROLLMENT_TERMINAL_STATES.has(s);
}

/**
 * Verifica se una transizione è ammessa.
 * Ritorna {ok:true} se la transizione passa tutte le guard, altrimenti
 * {ok:false, code, message} per logging/UI.
 */
export function attemptEnrollmentTransition(
  current: EnrollmentState,
  transition: EnrollmentTransition,
  ctx: TransitionContext,
): TransitionResult & { target?: EnrollmentState } {
  // 1. Esiste la transizione dallo stato corrente?
  const target = ENROLLMENT_TARGETS[current]?.[transition];
  if (!target) {
    return deny(
      "INVALID_TRANSITION",
      `Transizione '${transition}' non consentita da stato '${current}'`,
    );
  }

  // 2. Guard per transizione
  switch (transition) {
    case "submit":
      // submit è la creazione: di fatto non parte da uno stato esistente.
      // Mantenuta qui per completezza; nel codice di creazione si usa solo la guard piano.
      if (ctx.planStatus && !["draft", "open"].includes(ctx.planStatus)) {
        return deny("PLAN_NOT_OPEN",
          `Piano in stato '${ctx.planStatus}': impossibile creare nuove iscrizioni`);
      }
      break;

    case "approve":
    case "revert_to_proposed": {
      const r = requireActor(ctx, ["people_admin"]);
      if (r) return r;
      if (ctx.planStatus === "closed") {
        return deny("PLAN_CLOSED",
          "Impossibile modificare iscrizioni di un piano chiuso");
      }
      if (transition === "revert_to_proposed") {
        const rr = requireReason(ctx);
        if (rr) return rr;
      }
      break;
    }

    case "start": {
      const r = requireActor(ctx, ["people_admin", "employee"]);
      if (r) return r;
      // Se manca actual_start, si imposta a oggi (side-effect a livello chiamante)
      if (ctx.actualStart && ctx.actualStart > new Date()) {
        return deny("ACTUAL_START_IN_FUTURE",
          "La data di inizio effettivo non può essere nel futuro");
      }
      break;
    }

    case "complete": {
      const r = requireActor(ctx, ["people_admin", "employee"]);
      if (r) return r;
      break;
    }

    case "fail": {
      const r = requireActor(ctx, ["people_admin"]);
      if (r) return r;
      if (ctx.hasLinkedCertification === false) {
        return deny("FAIL_REQUIRES_CERT",
          "Stato 'failed' applicabile solo a corsi collegati a una certificazione");
      }
      break;
    }

    case "cancel": {
      const r = requireActor(ctx, ["people_admin", "manager"]);
      if (r) return r;
      const rr = requireReason(ctx);
      if (rr) return rr;
      break;
    }

    case "expire": {
      const r = requireActor(ctx, ["system"]);
      if (r) return r;
      if (ctx.planStatus !== "closed") {
        return deny("PLAN_NOT_CLOSED",
          "Transizione 'expire' consentita solo alla chiusura del piano");
      }
      break;
    }

    case "reopen": {
      const r = requireActor(ctx, ["people_admin"]);
      if (r) return r;
      const rr = requireReason(ctx);
      if (rr) return rr;
      break;
    }
  }

  return { ...allow(), target };
}

// =============================================================================
// 2. CERTIFICATION_AWARD
// =============================================================================

export type AwardOutcome =
  | "in_progress"
  | "passed_exam"
  | "failed_exam"
  | "attendance_only";

export type AwardTransition =
  | "issue"            // creazione diretta in stato esito noto
  | "mark_passed"      // da in_progress a passed_exam
  | "mark_failed"      // da in_progress a failed_exam
  | "mark_attendance"  // da in_progress a attendance_only
  | "correct";         // amministratore corregge l'outcome (auditato)

const AWARD_TARGETS: Record<
  AwardOutcome,
  Partial<Record<AwardTransition, AwardOutcome>>
> = {
  in_progress: {
    mark_passed: "passed_exam",
    mark_failed: "failed_exam",
    mark_attendance: "attendance_only",
  },
  passed_exam:     { correct: "passed_exam" },     // riresta lo stesso, ma con valori modificati
  failed_exam:     { correct: "failed_exam" },
  attendance_only: { correct: "attendance_only" },
};

export function attemptAwardTransition(
  current: AwardOutcome,
  transition: AwardTransition,
  ctx: TransitionContext,
): TransitionResult & { target?: AwardOutcome } {
  if (transition === "issue") {
    // 'issue' è la creazione; la macchina non parte da uno stato.
    if (current !== undefined) {
      return deny("INVALID_TRANSITION", "'issue' è valido solo in creazione");
    }
    return allow();
  }

  const target = AWARD_TARGETS[current]?.[transition];
  if (!target) {
    return deny(
      "INVALID_TRANSITION",
      `Transizione '${transition}' non consentita da outcome '${current}'`,
    );
  }

  if (transition === "correct") {
    const r = requireActor(ctx, ["people_admin"]);
    if (r) return r;
    const rr = requireReason(ctx);
    if (rr) return rr;
  } else {
    // mark_* possono essere fatte da people_admin o dall'utente con doc verificato
    const r = requireActor(ctx, ["people_admin", "employee"]);
    if (r) return r;
  }

  return { ...allow(), target };
}

// =============================================================================
// 3. TRAINING_REQUEST
// =============================================================================

export type RequestState =
  | "submitted"
  | "under_review"
  | "accepted"
  | "rejected"
  | "converted";

export type RequestTransition =
  | "submit"
  | "start_review"
  | "accept"
  | "reject"
  | "convert"
  | "withdraw"; // self-cancel da parte dell'employee

const REQUEST_TARGETS: Record<
  RequestState,
  Partial<Record<RequestTransition, RequestState>>
> = {
  submitted:    { start_review: "under_review", withdraw: "rejected" },
  under_review: { accept: "accepted", reject: "rejected", withdraw: "rejected" },
  accepted:     { convert: "converted" },
  rejected:     {},
  converted:    {},
};

export function attemptRequestTransition(
  current: RequestState,
  transition: RequestTransition,
  ctx: TransitionContext,
): TransitionResult & { target?: RequestState } {
  if (transition === "submit") {
    const r = requireActor(ctx, ["employee"]);
    if (r) return r;
    return allow();
  }

  const target = REQUEST_TARGETS[current]?.[transition];
  if (!target) {
    return deny(
      "INVALID_TRANSITION",
      `Transizione '${transition}' non consentita da stato '${current}'`,
    );
  }

  switch (transition) {
    case "start_review":
    case "accept": {
      const r = requireActor(ctx, ["people_admin"]);
      if (r) return r;
      break;
    }

    case "reject": {
      const r = requireActor(ctx, ["people_admin"]);
      if (r) return r;
      const rr = requireReason(ctx);
      if (rr) return rr;
      break;
    }

    case "convert": {
      const r = requireActor(ctx, ["people_admin"]);
      if (r) return r;
      if (!ctx.targetPlanIsOpenable) {
        return deny("PLAN_NOT_OPENABLE",
          "Per convertire la richiesta serve un training_plan apribile per l'anno target");
      }
      break;
    }

    case "withdraw": {
      // Solo l'employee che ha creato la richiesta (controllo ownership a livello chiamante)
      const r = requireActor(ctx, ["employee"]);
      if (r) return r;
      break;
    }
  }

  return { ...allow(), target };
}
