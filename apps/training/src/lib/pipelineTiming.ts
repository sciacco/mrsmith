import type { PlanEnrollment } from '../api/types.js';

const DAY_MS = 24 * 60 * 60 * 1000;

function startOfDay(date: Date): Date {
  const copy = new Date(date);
  copy.setHours(0, 0, 0, 0);
  return copy;
}

function dayIndex(date: Date): number {
  return Math.floor(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()) / DAY_MS);
}

function parseDate(value: string | undefined): Date | null {
  if (!value) return null;
  const stamped = value.length > 10 ? value : `${value}T00:00:00`;
  const date = new Date(stamped);
  return Number.isFinite(date.getTime()) ? startOfDay(date) : null;
}

export function pipelineReferenceDate(enrollment: PlanEnrollment): Date | null {
  if (enrollment.status === 'proposed' || enrollment.status === 'approved') {
    return parseDate(enrollment.plannedStart);
  }
  if (enrollment.status === 'in_progress') {
    return parseDate(enrollment.plannedEnd);
  }
  return null;
}

export function daysUntilPipelineReference(enrollment: PlanEnrollment, now: Date = new Date()): number | null {
  const reference = pipelineReferenceDate(enrollment);
  if (!reference) return null;
  return dayIndex(reference) - dayIndex(now);
}

export function pipelineOverdueDays(enrollment: PlanEnrollment, now: Date = new Date()): number {
  const daysUntil = daysUntilPipelineReference(enrollment, now);
  if (daysUntil === null || daysUntil >= 0) return 0;
  return -daysUntil;
}
