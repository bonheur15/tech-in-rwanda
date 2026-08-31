import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, invalidateTags, request } from './generated';
const response = (value: unknown) =>
  new Response(JSON.stringify({ data: value, meta: { requestId: 'test' } }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
afterEach(() => invalidateTags('archive', 'post', 'comments', 'bookmarks', 'media'));
describe('generated client', () => {
  it('deduplicates and caches GET requests', async () => {
    let done!: (v: Response) => void;
    const fetcher = vi.fn(() => new Promise<Response>((r) => (done = r)));
    const one = api.listPosts({
      baseUrl: 'https://example.test',
      fetch: fetcher,
    });
    const two = api.listPosts({
      baseUrl: 'https://example.test',
      fetch: fetcher,
    });
    done(response([]));
    await Promise.all([one, two]);
    await api.listPosts({ baseUrl: 'https://example.test', fetch: fetcher });
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it('returns structured failures', async () => {
    const fetcher = vi.fn(
      async () =>
        new Response(JSON.stringify({ error: { code: 'nope', message: 'No' } }), { status: 403 }),
    );
    await expect(
      api.listPosts({ baseUrl: 'https://fail.test', fetch: fetcher }),
    ).rejects.toMatchObject({ status: 403, code: 'nope' });
  });
  it('adds CSRF and idempotency headers to mutations', async () => {
    const fetcher = vi.fn(async (_url: string | URL | Request, init?: RequestInit) => {
      const headers = new Headers(init?.headers);
      expect(headers.get('X-CSRF-Token')).toBe('csrf-secret');
      expect(headers.get('Idempotency-Key')).toBe('operation-1');
      return response({ versionId: 'v1' });
    });
    await api.publish('post-1', {
      baseUrl: 'https://example.test',
      fetch: fetcher as typeof fetch,
      csrfToken: 'csrf-secret',
      idempotencyKey: 'operation-1',
    });
  });
  it('does not set JSON content type for multipart uploads', async () => {
    const fetcher = vi.fn(async (_url: string | URL | Request, init?: RequestInit) => {
      expect(new Headers(init?.headers).has('Content-Type')).toBe(false);
      expect(init?.body).toBeInstanceOf(FormData);
      return response({ id: 'asset', src: '/media/asset/large.jpg' });
    });
    const form = new FormData();
    form.set('alt', 'Evidence');
    await api.upload(form, {
      baseUrl: 'https://example.test',
      fetch: fetcher as typeof fetch,
      csrfToken: 'csrf',
    });
  });
  it('invalidates tagged GET cache after a mutation', async () => {
    let calls = 0;
    const fetcher = vi.fn(async (_url: string | URL | Request, init?: RequestInit) => {
      calls++;
      return response(init?.method === 'GET' ? [] : { versionId: 'v1' });
    });
    const options = { baseUrl: 'https://invalidate.test', fetch: fetcher as typeof fetch };
    await api.listPosts(options);
    await api.listPosts(options);
    await api.publish('post-1', { ...options, csrfToken: 'csrf' });
    await api.listPosts(options);
    expect(calls).toBe(3);
  });
  it('rejects invalid envelopes and supports cancellation', async () => {
    const invalid = vi.fn(async () => new Response('{}', { status: 200 }));
    await expect(
      api.listPosts({ baseUrl: 'https://invalid.test', fetch: invalid }),
    ).rejects.toMatchObject({ code: 'invalid_response' });
    const controller = new AbortController();
    const hanging = vi.fn(
      (_url: string | URL | Request, init?: RequestInit) =>
        new Promise<Response>((_resolve, reject) =>
          init?.signal?.addEventListener('abort', () =>
            reject(new DOMException('Aborted', 'AbortError')),
          ),
        ),
    );
    const pending = request('GET', '/api/v1/public/posts?cancel=1', undefined, {
      baseUrl: 'https://cancel.test',
      fetch: hanging as typeof fetch,
      signal: controller.signal,
    });
    controller.abort();
    await expect(pending).rejects.toMatchObject({ code: 'request_aborted' });
  });
});
