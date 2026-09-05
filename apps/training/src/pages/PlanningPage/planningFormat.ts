import { formatInstant, formatLocalDate, formatNumber } from "@mrsmith/format";
import type {
  DeliveryStatus,
  PlanningEventItem,
  PlanningReminder,
  PlanningRequestItem,
} from "../../api/types";

export const deliveryLabels: Record<DeliveryStatus, string> = {
  planned: "pianificate",
  in_progress: "in corso",
  completed: "concluse",
  partially_completed: "parziali",
  not_attended: "non frequentate",
  cancelled: "annullate",
};

export const deliveryStatusLabels: Record<DeliveryStatus, string> = {
  planned: "Pianificata",
  in_progress: "In corso",
  completed: "Conclusa",
  partially_completed: "Parziale",
  not_attended: "Non frequentata",
  cancelled: "Annullata",
};

export function n(value: number) {
  return formatNumber(value);
}
export function localDate(value: string | null | undefined) {
  return value ? (formatLocalDate(value) ?? "—") : "—";
}
export function instant(value: string | null | undefined) {
  return value ? (formatInstant(value) ?? "—") : "—";
}

export function deliverySummary(counts: Record<DeliveryStatus, number>) {
  return (
    (Object.keys(deliveryLabels) as DeliveryStatus[])
      .filter((key) => counts[key] > 0)
      .map((key) => `${n(counts[key])} ${deliveryLabels[key]}`)
      .join(" · ") || "Nessuna iscrizione"
  );
}

export function reminderDate(reminder: PlanningReminder) {
  return reminder.date ? localDate(reminder.date) : "Senza data";
}

export function reminderMonthDay(date: string | null) {
  if (!date) return null;
  const [year, month, day] = date.split("-");
  const label = new Intl.DateTimeFormat("it-IT", { month: "short" }).format(
    new Date(`${date}T00:00:00`),
  );
  return {
    day,
    label: `${label}${year === String(new Date().getFullYear()) ? "" : ` ${year}`}`,
    month,
  };
}

export function requestState(request: PlanningRequestItem) {
  // Un esito chiuso viene sempre prima dell'etichetta di sospensione: la
  // sospensione descrive l'istruttoria aperta, non riscrive la chiusura.
  if (request.outcome)
    return request.outcome === "accepted"
      ? "Accolta"
      : request.outcome === "rejected"
        ? "Respinta"
        : request.outcome === "withdrawn"
          ? "Ritirata"
          : request.outcome;
  if (request.suspended) return "Istruttoria sospesa";
  return request.operative ? "Da trattare" : "Aperta";
}

export function eventState(event: PlanningEventItem) {
  if (event.cancelled) return "Annullato";
  return event.operative ? "Operativo" : "Terminato";
}

export function eventConditions(event: PlanningEventItem) {
  const conditions: string[] = [];
  if (event.withoutSessions) conditions.push("Senza sessioni");
  if (event.unassignedEnrollments)
    conditions.push("Iscrizioni senza assegnazione");
  if (event.needsReconciliation) conditions.push("Da riconciliare");
  return conditions.length
    ? conditions.join(" · ")
    : "Nessuna condizione da segnalare";
}
