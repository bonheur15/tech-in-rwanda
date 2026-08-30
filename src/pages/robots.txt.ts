export function GET({ url }: { url: URL }) {
  return new Response(
    `User-agent: *\nAllow: /\nDisallow: /workspace/\nSitemap: ${url.origin}/sitemap.xml\n`,
    { headers: { 'Content-Type': 'text/plain; charset=utf-8' } },
  );
}
