import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { buildSchema } from 'graphql';

export function verifyArtifact(directory, expectedCommit) {
  if (!/^[a-f0-9]{40}$/.test(expectedCommit)) throw new Error('full commit SHA required');
  const manifest = JSON.parse(readFileSync(resolve(directory, 'manifest.json'), 'utf8'));
  const schema = readFileSync(resolve(directory, 'schema.graphqls'));
  const digest = createHash('sha256').update(schema).digest('hex');
  if (manifest.format_version !== 1 || manifest.repository !== 'overmindv/api-gateway'
      || manifest.commit !== expectedCommit || manifest.schema_sha256 !== digest) {
    throw new Error('schema provenance or checksum mismatch');
  }
  buildSchema(schema.toString('utf8'));
  return manifest;
}

export function exportArtifact(directory) {
  const commit = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
  execFileSync('git', ['diff', '--exit-code', 'HEAD', '--', 'api/graphql/schema.graphqls']);
  const schema = readFileSync('api/graphql/schema.graphqls');
  buildSchema(schema.toString('utf8'));
  mkdirSync(directory, { recursive: true });
  writeFileSync(resolve(directory, 'schema.graphqls'), schema);
  writeFileSync(resolve(directory, 'manifest.json'), JSON.stringify({
    format_version: 1, repository: 'overmindv/api-gateway', commit,
    schema_sha256: createHash('sha256').update(schema).digest('hex'),
  }, null, 2) + '\n');
  return verifyArtifact(directory, commit);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [command, directory, commit] = process.argv.slice(2);
  if (!directory) throw new Error('artifact directory required');
  if (command === 'export') exportArtifact(directory);
  else if (command === 'verify') verifyArtifact(directory, commit);
  else throw new Error('usage: schema-artifact.mjs export DIR | verify DIR COMMIT');
}
