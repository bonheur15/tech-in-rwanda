import { defineMiddleware } from "astro:middleware";

const internalOrigin = import.meta.env.INTERNAL_API_URL ?? "http://127.0.0.1:8081";

export const onRequest = defineMiddleware(async (context, next) => {
  if (!context.url.pathname.startsWith("/api/") && !context.url.pathname.startsWith("/media/")) return next();
  const target = new URL(context.url.pathname + context.url.search, internalOrigin);
  const headers = new Headers(context.request.headers);
  headers.set("x-forwarded-host", context.url.host);
  headers.set("x-forwarded-proto", context.url.protocol.slice(0, -1));
  const response = await fetch(target, {
    method: context.request.method,
    headers,
    body: ["GET", "HEAD"].includes(context.request.method) ? undefined : context.request.body,
    redirect: "manual",
    duplex: "half",
  } as RequestInit & { duplex: "half" });
  return new Response(response.body, { status: response.status, statusText: response.statusText, headers: response.headers });
});
