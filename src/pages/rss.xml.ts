export async function GET({ url }: { url: URL }) {
  const origin = import.meta.env.INTERNAL_API_URL ?? url.origin;
  const response = await fetch(`${origin}/api/v1/public/posts?limit=50`);
  const posts = response.ok ? (await response.json()).data : [];
  const escape = (v: string) => v.replace(/[<>&'"]/g, (c) => ({"<":"&lt;",">":"&gt;","&":"&amp;","'":"&apos;",'"':"&quot;"}[c]!));
  const items = posts.map((p: any) => `<item><title>${escape(p.title)}</title><link>${url.origin}/critiques/${p.slug}/</link><guid>${p.id}</guid><description>${escape(p.excerpt)}</description><pubDate>${new Date(p.publishedAt).toUTCString()}</pubDate></item>`).join("");
  return new Response(`<?xml version="1.0"?><rss version="2.0"><channel><title>Rwanda Free Space</title><link>${url.origin}</link><description>Constructive criticism of technology in Rwanda.</description>${items}</channel></rss>`, { headers: { "Content-Type": "application/rss+xml; charset=utf-8" } });
}
