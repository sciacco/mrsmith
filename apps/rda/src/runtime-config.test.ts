import { getRdaQuoteThreshold, setRuntimeConfig } from './runtime-config.js';

function assertEqual<T>(actual: T, expected: T, message: string) {
  if (actual !== expected) throw new Error(`${message}: expected ${String(expected)}, got ${String(actual)}`);
}

function assertThrows(run: () => void, message: string) {
  try {
    run();
  } catch {
    return;
  }
  throw new Error(message);
}

function test(name: string, run: () => void) {
  try {
    run();
  } catch (error) {
    throw new Error(`${name}: ${error instanceof Error ? error.message : String(error)}`);
  }
}

test('runtime config stores RDA quote threshold from backend config', () => {
  setRuntimeConfig({ rdaQuoteThreshold: 4500 });

  assertEqual(getRdaQuoteThreshold(), 4500, 'RDA quote threshold');
});

test('runtime config rejects missing threshold', () => {
  assertThrows(() => setRuntimeConfig({ rdaQuoteThreshold: 0 }), 'zero threshold should be rejected');
});

console.log('runtime-config tests passed');
