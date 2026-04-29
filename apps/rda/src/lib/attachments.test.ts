import {
  attachmentTypeLabel,
  countQuoteAttachments,
  defaultAttachmentTypeForPOState,
  isAttachmentType,
} from './attachments.js';

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

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

test('attachment type labels match business copy', () => {
  assertEqual(attachmentTypeLabel('quote'), 'Preventivo', 'quote label');
  assertEqual(attachmentTypeLabel('transport_document'), 'Documento di trasporto', 'transport document label');
  assertEqual(attachmentTypeLabel('other'), 'Altro', 'other label');
  assertEqual(attachmentTypeLabel('unexpected'), 'Altro', 'unknown label fallback');
});

test('default attachment type follows PO state', () => {
  assertEqual(defaultAttachmentTypeForPOState('DRAFT'), 'quote', 'draft upload should default to quote');
  assertEqual(defaultAttachmentTypeForPOState('PENDING_VERIFICATION'), 'transport_document', 'verification upload should default to transport document');
  assertEqual(defaultAttachmentTypeForPOState('PENDING_SEND'), 'quote', 'other states keep quote fallback');
});

test('only quote attachments count for submit threshold', () => {
  const quoteCount = countQuoteAttachments([
    { attachment_type: 'quote' },
    { attachment_type: 'transport_document' },
    { attachment_type: 'other' },
    {},
    { attachment_type: 'quote' },
  ]);

  assertEqual(quoteCount, 2, 'only quote attachments should count');
});

test('attachment type guard accepts only Mistra enum values', () => {
  assert(isAttachmentType('quote'), 'quote should be valid');
  assert(isAttachmentType('transport_document'), 'transport document should be valid');
  assert(isAttachmentType('other'), 'other should be valid');
  assert(!isAttachmentType('invoice'), 'invoice should not be valid');
});

console.log('attachments tests passed');
