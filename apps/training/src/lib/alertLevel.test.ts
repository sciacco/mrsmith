import type { PlanEnrollment } from '../api/types.js';
import { classifyAlertLevel } from './alertLevel.js';

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
    mandatory: false,
    ...overrides,
  };
}

const now = new Date('2026-05-21T10:00:00');

test('expired status is critical', () => {
  assertEqual(classifyAlertLevel(base({ status: 'expired' }), { now }), 'critical', 'expired');
});

test('failed status is critical', () => {
  assertEqual(classifyAlertLevel(base({ status: 'failed' }), { now }), 'critical', 'failed');
});

test('in_progress overdue (plannedEnd in past) is critical', () => {
  assertEqual(
    classifyAlertLevel(base({ status: 'in_progress', plannedEnd: '2026-05-01' }), { now }),
    'critical',
    'in_progress overdue',
  );
});

test('in_progress with plannedEnd within 30 days is warning', () => {
  assertEqual(
    classifyAlertLevel(base({ status: 'in_progress', plannedEnd: '2026-06-10' }), { now }),
    'warning',
    'in_progress nearing end',
  );
});

test('in_progress with plannedEnd far in future is info', () => {
  assertEqual(
    classifyAlertLevel(base({ status: 'in_progress', plannedEnd: '2026-12-15' }), { now }),
    'info',
    'in_progress on track',
  );
});

test('approved with plannedStart in the past is critical (should have started)', () => {
  assertEqual(
    classifyAlertLevel(base({ status: 'approved', plannedStart: '2026-05-01' }), { now }),
    'critical',
    'approved overdue start',
  );
});

test('approved with plannedStart within 7 days is warning', () => {
  assertEqual(
    classifyAlertLevel(base({ status: 'approved', plannedStart: '2026-05-25' }), { now }),
    'warning',
    'approved imminent',
  );
});

test('approved with plannedStart far in future is info', () => {
  assertEqual(
    classifyAlertLevel(base({ status: 'approved', plannedStart: '2026-09-15' }), { now }),
    'info',
    'approved future',
  );
});

test('proposed mandatory is warning', () => {
  assertEqual(
    classifyAlertLevel(base({ status: 'proposed', mandatory: true }), { now }),
    'warning',
    'proposed mandatory',
  );
});

test('proposed optional is info', () => {
  assertEqual(
    classifyAlertLevel(base({ status: 'proposed', mandatory: false }), { now }),
    'info',
    'proposed optional',
  );
});

test('completed is info', () => {
  assertEqual(classifyAlertLevel(base({ status: 'completed' }), { now }), 'info', 'completed');
});

test('cancelled is info', () => {
  assertEqual(classifyAlertLevel(base({ status: 'cancelled' }), { now }), 'info', 'cancelled');
});

console.log('alertLevel tests passed');
