import { afterEach, describe, expect, it, vi } from "vitest";

import { APIRequestError, clearServerTimeCache, getServerTime, type ServerTimeResponse } from "./generated";

const payload: ServerTimeResponse = {
  data: {
    iso: "2026-08-30T22:15:23+02:00",
    display: "Sunday, 30 August 2026 at 22:15:23 CAT",
    timeZone: "Africa/Kigali",
    unixMilliseconds: 1_788_140_123_000,
  },
  meta: { requestId: "request-test" },
};

afterEach(() => {
  clearServerTimeCache();
  vi.useRealTimers();
});

describe("getServerTime", () => {
  it("caches a successful response for the configured TTL", async () => {
    const fetcher = vi.fn(async () => jsonResponse(payload));
    const options = { baseUrl: "https://api.example.test", fetch: fetcher, ttlMs: 10_000 };

    const first = await getServerTime(options);
    const second = await getServerTime(options);

    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(first.client.source).toBe("network");
    expect(second.client.source).toBe("cache");
    expect(second.data).toEqual(payload.data);
  });

  it("deduplicates identical requests already in flight", async () => {
    let complete!: (response: Response) => void;
    const fetcher = vi.fn(() => new Promise<Response>((resolve) => (complete = resolve)));
    const options = { baseUrl: "https://dedupe.example.test", fetch: fetcher };

    const first = getServerTime(options);
    const second = getServerTime(options);
    complete(jsonResponse(payload));

    const [firstResult, secondResult] = await Promise.all([first, second]);
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(firstResult.client.source).toBe("network");
    expect(secondResult.client.source).toBe("shared");
  });

  it("bypasses cached data when forceRefresh is enabled", async () => {
    const fetcher = vi.fn(async () => jsonResponse(payload));
    const baseOptions = { baseUrl: "https://refresh.example.test", fetch: fetcher };

    await getServerTime(baseOptions);
    await getServerTime({ ...baseOptions, forceRefresh: true });

    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("rejects malformed successful responses", async () => {
    const fetcher = vi.fn(async () => jsonResponse({ unexpected: true }));

    await expect(getServerTime({ baseUrl: "https://invalid.example.test", fetch: fetcher })).rejects.toMatchObject({
      name: "APIRequestError",
      code: "invalid_response",
    });
  });

  it("turns request timeouts into structured errors", async () => {
    vi.useFakeTimers();
    const fetcher = vi.fn((_input: RequestInfo | URL, init?: RequestInit) =>
      new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener("abort", () => reject(init.signal?.reason), { once: true });
      }),
    );

    const request = getServerTime({ baseUrl: "https://timeout.example.test", fetch: fetcher, timeoutMs: 10 });
    await vi.advanceTimersByTimeAsync(11);

    await expect(request).rejects.toBeInstanceOf(APIRequestError);
    await expect(request).rejects.toMatchObject({ code: "request_aborted" });
  });
});

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
