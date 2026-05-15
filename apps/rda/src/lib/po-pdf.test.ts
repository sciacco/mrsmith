import { canDownloadPOPDF } from './po-pdf.js';

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

test('PO PDF is available only in approved and post-approval states', () => {
  for (const state of [
    'APPROVED',
    'PENDING_SEND',
    'SENT',
    'PENDING_VERIFICATION',
    'PENDING_DISPUTE',
    'DELIVERED_AND_COMPLIANT',
    'CLOSED',
  ]) {
    assertEqual(canDownloadPOPDF(state), true, `${state} should allow PDF download`);
  }

  for (const state of [
    undefined,
    null,
    '',
    'DRAFT',
    'PENDING_APPROVAL',
    'PENDING_APPROVAL_PAYMENT_METHOD',
    'PENDING_LEASING',
    'PENDING_LEASING_ORDER_CREATION',
    'PENDING_APPROVAL_NO_LEASING',
    'PENDING_BUDGET_INCREMENT',
    'PENDING_PDF_GENERATION',
    'PENDING_ERP_SAVE',
  ]) {
    assertEqual(canDownloadPOPDF(state), false, `${String(state)} should block PDF download`);
  }
});

console.log('po-pdf tests passed');
