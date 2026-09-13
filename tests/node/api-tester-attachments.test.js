'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');

const loadBindings = () => import('../../webui/src/features/apiTester/fileAccountBinding.js');

test('direct-token photos cannot be sent with another token and work after restoring the owner', async () => {
  const { bindAttachedFileCredential, hasAttachmentCredentialMismatch } = await loadBindings();
  // Direct-token upload responses do not contain account_id.
  const photo = bindAttachedFileCredential({ id: 'photo-a', filename: 'photo.png' }, 'token-a');
  assert.equal(hasAttachmentCredentialMismatch([photo], 'token-b'), true);
  assert.equal(hasAttachmentCredentialMismatch([photo], 'token-a'), false);
  assert.equal(JSON.stringify(photo).includes('token-a'), false);
});

test('managed keys share attachments while the original account binding remains intact', async () => {
  const { bindAttachedFileCredential, hasAttachmentCredentialMismatch, getAttachedFileAccountId } = await loadBindings();
  const photo = bindAttachedFileCredential({ id: 'photo-managed', account_id: 'account-a' }, 'managed-key-a', true);
  assert.equal(hasAttachmentCredentialMismatch([photo], 'managed-key-b', true), false);
  assert.equal(getAttachedFileAccountId([photo]), 'account-a');
  assert.equal(JSON.stringify(photo).includes('managed-key-a'), false);
});

test('changing between managed and direct modes cannot reuse another account photo', async () => {
  const { bindAttachedFileCredential, hasAttachmentCredentialMismatch } = await loadBindings();
  const managed = bindAttachedFileCredential({ id: 'managed-photo', account_id: 'account-a' }, 'key', true);
  const direct = bindAttachedFileCredential({ id: 'direct-photo' }, 'key', false);
  assert.equal(hasAttachmentCredentialMismatch([managed], 'key', false), true);
  assert.equal(hasAttachmentCredentialMismatch([direct], 'key', true), true);
});

test('every attachment must belong to the active credentials and removing the foreign photo unblocks sending', async () => {
  const { bindAttachedFileCredential, hasAttachmentCredentialMismatch } = await loadBindings();
  const first = bindAttachedFileCredential({ id: 'photo-a' }, 'token-a');
  const second = bindAttachedFileCredential({ id: 'photo-b' }, 'token-b');
  const files = [first, second];
  assert.equal(hasAttachmentCredentialMismatch(files, 'token-a'), true);
  assert.equal(hasAttachmentCredentialMismatch(files.filter(file => file.id !== second.id), 'token-a'), false);
  assert.equal(hasAttachmentCredentialMismatch([], 'token-b'), false);
});
