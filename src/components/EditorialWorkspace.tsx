import { useEffect, useMemo, useState } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Image from '@tiptap/extension-image';

const csrf = () =>
  decodeURIComponent(
    document.cookie
      .split('; ')
      .find((v) => v.startsWith('__Host-rfs_csrf=') || v.startsWith('rfs_csrf='))
      ?.split('=')[1] ?? '',
  );
async function api(path: string, init: RequestInit = {}) {
  const response = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrf(),
      ...(init.headers ?? {}),
    },
  });
  const body = await response.json();
  if (!response.ok) throw new Error(body.error?.message ?? 'Request failed');
  return body.data;
}

export default function EditorialWorkspace() {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [signedIn, setSignedIn] = useState(false);
  const [postId, setPostId] = useState('');
  const [title, setTitle] = useState('Untitled critique');
  const [excerpt, setExcerpt] = useState('');
  const [revision, setRevision] = useState(1);
  const [saveState, setSaveState] = useState('saved');
  const [error, setError] = useState('');
  const editor = useEditor({
    extensions: [StarterKit, Image.configure({ allowBase64: false })],
    content: '<p>Start with what you observed, then explain why it matters.</p>',
    immediatelyRender: false,
    onUpdate: () => setSaveState(navigator.onLine ? 'waiting' : 'offline'),
  });
  const payload = useMemo(() => editor?.getJSON(), [editor, saveState]);

  useEffect(() => {
    if (!postId || saveState !== 'waiting') return;
    const timer = window.setTimeout(async () => {
      setSaveState('saving');
      try {
        const p = await api(`/api/v1/posts/${postId}/draft`, {
          method: 'PUT',
          body: JSON.stringify({
            title,
            excerpt,
            content: editor?.getJSON(),
            revision,
          }),
        });
        setRevision(p.revision);
        setSaveState('saved');
      } catch (e) {
        setError(String(e));
        setSaveState('retrying');
      }
    }, 2000);
    return () => clearTimeout(timer);
  }, [postId, saveState, title, excerpt, payload]);
  if (!signedIn)
    return (
      <section className="mx-auto max-w-md rounded-sm border border-line bg-white p-8">
        <p className="text-xs font-bold uppercase tracking-widest text-accent">Staff workspace</p>
        <h1 className="mt-3 font-display text-4xl">Write with room to think.</h1>
        <label className="mt-8 block text-sm">
          Email
          <input
            className="mt-2 w-full border border-line p-3"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        <button
          className="mt-4 w-full bg-ink p-3 text-canvas"
          onClick={async () => {
            await api('/api/v1/auth/staff/request-otp', {
              method: 'POST',
              body: JSON.stringify({ email }),
            });
            setError('Code sent. In development, check the API terminal.');
          }}
        >
          Send sign-in code
        </button>
        <label className="mt-5 block text-sm">
          Six-digit code
          <input
            className="mt-2 w-full border border-line p-3"
            inputMode="numeric"
            value={code}
            onChange={(e) => setCode(e.target.value)}
          />
        </label>
        <button
          className="mt-4 w-full bg-accent p-3 text-white"
          onClick={async () => {
            try {
              await api('/api/v1/auth/staff/verify-otp', {
                method: 'POST',
                body: JSON.stringify({ email, code }),
              });
              setSignedIn(true);
              setError('');
            } catch (e) {
              setError(String(e));
            }
          }}
        >
          Enter workspace
        </button>
        {error && <p className="mt-4 text-sm text-accent">{error}</p>}
      </section>
    );
  return (
    <div className="grid gap-6 lg:grid-cols-[15rem_1fr]">
      <aside className="border border-line bg-ink p-5 text-canvas">
        <p className="font-display text-2xl">RFS Workspace</p>
        <nav className="mt-8 grid gap-3 text-sm text-[#b8c0ba]">
          <a href="#editor">Draft editor</a>
          <a href="#versions">Versions</a>
          <a href="#reviews">Review queue</a>
          <a href="#media">Media library</a>
          <a href="#people">People</a>
          <a href="#sessions">Sessions</a>
        </nav>
      </aside>
      <main className="border border-line bg-white p-[clamp(1.25rem,4vw,3rem)]">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <p className="text-xs font-bold uppercase tracking-widest text-accent">
            {postId ? `Draft · ${saveState}` : 'New critique'}
          </p>
          <div className="flex gap-2">
            {!postId ? (
              <button
                className="bg-ink px-4 py-2 text-sm text-white"
                onClick={async () => {
                  const p = await api('/api/v1/posts', {
                    method: 'POST',
                    body: JSON.stringify({
                      title,
                      excerpt,
                      content: editor?.getJSON(),
                    }),
                  });
                  setPostId(p.id);
                  setRevision(p.revision);
                }}
              >
                Create draft
              </button>
            ) : (
              <>
                <button
                  className="border border-line px-4 py-2 text-sm"
                  onClick={() =>
                    api(`/api/v1/posts/${postId}/checkpoint`, {
                      method: 'POST',
                      body: JSON.stringify({ reason: 'manual checkpoint' }),
                    })
                  }
                >
                  Checkpoint
                </button>
                <button
                  className="bg-accent px-4 py-2 text-sm text-white"
                  onClick={() =>
                    api(`/api/v1/posts/${postId}/publish`, {
                      method: 'POST',
                      body: '{}',
                    })
                  }
                >
                  Publish
                </button>
              </>
            )}
          </div>
        </div>
        <input
          className="mt-8 w-full border-0 border-b border-line bg-transparent pb-4 font-display text-[clamp(2.5rem,6vw,5rem)] leading-none outline-none"
          value={title}
          onChange={(e) => {
            setTitle(e.target.value);
            setSaveState('waiting');
          }}
        />
        <textarea
          className="mt-5 w-full resize-none border border-line p-3 text-muted"
          placeholder="A clear promise to the reader"
          value={excerpt}
          onChange={(e) => {
            setExcerpt(e.target.value);
            setSaveState('waiting');
          }}
        />
        <div className="mt-6 flex flex-wrap gap-2 border-y border-line py-3">
          {[
            ['Bold', () => editor?.chain().focus().toggleBold().run()],
            ['Heading', () => editor?.chain().focus().toggleHeading({ level: 2 }).run()],
            ['Quote', () => editor?.chain().focus().toggleBlockquote().run()],
            ['List', () => editor?.chain().focus().toggleBulletList().run()],
            ['Code', () => editor?.chain().focus().toggleCodeBlock().run()],
          ].map(([label, action]) => (
            <button
              key={String(label)}
              className="border border-line px-3 py-1.5 text-xs"
              onClick={action as () => void}
            >
              {String(label)}
            </button>
          ))}
        </div>
        <EditorContent
          editor={editor}
          className="article-body min-h-[28rem] py-8 outline-none [&_.tiptap]:min-h-[28rem] [&_.tiptap]:outline-none"
        />
        {error && <p className="text-sm text-accent">{error}</p>}
      </main>
    </div>
  );
}
