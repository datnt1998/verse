import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { Project } from '@editor/types/project';
import { compositionFromProject, runAudioMeasurement } from './audio-measurement';
import { getEditorStorage } from '@editor/storage/storage-backend';

function savedProject(): Project {
  return { id: 'saved', name: 'Saved mix', description: '', createdAt: 1, updatedAt: 1, duration: 6, metadata: { width: 1920, height: 1080, fps: 30 }, timeline: {
    tracks: [{ id: 'audio', name: 'Giọng', kind: 'audio', height: 60, locked: false, visible: true, muted: false, solo: false, order: 0 }],
    items: [{ id: 'voice', label: 'Giọng gốc', type: 'audio', trackId: 'audio', from: 0, durationInFrames: 180, mediaId: 'wav', src: '/api/editor/media/wav/file', volume: -3, sourceStart: 0, sourceEnd: 180, sourceDuration: 180, sourceFps: 30 }],
    masterBusDb: -2,
    inPoint: 30, outPoint: 60,
    transitions: [], keyframes: [], compositions: [],
  } };
}

test('measurement uses full saved timeline, not the editor in/out range', () => {
  const project = savedProject();
  const before = structuredClone(project);
  const { composition, compositions } = compositionFromProject(project);
  assert.equal(composition.durationInFrames, 180);
  assert.equal(composition.fps, 30);
  assert.equal(composition.width, 1920);
  assert.equal(composition.height, 1080);
  assert.equal(composition.masterBusDb, -2);
  assert.deepEqual(compositions, []);
  assert.deepEqual(project, before, 'measurement never mutates saved project');
});

test('track mute and saved gain are retained for the exact engine mixer', () => {
  const project = savedProject();
  project.timeline!.tracks[0]!.muted = true;
  const { composition } = compositionFromProject(project);
  assert.equal(composition.tracks[0]?.muted, true);
  assert.equal(composition.tracks[0]?.items[0]?.volume, -3);
});

for (const issued of [true, false]) {
  test(issued ? 'launcher measurement claims the issued id without creating another request' : 'editor measurement still creates its own request', async (t) => {
    t.mock.method(getEditorStorage(), 'loadProject', async () => ({ data: savedProject(), revision: 'rev-new' }));
    const calls: Record<string, unknown>[] = [];
    const request = {
      version: 1, id: 'req-a', nonce: 'nonce-a', status: 'running', createdAt: '2026-10-12T00:00:00.000Z',
      binding: { projectRevision: 'rev-old', settingsHash: 'settings', artifactHash: 'artifacts' },
      ownerId: 'runtime', leaseExpiresAt: null, fence: 'fence-a', chunksReceived: 0, bytesReceived: 0,
      sampleRate: null, channels: null, result: null, error: null, finishedAt: null,
    };
    t.mock.method(globalThis, 'fetch', async (_input: unknown, init?: RequestInit) => {
      calls.push(JSON.parse(String(init?.body)));
      return Response.json({ request });
    });
    // Stop at the existing revision guard, before any engine worker runs.
    await assert.rejects(runAudioMeasurement({
      projectId: 'saved', jobId: 'job-a', ownerId: 'runtime',
      ...(issued ? { request: { requestId: request.id, nonce: request.nonce } } : {}),
    }), /Project đã đổi/);
    assert.deepEqual(calls.map((call) => call.action), issued ? ['claim', 'fail'] : ['create', 'claim', 'fail']);
    const claim = calls.find((call) => call.action === 'claim');
    assert.deepEqual(claim, { action: 'claim', requestId: request.id, nonce: request.nonce, ownerId: 'runtime' });
    const failed = calls.find((call) => call.action === 'fail');
    assert.equal(failed?.requestId, request.id);
    assert.equal(failed?.fence, request.fence, 'failure uses the fence issued by the claim, not its nonce');
  });
}
