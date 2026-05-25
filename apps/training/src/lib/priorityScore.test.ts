import type { PlanEnrollment } from '../api/types.js';
import { priorityScore } from './priorityScore.js';

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
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

test('critical scores above warning scores', () => {
  const critical = priorityScore(base({ status: 'expired' }), { now });
  const warning = priorityScore(base({ status: 'approved', plannedStart: '2026-05-25' }), { now });
  assert(critical > warning, `critical (${critical}) should be > warning (${warning})`);
});

test('warning scores above info scores', () => {
  const warning = priorityScore(base({ status: 'approved', plannedStart: '2026-05-25' }), { now });
  const info = priorityScore(base({ status: 'completed' }), { now });
  assert(warning > info, `warning (${warning}) should be > info (${info})`);
});

test('older proposed scores higher than newer proposed (same severity)', () => {
  const older = priorityScore(base({ status: 'proposed', plannedStart: '2026-03-01' }), { now });
  const newer = priorityScore(base({ status: 'proposed', plannedStart: '2026-05-15' }), { now });
  assert(older > newer, `older (${older}) should be > newer (${newer})`);
});

test('required by rule boosts score over optional at same severity', () => {
  const required = priorityScore(base({ status: 'proposed', requiredByRule: true }), { now });
  const optional = priorityScore(base({ status: 'proposed', requiredByRule: false }), { now });
  assert(required > optional, `required (${required}) should be > optional (${optional})`);
});

test('in_progress score ignores old plannedStart when plannedEnd is far in future', () => {
  const withOldStart = priorityScore(
    base({ status: 'in_progress', plannedStart: '2026-03-01', plannedEnd: '2026-12-15' }),
    { now },
  );
  const withoutDates = priorityScore(base({ status: 'in_progress' }), { now });
  assert(withOldStart === withoutDates, `old start (${withOldStart}) should not score above no dates (${withoutDates})`);
});

console.log('priorityScore tests passed');
