import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { exportArtifact, verifyArtifact } from './schema-artifact.mjs';

test('artifact is deterministic and rejects tampering or wrong commit', () => {
  const dir = mkdtempSync(join(tmpdir(), 'gateway-schema-'));
  try {
    const manifest = exportArtifact(dir);
    const original = readFileSync(join(dir, 'manifest.json'), 'utf8');
    exportArtifact(dir);
    assert.equal(readFileSync(join(dir, 'manifest.json'), 'utf8'), original);
    assert.equal(verifyArtifact(dir, manifest.commit).commit, manifest.commit);
    assert.throws(() => verifyArtifact(dir, '0'.repeat(40)));
    writeFileSync(join(dir, 'schema.graphqls'), 'type Query { changed: Int }');
    assert.throws(() => verifyArtifact(dir, manifest.commit));
  } finally { rmSync(dir, { recursive: true }); }
});
