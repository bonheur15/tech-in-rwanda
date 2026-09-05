import type { ReactNode } from 'react';

const safeHref = (href: unknown) =>
  typeof href === 'string' && /^(https?:|mailto:|\/|#)/.test(href) ? href : null;
export function RichContent({ node }: { node: any }): ReactNode {
  if (!node) return null;
  const children = (node.content ?? []).map((child: any, index: number) => (
    <RichContent key={index} node={child} />
  ));
  if (node.type === 'text') {
    let value: ReactNode = node.text ?? '';
    for (const mark of node.marks ?? []) {
      if (mark.type === 'bold') value = <strong>{value}</strong>;
      else if (mark.type === 'italic') value = <em>{value}</em>;
      else if (mark.type === 'strike') value = <s>{value}</s>;
      else if (mark.type === 'code') value = <code>{value}</code>;
      else if (mark.type === 'link' && safeHref(mark.attrs?.href))
        value = (
          <a
            href={safeHref(mark.attrs.href)!}
            rel={String(mark.attrs.href).startsWith('http') ? 'noopener noreferrer' : undefined}
          >
            {value}
          </a>
        );
    }
    return value;
  }
  if (node.type === 'doc') return <>{children}</>;
  if (node.type === 'paragraph') return <p>{children}</p>;
  if (node.type === 'heading')
    return node.attrs?.level === 3 ? <h3>{children}</h3> : <h2>{children}</h2>;
  if (node.type === 'blockquote') return <blockquote>{children}</blockquote>;
  if (node.type === 'bulletList') return <ul>{children}</ul>;
  if (node.type === 'orderedList') return <ol>{children}</ol>;
  if (node.type === 'listItem') return <li>{children}</li>;
  if (node.type === 'codeBlock')
    return (
      <pre>
        <code>{(node.content ?? []).map((v: any) => v.text ?? '').join('')}</code>
      </pre>
    );
  if (node.type === 'horizontalRule') return <hr />;
  if (node.type === 'image') {
    const a = node.attrs ?? {},
      placement = ['small', 'center', 'wide', 'full', 'left', 'right'].includes(a.placement)
        ? a.placement
        : 'center',
      max = placement === 'left' || placement === 'right' ? 60 : 100,
      width = Math.max(30, Math.min(max, Number(a.width) || 100));
    return (
      <figure
        className={`rich-image rich-image-${placement}`}
        style={
          {
            '--image-width': `${width}%`,
            '--focal-x': `${(Number(a.focalX) || 0.5) * 100}%`,
            '--focal-y': `${(Number(a.focalY) || 0.5) * 100}%`,
          } as React.CSSProperties
        }
      >
        <div
          className={`rich-image-crop crop-${String(a.cropAspect ?? 'original').replace(':', '-')}`}
        >
          <img src={a.src} alt={a.alt ?? ''} />
        </div>
        {(a.caption || a.credit) && (
          <figcaption>
            {a.caption}
            {a.caption && a.credit ? ' · ' : ''}
            {a.credit}
          </figcaption>
        )}
      </figure>
    );
  }
  return <>{children}</>;
}
