import type { PlanEnrollment } from '../api/types.js';
import { bucketForEnrollment } from './temporalGrouping.js';

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
    employeeName: 'Test',
    employeeEmail: 'test@example.com',
    courseTitle: 'Course',
    status: 'approved',
    year: 2026,
    documentValidated: false,
    mandatory: false,
    ...overrides,
  };
}

const now = new Date('2026-05-21T10:00:00');

test('today bucket when plannedStart is today or earlier', () => {
  assertEqual(bucketForEnrollment(base({ plannedStart: '2026-05-21' }), now), 'today', 'today');
  assertEqual(bucketForEnrollment(base({ plannedStart: '2026-05-10' }), now), 'today', 'past');
});

test('this_week bucket when within 7 days', () => {
  assertEqual(bucketForEnrollment(base({ plannedStart: '2026-05-25' }), now), 'this_week', 'within week');
  assertEqual(bucketForEnrollment(base({ plannedStart: '2026-05-28' }), now), 'this_week', 'within week 2');
});

test('this_month bucket when within 30 days', () => {
  assertEqual(bucketForEnrollment(base({ plannedStart: '2026-06-10' }), now), 'this_month', 'within month');
});

test('later bucket when beyond 30 days', () => {
  assertEqual(bucketForEnrollment(base({ plannedStart: '2026-09-15' }), now), 'later', 'far future');
});

test('later bucket when no planned date', () => {
  assertEqual(bucketForEnrollment(base({}), now), 'later', 'no date');
});

console.log('temporalGrouping tests passed');
