import { register } from 'node:module';
import { resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const scriptsDirectory = fileURLToPath(new URL('.', import.meta.url));
const resolverURL = pathToFileURL(resolve(scriptsDirectory, 'resolve-ts-loader.mjs')).href;

register(resolverURL, import.meta.url);
