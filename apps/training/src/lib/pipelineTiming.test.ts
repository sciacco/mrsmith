import type { PlanEnrollment } from '../api/types.js';
import { daysUntilPipelineReference, pipelineOverdueDays } from './pipelineTiming.js';

function assertEqual<T>(actual: T, expected: T, message: string) {
  if (actual !== expected) throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
}

function test(name: string, run: () => void) {
  try {
    run();
  } catch (error) {
    throw new Error(`${name}: ${error instanceof Error ? error.message : String(error)}`);
  }
}

function base(overrides: Partial<PlanEnrollment> = {}): PlanEnrollment {
  return {
    id: 'enr-1',
    employeeName: 'Test User',
    employeeEmail: 'test@example.com',
    courseTitle: 'Course',
    status: 'approved',
    year: 2026,
    documentValidated: false,
    complianceRelated: false,
    requiredByRule: false,
    ...overrides,
  };
}

const now = new Date('2026-05-21T10:00:00');

test('in_progress uses plannedEnd, not the already-past plannedStart', () => {
  const enrollment = base({
    status: 'in_progress',
    plannedStart: '2026-03-09',
    plannedEnd: '2026-09-30',
  });

  assertEqual(pipelineOverdueDays(enrollment, now), 0, 'in_progress future plannedEnd overdue days');
  assertEqual(daysUntilPipelineReference(enrollment, now), 132, 'in_progress days until plannedEnd');
});

test('in_progress is overdue only after plannedEnd', () => {
  const enrollment = base({
    status: 'in_progress',
    plannedStart: '2026-03-09',
    plannedEnd: '2026-05-01',
  });

  assertEqual(pipelineOverdueDays(enrollment, now), 20, 'in_progress past plannedEnd overdue days');
});

test('approved uses plannedStart', () => {
  const enrollment = base({
    status: 'approved',
    plannedStart: '2026-05-01',
    plannedEnd: '2026-09-30',
  });

  assertEqual(pipelineOverdueDays(enrollment, now), 20, 'approved past plannedStart overdue days');
});

test('proposed uses plannedStart', () => {
  const enrollment = base({
    status: 'proposed',
    plannedStart: '2026-05-25',
    plannedEnd: '2026-09-30',
  });

  assertEqual(daysUntilPipelineReference(enrollment, now), 4, 'proposed days until plannedStart');
});

test('terminal statuses do not expose a pipeline timing reference', () => {
  const enrollment = base({
    status: 'completed',
    plannedStart: '2026-03-09',
    plannedEnd: '2026-05-01',
  });

  assertEqual(daysUntilPipelineReference(enrollment, now), null, 'completed reference');
  assertEqual(pipelineOverdueDays(enrollment, now), 0, 'completed overdue days');
});

console.log('pipelineTiming tests passed');
