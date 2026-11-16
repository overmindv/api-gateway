import test from 'node:test';
import assert from 'node:assert/strict';
import { breakingChanges } from './schema-diff.mjs';

test('removed field and required argument reject compatibility', () => {
  const before = 'type Query { user(id: ID): String, count: Int }';
  const after = 'type Query { user(id: ID!): String }';
  const changes = breakingChanges(before, after);
  assert.ok(changes.some(change => change.type === 'FIELD_REMOVED'));
  assert.ok(changes.some(change => change.type === 'ARG_CHANGED_KIND'));
});
test('adding optional API preserves existing clients', () => {
  assert.deepEqual(breakingChanges('type Query { count: Int }', 'type Query { count: Int, user(id: ID): String }'), []);
});
test('invalid SDL fails closed', () => {
  assert.throws(() => breakingChanges('type Query { count: Int }', 'invalid'));
});
