import { buildSchema, findBreakingChanges } from 'graphql';
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export function breakingChanges(before, after) {
  return findBreakingChanges(buildSchema(before), buildSchema(after));
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [before, after] = process.argv.slice(2);
  if (!before || !after) throw new Error('usage: schema-diff.mjs BASE CURRENT');
  const changes = breakingChanges(readFileSync(before, 'utf8'), readFileSync(after, 'utf8'));
  for (const change of changes) console.error(`${change.type}: ${change.description}`);
  if (changes.length) process.exitCode = 1;
}
