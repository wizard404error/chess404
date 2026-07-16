import { cpSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const src = resolve(__dirname, '..', 'src', 'cards.json');
const dst = resolve(__dirname, '..', '..', '..', 'services', 'realtime', 'internal', 'match', 'cards.json');

cpSync(src, dst);
console.log(`Copied cards.json to ${dst}`);
