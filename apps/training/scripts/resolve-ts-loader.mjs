import { extname, isAbsolute, relative, resolve as resolvePath, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = fileURLToPath(new URL('../../../', import.meta.url));
const fallbackRoots = [
  resolvePath(repositoryRoot, 'apps/training/src'),
  resolvePath(repositoryRoot, 'packages/format/src'),
];

function isRelativeSpecifier(specifier) {
  return specifier === '.' || specifier === '..' || specifier.startsWith('./') || specifier.startsWith('../');
}

function isExtensionless(specifier) {
  const pathname = specifier.split(/[?#]/, 1)[0];
  return extname(pathname) === '';
}

function isWithin(root, filePath) {
  const pathFromRoot = relative(root, filePath);
  return (
    pathFromRoot === '' ||
    (!isAbsolute(pathFromRoot) && pathFromRoot !== '..' && !pathFromRoot.startsWith(`..${sep}`))
  );
}

function addTypeScriptExtension(specifier) {
  const suffixStart = specifier.search(/[?#]/);
  if (suffixStart === -1) return `${specifier}.ts`;
  return `${specifier.slice(0, suffixStart)}.ts${specifier.slice(suffixStart)}`;
}

function isAllowedFallback(context, specifier) {
  if (!context.parentURL?.startsWith('file:')) return false;

  try {
    const parentPath = fileURLToPath(context.parentURL);
    const targetPath = fileURLToPath(new URL(addTypeScriptExtension(specifier), context.parentURL));
    return fallbackRoots.some((root) => isWithin(root, parentPath) && isWithin(root, targetPath));
  } catch {
    return false;
  }
}

function isModuleNotFound(error) {
  return error !== null && typeof error === 'object' && error.code === 'ERR_MODULE_NOT_FOUND';
}

export async function resolve(specifier, context, nextResolve) {
  try {
    return await nextResolve(specifier, context);
  } catch (error) {
    if (!isModuleNotFound(error) || !isRelativeSpecifier(specifier) || !isExtensionless(specifier)) {
      throw error;
    }

    if (!isAllowedFallback(context, specifier)) throw error;

    try {
      return await nextResolve(addTypeScriptExtension(specifier), context);
    } catch {
      throw error;
    }
  }
}
