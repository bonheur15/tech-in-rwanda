import { afterEach, describe, expect, it, vi } from 'vitest';
import { api, invalidateTags } from './generated';
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
});
