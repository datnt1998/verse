import assert from 'node:assert/strict';
import { test } from 'node:test';
import { claimMeasurementApi, createMeasurementApi, fetchMeasurementRuntime, MeasurementApiError, measurementFailure, uploadMeasurementChunk } from './measurement-runtime';


test('runtime client encodes identifiers, binds claims and preserves binary PCM', async (t) => {
  const original = globalThis.fetch;
  const calls: { url: string; init?: RequestInit }[] = [];
  t.after(() => { globalThis.fetch = original; });
  globalThis.fetch = async (input, init) => {
    calls.push({ url: String(input), init });
    return Response.json({
      request: {
        version: 1, id: 'r', nonce: 'nonce', status: 'queued', createdAt: '2026-10-12T00:00:00.000Z',
        binding: { projectRevision: 'rev', settingsHash: 'settings', artifactHash: 'artifacts' },
        ownerId: null, leaseExpiresAt: null, fence: null, chunksReceived: 0, bytesReceived: 0,
        sampleRate: null, channels: null, result: null, error: null, finishedAt: null,
      },
      runtime: { workspaceId: 'workspace', ready: true, capabilities: { audioDecode: true, gpuRequired: false } },
      clone: null, chunksReceived: 1, bytesReceived: 8,
    });
  };
  await fetchMeasurementRuntime('project a');
  await createMeasurementApi('job/a');
  await claimMeasurementApi('job/a', { requestId: 'r', nonce: 'nonce', ownerId: 'owner' });
  const bytes = new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8]);
  await uploadMeasurementChunk('job/a', { requestId: 'r', nonce: 'nonce', fence: 'f', seq: 0, sampleRate: 48000, channels: 2, bytes });
  assert.match(calls[0]!.url, /projectId=project%20a/);
  assert.match(calls[1]!.url, /job%2Fa/);
  assert.deepEqual(JSON.parse(String(calls[2]!.init?.body)), { action: 'claim', requestId: 'r', nonce: 'nonce', ownerId: 'owner' });
  assert.deepEqual(calls[3]!.init?.body, bytes);
  assert.equal(calls[3]!.init?.method, 'PUT');
});

test('runtime errors preserve status and recovery body', async (t) => {
  const original = globalThis.fetch;
  t.after(() => { globalThis.fetch = original; });
  globalThis.fetch = async () => Response.json({ error: 'Nonce đo không khớp', code: 'conflict' }, { status: 409 });
  await assert.rejects(createMeasurementApi('job'), (error: unknown) => error instanceof MeasurementApiError && error.status === 409 && error.message === 'Nonce đo không khớp');
});

test('raw browser decode failures become typed Vietnamese failures', () => {
  const failure = measurementFailure(new Error('Unable to decode audio data'));
  assert.equal(failure.code, 'audio_decode_failed');
  assert.doesNotMatch(failure.message, /Unable|decode/i);
});

test('a typed audio decode error keeps its stable code and safe message', () => {
  const error = Object.assign(new Error('Không giải mã được âm thanh của tệp nguồn. Kiểm tra tệp âm thanh rồi đo lại.'), {
    name: 'AudioDecodeError',
    code: 'audio_decode_failed',
  });
  assert.deepEqual(measurementFailure(error), {
    code: 'audio_decode_failed',
    message: 'Không giải mã được âm thanh của tệp nguồn. Kiểm tra tệp âm thanh rồi đo lại.',
  });
});

test('unknown English engine failures never reach the journal verbatim', () => {
  const failure = measurementFailure(new Error('AudioSampleSink exploded at offset 12'));
  assert.equal(failure.code, 'internal_error');
  assert.doesNotMatch(failure.message, /AudioSampleSink|offset/);
});

test('existing Vietnamese diagnostics stay actionable', () => {
  const failure = measurementFailure(new Error('Bộ trộn âm thanh không trả stereo 48 kHz'));
  assert.deepEqual(failure, {
    code: 'internal_error',
    message: 'Bộ trộn âm thanh không trả stereo 48 kHz',
  });
});

test('cancellation is reported as a typed Vietnamese cancellation', () => {
  assert.deepEqual(measurementFailure(new DOMException('Đã huỷ', 'AbortError')), {
    code: 'cancelled',
    message: 'Đã huỷ đo.',
  });
});
