import assert from 'node:assert/strict';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';

const fixturePath = fileURLToPath(new URL('../type-fixtures/public-api-types.ts', import.meta.url));
function formatDiagnostic(diagnostic: ts.Diagnostic): string {
  const message = ts.flattenDiagnosticMessageText(diagnostic.messageText, '\n');
  if (!diagnostic.file || diagnostic.start === undefined) return message;

  const position = diagnostic.file.getLineAndCharacterOfPosition(diagnostic.start);
  return `${diagnostic.file.fileName}:${position.line + 1}:${position.character + 1} - ${message}`;
}

test('public option types compile with their documented shapes', () => {
  const program = ts.createProgram({
    rootNames: [fixturePath],
    options: {
      allowImportingTsExtensions: true,
      esModuleInterop: true,
      isolatedModules: true,
      lib: ['lib.es2022.d.ts', 'lib.dom.d.ts', 'lib.dom.iterable.d.ts'],
      module: ts.ModuleKind.ESNext,
      moduleResolution: ts.ModuleResolutionKind.Bundler,
      noEmit: true,
      skipLibCheck: true,
      strict: true,
      types: [],
      target: ts.ScriptTarget.ES2022,
    },
  });
  const diagnostics = ts.getPreEmitDiagnostics(program);

  assert.equal(diagnostics.length, 0, diagnostics.map(formatDiagnostic).join('\n'));
});
