import { useCallback, useEffect, useState, type SyntheticEvent } from 'react';
import { EditorContent, useEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Image from '@tiptap/extension-image';

const EditorialImage = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      placement: { default: 'center' },
      caption: { default: '' },
      credit: { default: '' },
    };
  },
});

type Me = {
  identityId: string;
  kind: string;
  role: string;
  publishMode: string;
  status: string;
  displayName?: string;
  handle?: string;
  bio?: string;
  avatar?: string | null;
};
type Tab =
  | 'overview'
  | 'articles'
  | 'library'
  | 'editor'
  | 'reviews'
  | 'media'
  | 'comments'
  | 'reports'
  | 'people'
  | 'readers'
  | 'sessions'
  | 'audit'
  | 'profile';
const tabs: [Tab, string][] = [
  ['overview', 'Overview'],
  ['articles', 'Articles'],
  ['library', 'Fork library'],
  ['editor', 'Write'],
  ['reviews', 'Reviews'],
  ['media', 'Media'],
  ['comments', 'Comments'],
  ['reports', 'Reports'],
  ['people', 'Authors'],
  ['readers', 'Readers'],
  ['sessions', 'Sessions'],
  ['audit', 'Audit log'],
  ['profile', 'Profile'],
];
const cookie = (name: string) =>
  decodeURIComponent(
    document.cookie
      .split('; ')
      .find((v) => v.startsWith(name + '='))
      ?.split('=')[1] ?? '',
  );
async function call<T = any>(path: string, init: RequestInit = {}) {
  const isForm = init.body instanceof FormData;
  const response = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      ...(isForm ? {} : { 'Content-Type': 'application/json' }),
      ...(!['GET', 'HEAD'].includes(init.method ?? 'GET')
        ? { 'X-CSRF-Token': cookie('__Host-rfs_staff_csrf') || cookie('rfs_staff_csrf') }
        : {}),
      ...(init.headers ?? {}),
    },
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload.error?.message ?? 'Request failed');
  return payload.data as T;
}
const date = (v?: string | null) =>
  v
    ? new Intl.DateTimeFormat('en-RW', {
        dateStyle: 'medium',
        timeStyle: 'short',
        timeZone: 'Africa/Kigali',
      }).format(new Date(v))
    : 'Not yet';

export default function AdminApp() {
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<Tab>('overview');
  const [refresh, setRefresh] = useState(0);
  useEffect(() => {
    call<Me>('/api/v1/auth/me')
      .then((v) => v.kind === 'staff' && setMe(v))
      .catch(() => setMe(null))
      .finally(() => setLoading(false));
  }, [refresh]);
  if (loading)
    return (
      <div className="grid min-h-screen place-items-center">
        <p className="text-sm text-[#687169]">Opening the newsroom…</p>
      </div>
    );
  if (!me)
    return (
      <StaffLogin
        onDone={() => {
          setLoading(true);
          setRefresh((x) => x + 1);
        }}
      />
    );
  return (
    <Dashboard
      me={me}
      tab={tab}
      setTab={setTab}
      signOut={async () => {
        await call('/api/v1/auth/logout', { method: 'POST', body: '{}' });
        setMe(null);
      }}
    />
  );
}

function StaffLogin({ onDone }: { onDone: () => void }) {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [step, setStep] = useState<'email' | 'code'>('email');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const submitEmail = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setMessage('');
    try {
      const result = await call<{ delivery?: string }>('/api/v1/auth/staff/request-otp', {
        method: 'POST',
        body: JSON.stringify({ email }),
      });
      setStep('code');
      setMessage(
        result.delivery === 'terminal'
          ? 'Development delivery is active. Read the six-digit code from the Go API terminal.'
          : 'If this address belongs to active staff, a six-digit code has been sent. It expires in 10 minutes.',
      );
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Could not request a code');
    } finally {
      setBusy(false);
    }
  };
  const verify = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setMessage('');
    try {
      await call('/api/v1/auth/staff/verify-otp', {
        method: 'POST',
        body: JSON.stringify({ email, code }),
      });
      onDone();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'The code is invalid or expired');
    } finally {
      setBusy(false);
    }
  };
  return (
    <main className="grid min-h-screen lg:grid-cols-[1.05fr_.95fr]">
      <section className="hidden bg-[#17221d] p-16 text-[#f5f1e8] lg:flex lg:flex-col lg:justify-between">
        <a href="/" className="flex items-center gap-3 font-semibold">
          <img src="/logo.svg" className="size-11" />
          Rwanda Free Space
        </a>
        <div>
          <p className="text-xs font-bold uppercase tracking-[.2em] text-[#df6b60]">
            Private newsroom
          </p>
          <h1 className="mt-6 max-w-[9ch] font-display text-7xl leading-[.92] tracking-[-.055em]">
            Write clearly. Publish carefully.
          </h1>
          <p className="mt-8 max-w-lg text-lg leading-relaxed text-[#aeb8b1]">
            A focused place for drafts, review, evidence, revisions, and responsible publication.
          </p>
        </div>
        <p className="text-xs text-[#77837a]">Staff access only</p>
      </section>
      <section className="flex items-center justify-center px-6 py-16">
        <form onSubmit={step === 'email' ? submitEmail : verify} className="w-full max-w-md">
          <div className="mb-12 lg:hidden">
            <a href="/" className="flex items-center gap-3 font-semibold">
              <img src="/logo.svg" className="size-10" />
              Rwanda Free Space
            </a>
          </div>
          <p className="text-xs font-bold uppercase tracking-[.18em] text-accent">
            {step === 'email' ? 'Staff sign in' : 'Check your email'}
          </p>
          <h2 className="mt-4 font-display text-5xl leading-none">
            {step === 'email' ? 'Enter the newsroom.' : 'Use your sign-in code.'}
          </h2>
          <p className="mt-5 text-[#687169]">
            {step === 'email' ? (
              'No password. We will send a short-lived code to an approved staff address.'
            ) : (
              <>
                Sent for <strong className="text-ink">{email}</strong>.
              </>
            )}
          </p>
          {step === 'email' ? (
            <label className="mt-10 block text-sm font-semibold">
              Email address
              <input
                required
                autoFocus
                type="email"
                autoComplete="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="mt-2 w-full border border-[#c9c6bc] bg-white px-4 py-3.5 text-base outline-none focus:border-ink"
                placeholder="you@example.com"
              />
            </label>
          ) : (
            <label className="mt-10 block text-sm font-semibold">
              Six-digit code
              <input
                required
                autoFocus
                inputMode="numeric"
                pattern="[0-9]{6}"
                maxLength={6}
                autoComplete="one-time-code"
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
                className="mt-2 w-full border border-[#c9c6bc] bg-white px-4 py-3.5 font-mono text-2xl tracking-[.35em] outline-none focus:border-ink"
                placeholder="000000"
              />
            </label>
          )}
          <button
            disabled={busy}
            className="mt-6 w-full bg-ink px-5 py-3.5 font-semibold text-white disabled:opacity-50"
          >
            {busy ? 'Please wait…' : step === 'email' ? 'Send secure code' : 'Sign in'}
          </button>
          {step === 'code' && (
            <button
              type="button"
              onClick={() => {
                setStep('email');
                setCode('');
                setMessage('');
              }}
              className="mt-4 w-full text-sm text-[#687169] underline underline-offset-4"
            >
              Use another email
            </button>
          )}
          {message && (
            <p
              role="status"
              className="mt-6 border-l-2 border-accent pl-4 text-sm leading-relaxed text-[#687169]"
            >
              {message}
            </p>
          )}
        </form>
      </section>
    </main>
  );
}

function Dashboard({
  me,
  tab,
  setTab,
  signOut,
}: {
  me: Me;
  tab: Tab;
  setTab: (v: Tab) => void;
  signOut: () => void;
}) {
  const visible = tabs.filter(
    ([id]) =>
      me.role === 'superadmin' ||
      !['comments', 'reports', 'people', 'readers', 'audit'].includes(id),
  );
  return (
    <div className="min-h-screen xl:grid xl:grid-cols-[17rem_1fr]">
      <aside className="border-r border-[#313b35] bg-ink p-5 text-canvas xl:fixed xl:inset-y-0 xl:w-[17rem]">
        <a href="/" className="flex items-center gap-3 font-semibold">
          <img src="/logo.svg" className="size-9" />
          Rwanda Free Space
        </a>
        <div className="mt-8 border-y border-[#35413a] py-5">
          <p className="text-sm font-semibold">{me.displayName}</p>
          <p className="mt-1 text-xs capitalize text-[#929f96]">
            {me.role} · {me.publishMode.replace('_', ' ')}
          </p>
        </div>
        <nav className="mt-5 grid grid-cols-2 gap-1 xl:grid-cols-1">
          {visible.map(([id, label]) => (
            <button
              key={id}
              onClick={() => setTab(id)}
              className={`rounded-sm px-3 py-2.5 text-left text-sm ${tab === id ? 'bg-[#f5f1e8] font-semibold text-ink' : 'text-[#aeb8b1] hover:bg-[#27332c]'}`}
            >
              {label}
            </button>
          ))}
        </nav>
        <button onClick={signOut} className="mt-8 px-3 text-sm text-[#df6b60]">
          Sign out
        </button>
      </aside>
      <main className="min-w-0 p-[clamp(1rem,4vw,3.5rem)] xl:col-start-2">
        <Panel me={me} tab={tab} go={setTab} />
      </main>
    </div>
  );
}

function Panel({ me, tab, go }: { me: Me; tab: Tab; go: (v: Tab) => void }) {
  if (tab === 'editor') return <Editor me={me} />;
  if (tab === 'people') return <PeoplePanel />;
  if (tab === 'media') return <MediaPanel />;
  const paths: Record<Exclude<Tab, 'editor' | 'profile'>, string> = {
    overview: '/api/v1/admin/overview',
    articles: '/api/v1/admin/posts',
    library: '/api/v1/public/posts?limit=100',
    reviews: '/api/v1/admin/reviews',
    media: '/api/v1/admin/media',
    comments: '/api/v1/admin/comments',
    reports: '/api/v1/admin/reports',
    people: '/api/v1/admin/staff',
    readers: '/api/v1/admin/readers',
    sessions: '/api/v1/sessions',
    audit: '/api/v1/admin/audit',
  };
  if (tab === 'profile') return <Profile me={me} />;
  return <DataPanel me={me} tab={tab} path={paths[tab]} go={go} />;
}

function DataPanel({
  me,
  tab,
  path,
  go,
}: {
  me: Me;
  tab: Exclude<Tab, 'editor' | 'profile'>;
  path: string;
  go: (v: Tab) => void;
}) {
  const [data, setData] = useState<any>(null);
  const [error, setError] = useState('');
  const load = useCallback(() => {
    setError('');
    call(path)
      .then(setData)
      .catch((e) => setError(e.message));
  }, [path]);
  useEffect(load, [load]);
  const title = tabs.find((x) => x[0] === tab)?.[1];
  if (error) return <Empty title={title ?? 'Dashboard'} text={error} />;
  if (data === null) return <Empty title={title ?? 'Dashboard'} text="Loading…" />;
  if (tab === 'overview')
    return (
      <>
        <Heading
          eyebrow="Newsroom"
          title={`Good ${new Date().getHours() < 12 ? 'morning' : 'day'}, ${me.displayName?.split(' ')[0] ?? 'editor'}.`}
          action={
            <button
              onClick={() => go('editor')}
              className="bg-accent px-4 py-2.5 text-sm font-semibold text-white"
            >
              New critique
            </button>
          }
        />
        <div className="mt-10 grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
          {Object.entries(data).map(([key, value]) => (
            <div className="border border-line bg-white p-6" key={key}>
              <p className="text-xs font-bold uppercase tracking-widest text-muted">{key}</p>
              <p className="mt-5 font-display text-5xl">{String(value)}</p>
            </div>
          ))}
        </div>
        <div className="mt-8 grid gap-6 lg:grid-cols-2">
          <section className="border border-line bg-white p-7">
            <h2 className="font-display text-3xl">Editorial rhythm</h2>
            <p className="mt-3 text-muted">
              Draft deliberately, create checkpoints before substantial edits, and publish only the
              exact version you reviewed.
            </p>
          </section>
          <section className="border border-line bg-ink p-7 text-canvas">
            <h2 className="font-display text-3xl">Your publishing mode</h2>
            <p className="mt-3 text-[#aeb8b1]">
              {me.publishMode === 'direct_publish'
                ? 'You can publish directly. Every publication still creates an immutable version.'
                : 'Your work must be approved by a superadmin before publication.'}
            </p>
          </section>
        </div>
      </>
    );
  const rows = Array.isArray(data) ? data : [];
  return (
    <>
      <Heading
        eyebrow="Administration"
        title={title ?? 'Dashboard'}
        action={
          tab === 'articles' ? (
            <button
              onClick={() => go('editor')}
              className="bg-ink px-4 py-2.5 text-sm font-semibold text-white"
            >
              New article
            </button>
          ) : tab === 'sessions' ? (
            <button
              onClick={async () => {
                await call('/api/v1/sessions', { method: 'DELETE', body: '{}' });
                load();
              }}
              className="border border-ink px-4 py-2.5 text-sm font-semibold"
            >
              Revoke all other sessions
            </button>
          ) : undefined
        }
      />
      {rows.length === 0 ? (
        <div className="mt-10 border border-dashed border-line bg-white p-14 text-center text-muted">
          Nothing needs attention here.
        </div>
      ) : (
        <div className="mt-8 overflow-x-auto border border-line bg-white">
          <table className="w-full min-w-[45rem] border-collapse text-left text-sm">
            <thead className="bg-[#e2dfd6] text-xs uppercase tracking-wider text-muted">
              <tr>
                {Object.keys(rows[0])
                  .slice(0, 6)
                  .map((k) => (
                    <th className="px-4 py-3" key={k}>
                      {k.replace(/([A-Z])/g, ' $1')}
                    </th>
                  ))}
                <th className="px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row: any) => (
                <tr key={row.id} className="border-t border-line">
                  {Object.keys(row)
                    .slice(0, 6)
                    .map((k) => (
                      <td className="max-w-xs truncate px-4 py-3" key={k}>
                        {k.toLowerCase().includes('at')
                          ? date(row[k])
                          : typeof row[k] === 'boolean'
                            ? row[k]
                              ? 'Yes'
                              : 'No'
                            : String(row[k] ?? '—')}
                      </td>
                    ))}
                  <td className="whitespace-nowrap px-4 py-3">
                    {tab === 'articles' && (
                      <>
                        <button
                          onClick={() => {
                            sessionStorage.setItem('rfs-edit-post', row.id);
                            go('editor');
                          }}
                          className="mr-3 font-semibold text-accent"
                        >
                          Edit
                        </button>
                        {row.state === 'published' && (
                          <a
                            href={`/critiques/${row.slug}/`}
                            target="_blank"
                            className="mr-3 font-semibold"
                          >
                            Preview
                          </a>
                        )}
                        <button
                          onClick={async () => {
                            if (!confirm(`Create an independent fork of “${row.title}”?`)) return;
                            const fork = await call<any>(`/api/v1/posts/${row.id}/fork`, {
                              method: 'POST',
                              body: '{}',
                            });
                            sessionStorage.setItem('rfs-edit-post', fork.id);
                            go('editor');
                          }}
                          className="mr-3 font-semibold"
                        >
                          Fork
                        </button>
                        <button
                          onClick={async () => {
                            const typed = prompt(
                              `Type the exact title to permanently delete this post:\n${row.title}`,
                            );
                            if (
                              typed !== row.title ||
                              !confirm(
                                'This permanently deletes the post, versions, comments, bookmarks, and references. Continue?',
                              )
                            )
                              return;
                            await call(`/api/v1/posts/${row.id}`, {
                              method: 'DELETE',
                              body: JSON.stringify({
                                title: typed,
                                confirmation: 'permanently delete',
                                reason: 'Deleted from editorial workspace',
                              }),
                            });
                            load();
                          }}
                          className="font-semibold text-red-700"
                        >
                          Delete
                        </button>
                      </>
                    )}
                    {tab === 'library' && (
                      <>
                        <a
                          href={`/critiques/${row.slug}/`}
                          target="_blank"
                          className="mr-3 font-semibold"
                        >
                          Read
                        </a>
                        <button
                          onClick={async () => {
                            const fork = await call<any>(`/api/v1/posts/${row.id}/fork`, {
                              method: 'POST',
                              body: '{}',
                            });
                            sessionStorage.setItem('rfs-edit-post', fork.id);
                            go('editor');
                          }}
                          className="font-semibold text-accent"
                        >
                          Fork into draft
                        </button>
                      </>
                    )}
                    {tab === 'reviews' && me.role === 'superadmin' && (
                      <>
                        <button
                          onClick={async () => {
                            await call(`/api/v1/reviews/${row.id}/approve`, {
                              method: 'POST',
                              body: '{}',
                            });
                            load();
                          }}
                          className="mr-3 font-semibold text-[#207044]"
                        >
                          Approve
                        </button>
                        <button
                          onClick={async () => {
                            const reason = prompt('Short rejection reason') ?? '';
                            await call(`/api/v1/reviews/${row.id}/reject`, {
                              method: 'POST',
                              body: JSON.stringify({ reason }),
                            });
                            load();
                          }}
                          className="font-semibold text-accent"
                        >
                          Reject
                        </button>
                      </>
                    )}
                    {tab === 'comments' && (
                      <>
                        <button
                          onClick={async () => {
                            await call(`/api/v1/admin/comments/${row.id}/approve`, {
                              method: 'POST',
                              body: '{}',
                            });
                            load();
                          }}
                          className="mr-3 font-semibold text-[#207044]"
                        >
                          Approve
                        </button>
                        <button
                          onClick={async () => {
                            await call(`/api/v1/admin/comments/${row.id}/reject`, {
                              method: 'POST',
                              body: '{}',
                            });
                            load();
                          }}
                          className="font-semibold text-accent"
                        >
                          Reject
                        </button>
                      </>
                    )}
                    {tab === 'sessions' && (
                      <button
                        onClick={async () => {
                          await call(`/api/v1/sessions/${row.id}`, {
                            method: 'DELETE',
                            body: '{}',
                          });
                          load();
                        }}
                        className="font-semibold text-accent"
                      >
                        Revoke
                      </button>
                    )}
                    {tab === 'reports' && (
                      <>
                        <button
                          onClick={async () => {
                            await call(`/api/v1/admin/comments/${row.commentId}/hide`, {
                              method: 'POST',
                              body: '{}',
                            });
                            await call(`/api/v1/admin/reports/${row.id}/resolve`, {
                              method: 'POST',
                              body: '{}',
                            });
                            load();
                          }}
                          className="mr-3 font-semibold text-accent"
                        >
                          Hide comment
                        </button>
                        <button
                          onClick={async () => {
                            await call(`/api/v1/admin/reports/${row.id}/resolve`, {
                              method: 'POST',
                              body: '{}',
                            });
                            load();
                          }}
                          className="font-semibold text-[#207044]"
                        >
                          Dismiss
                        </button>
                      </>
                    )}
                    {tab === 'readers' && (
                      <select
                        aria-label={`Status for ${row.username}`}
                        value={row.status}
                        onChange={async (e) => {
                          const status = e.target.value;
                          const reason =
                            status === 'active' ? '' : (prompt('Moderation reason') ?? '');
                          const durationDays =
                            status === 'suspended'
                              ? Number(prompt('Suspend for how many days?', '7') ?? '7')
                              : 0;
                          await call(`/api/v1/admin/readers/${row.id}`, {
                            method: 'PATCH',
                            body: JSON.stringify({ status, reason, durationDays }),
                          });
                          load();
                        }}
                        className="border border-line px-2 py-1.5"
                      >
                        <option value="active">Active</option>
                        <option value="suspended">Suspended</option>
                        <option value="banned">Banned</option>
                      </select>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}

function Editor({ me }: { me: Me }) {
  const [id, setId] = useState('');
  const [title, setTitle] = useState('Untitled critique');
  const [excerpt, setExcerpt] = useState('');
  const [revision, setRevision] = useState(1);
  const [state, setState] = useState('Not saved');
  const [error, setError] = useState('');
  const [versions, setVersions] = useState<any[]>([]);
  const [compare, setCompare] = useState<string[]>([]);
  const [categories, setCategories] = useState<any[]>([]),
    [tags, setTags] = useState<any[]>([]),
    [category, setCategory] = useState(''),
    [selectedTags, setSelectedTags] = useState<string[]>([]);
  const loadVersions = useCallback(
    (postId: string) =>
      call<any[]>(`/api/v1/posts/${postId}/versions`)
        .then(setVersions)
        .catch(() => {}),
    [],
  );
  const editor = useEditor({
    extensions: [StarterKit, EditorialImage.configure({ allowBase64: false })],
    content: '<p>Begin with the evidence. Explain the friction and propose a practical fix.</p>',
    immediatelyRender: false,
    onUpdate: () => setState(navigator.onLine ? 'Waiting to save' : 'Offline'),
  });
  useEffect(() => {
    call<any[]>('/api/v1/categories').then(setCategories);
    call<any[]>('/api/v1/tags').then(setTags);
  }, []);
  useEffect(() => {
    const editing = sessionStorage.getItem('rfs-edit-post');
    if (!editing) return;
    sessionStorage.removeItem('rfs-edit-post');
    call<any>(`/api/v1/posts/${editing}/draft`)
      .then((p) => {
        setId(p.id);
        setTitle(p.title);
        setExcerpt(p.excerpt);
        setRevision(p.revision);
        setCategory(p.categoryId ?? '');
        setSelectedTags(p.tagIds ?? []);
        editor?.commands.setContent(p.content);
        setState('Saved');
        loadVersions(p.id);
      })
      .catch((e) => setError(e.message));
  }, [editor, loadVersions]);
  useEffect(() => {
    if (!id || state !== 'Waiting to save') return;
    const timer = setTimeout(async () => {
      setState('Saving…');
      try {
        const p = await call<any>(`/api/v1/posts/${id}/draft`, {
          method: 'PUT',
          body: JSON.stringify({
            title,
            excerpt,
            content: editor?.getJSON(),
            revision,
          }),
        });
        setRevision(p.revision);
        setState('Saved');
      } catch (e) {
        setState('Retrying');
        setError(e instanceof Error ? e.message : 'Save failed');
      }
    }, 2000);
    return () => clearTimeout(timer);
  }, [id, state, title, excerpt, editor, revision]);
  const create = async () => {
    try {
      const p = await call<any>('/api/v1/posts', {
        method: 'POST',
        body: JSON.stringify({ title, excerpt, content: editor?.getJSON() }),
      });
      setId(p.id);
      setRevision(p.revision);
      setState('Saved');
      loadVersions(p.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Create failed');
    }
  };
  const uploadImage = async (file: File) => {
    if (!id) {
      setError('Create the draft before adding images.');
      return;
    }
    const alt = prompt('Alternative text describing this image');
    if (!alt) return;
    const caption = prompt('Optional caption') ?? '';
    const credit = prompt('Optional credit') ?? '';
    const placement =
      prompt('Placement: center, wide, full, left, right, or thumbnail', 'center') ?? 'center';
    if (!['center', 'wide', 'full', 'left', 'right', 'thumbnail'].includes(placement)) {
      setError('Choose a valid image placement.');
      return;
    }
    const form = new FormData();
    form.set('file', file);
    form.set('alt', alt);
    form.set('caption', caption);
    form.set('credit', credit);
    try {
      const asset = await call<any>('/api/v1/media', {
        method: 'POST',
        body: form,
      });
      await call(`/api/v1/posts/${id}/media/${asset.id}`, {
        method: 'POST',
        body: JSON.stringify({ placement }),
      });
      if (placement !== 'thumbnail')
        editor
          ?.chain()
          .focus()
          .setImage({ src: asset.src, alt, title: caption, placement, caption, credit } as any)
          .run();
      setState('Waiting to save');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Image upload failed');
    }
  };
  return (
    <>
      <Heading eyebrow={id ? state : 'New draft'} title="Write a critique" />
      {compare.length === 2 && (
        <section className="mt-8 grid gap-4 border border-line bg-white p-5 md:grid-cols-2">
          {compare.map((versionId) => {
            const version = versions.find((item) => item.id === versionId);
            return (
              <article key={versionId}>
                <p className="text-xs font-bold uppercase tracking-widest text-accent">
                  Version {version?.number}
                </p>
                <h2 className="mt-2 font-display text-3xl">{version?.title}</h2>
                <p className="mt-3 text-sm text-muted">{version?.excerpt}</p>
                <pre className="mt-4 max-h-72 overflow-auto whitespace-pre-wrap bg-paper p-3 text-xs">
                  {JSON.stringify(version?.content, null, 2)}
                </pre>
              </article>
            );
          })}
        </section>
      )}
      <div className="mt-8 grid gap-6 xl:grid-cols-[1fr_18rem]">
        <section className="border border-line bg-white p-[clamp(1.25rem,4vw,3rem)]">
          <input
            value={title}
            onChange={(e) => {
              setTitle(e.target.value);
              if (id) setState('Waiting to save');
            }}
            className="w-full border-0 border-b border-line pb-4 font-display text-[clamp(2.5rem,6vw,5rem)] leading-none outline-none"
          />
          <textarea
            value={excerpt}
            onChange={(e) => {
              setExcerpt(e.target.value);
              if (id) setState('Waiting to save');
            }}
            placeholder="A precise promise to the reader"
            className="mt-5 w-full border border-line p-3"
          />
          <div className="mt-5 flex flex-wrap gap-2 border-y border-line py-3">
            {[
              ['Bold', () => editor?.chain().focus().toggleBold().run()],
              ['H2', () => editor?.chain().focus().toggleHeading({ level: 2 }).run()],
              ['Quote', () => editor?.chain().focus().toggleBlockquote().run()],
              ['List', () => editor?.chain().focus().toggleBulletList().run()],
              ['Code', () => editor?.chain().focus().toggleCodeBlock().run()],
            ].map(([label, fn]) => (
              <button
                key={String(label)}
                onClick={fn as () => void}
                className="border border-line px-3 py-1.5 text-xs"
              >
                {String(label)}
              </button>
            ))}
            <label className="cursor-pointer border border-line px-3 py-1.5 text-xs">
              Image
              <input
                type="file"
                accept="image/jpeg,image/png"
                className="sr-only"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) uploadImage(file);
                  e.currentTarget.value = '';
                }}
              />
            </label>
          </div>
          <EditorContent
            editor={editor}
            className="article-body min-h-[32rem] py-8 [&_.tiptap]:min-h-[32rem] [&_.tiptap]:outline-none"
          />
        </section>
        <aside className="space-y-4">
          <div className="border border-line bg-white p-5">
            <p className="text-xs font-bold uppercase tracking-widest text-muted">Publication</p>
            {!id ? (
              <button
                onClick={create}
                className="mt-5 w-full bg-ink px-4 py-3 text-sm font-semibold text-white"
              >
                Create draft
              </button>
            ) : (
              <>
                <button
                  onClick={() =>
                    call(`/api/v1/posts/${id}/checkpoint`, {
                      method: 'POST',
                      body: JSON.stringify({ reason: 'manual checkpoint' }),
                    }).then(() => {
                      setState('Checkpoint created');
                      loadVersions(id);
                    })
                  }
                  className="mt-5 w-full border border-line px-4 py-3 text-sm font-semibold"
                >
                  Create checkpoint
                </button>
                <button
                  onClick={() =>
                    call(
                      `/api/v1/posts/${id}/${me.publishMode === 'review_required' ? 'submit' : 'publish'}`,
                      { method: 'POST', body: '{}' },
                    ).then(() =>
                      setState(
                        me.publishMode === 'review_required' ? 'Submitted for review' : 'Published',
                      ),
                    )
                  }
                  className="mt-3 w-full bg-accent px-4 py-3 text-sm font-semibold text-white"
                >
                  {me.publishMode === 'review_required' ? 'Submit for review' : 'Publish now'}
                </button>
              </>
            )}
          </div>
          {id && (
            <div className="border border-line bg-white p-5">
              <p className="text-xs font-bold uppercase tracking-widest text-muted">
                Classification
              </p>
              <label className="mt-4 block text-xs font-semibold">
                Category
                <select
                  value={category}
                  onChange={(e) => setCategory(e.target.value)}
                  className="mt-2 w-full border border-line px-2 py-2"
                >
                  <option value="">Uncategorized</option>
                  {categories.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
              </label>
              {me.role === 'superadmin' && (
                <button
                  onClick={async () => {
                    const name = prompt('New category name');
                    if (!name) return;
                    const description = prompt('Short category description') ?? '';
                    await call('/api/v1/categories', {
                      method: 'POST',
                      body: JSON.stringify({ name, description }),
                    });
                    setCategories(await call<any[]>('/api/v1/categories'));
                  }}
                  className="mt-2 text-xs font-semibold text-accent"
                >
                  Add category
                </button>
              )}
              <fieldset className="mt-4">
                <legend className="text-xs font-semibold">Tags</legend>
                <div className="mt-2 grid gap-2">
                  {tags.map((t) => (
                    <label key={t.id} className="text-xs">
                      <input
                        type="checkbox"
                        checked={selectedTags.includes(t.id)}
                        onChange={(e) =>
                          setSelectedTags((v) =>
                            e.target.checked ? [...v, t.id] : v.filter((x) => x !== t.id),
                          )
                        }
                        className="mr-2"
                      />
                      {t.name}
                    </label>
                  ))}
                </div>
              </fieldset>
              {me.role === 'superadmin' && (
                <button
                  onClick={async () => {
                    const name = prompt('New tag name');
                    if (!name) return;
                    await call('/api/v1/tags', { method: 'POST', body: JSON.stringify({ name }) });
                    setTags(await call<any[]>('/api/v1/tags'));
                  }}
                  className="mt-2 text-xs font-semibold text-accent"
                >
                  Add tag
                </button>
              )}
              <button
                onClick={() =>
                  call(`/api/v1/posts/${id}/metadata`, {
                    method: 'PATCH',
                    body: JSON.stringify({
                      categoryId: category,
                      tagIds: selectedTags,
                    }),
                  }).then(() => setState('Metadata saved'))
                }
                className="mt-4 w-full border border-line px-3 py-2 text-xs font-semibold"
              >
                Save classification
              </button>
            </div>
          )}
          <div className="border border-line bg-white p-5">
            <p className="text-xs font-bold uppercase tracking-widest text-muted">Autosave</p>
            <p className="mt-3 text-sm">{state}</p>
            <p className="mt-1 text-xs text-muted">Revision {revision}</p>
          </div>
          {id && (
            <div className="border border-line bg-white p-5">
              <p className="text-xs font-bold uppercase tracking-widest text-muted">
                Version history
              </p>
              <div className="mt-3 grid gap-2">
                {versions.length === 0 ? (
                  <p className="text-xs text-muted">No checkpoints yet.</p>
                ) : (
                  versions.slice(0, 8).map((v) => (
                    <div key={v.id} className="border-t border-line pt-2 text-left text-xs">
                      <label className="mb-2 flex items-center gap-2">
                        <input
                          type="checkbox"
                          checked={compare.includes(v.id)}
                          onChange={(event) =>
                            setCompare((current) =>
                              event.target.checked
                                ? [...current.filter((id) => id !== v.id), v.id].slice(-2)
                                : current.filter((id) => id !== v.id),
                            )
                          }
                        />{' '}
                        Compare
                      </label>
                      <button
                        onClick={async () => {
                          if (
                            !confirm(
                              `Restore version ${v.number}? A new checkpoint will preserve this restore.`,
                            )
                          )
                            return;
                          const p = await call<any>(`/api/v1/posts/${id}/restore/${v.id}`, {
                            method: 'POST',
                            body: '{}',
                          });
                          setTitle(p.title);
                          setExcerpt(p.excerpt);
                          setRevision(p.revision);
                          editor?.commands.setContent(p.content);
                          loadVersions(id);
                          setState('Version restored');
                        }}
                        className="text-left"
                      >
                        <strong>v{v.number}</strong> · {v.reason}
                        <br />
                        <span className="text-muted">{date(v.createdAt)}</span>
                      </button>
                    </div>
                  ))
                )}
              </div>
            </div>
          )}
          {error && <p className="border-l-2 border-accent pl-3 text-sm text-accent">{error}</p>}
        </aside>
      </div>
    </>
  );
}

function PeoplePanel() {
  const [rows, setRows] = useState<any[]>([]);
  const [error, setError] = useState('');
  const [email, setEmail] = useState('');
  const [handle, setHandle] = useState('');
  const [name, setName] = useState('');
  const [mode, setMode] = useState('review_required');
  const load = () =>
    call<any[]>('/api/v1/admin/staff')
      .then(setRows)
      .catch((e) => setError(e.message));
  useEffect(() => {
    void load();
  }, []);
  const add = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    try {
      await call('/api/v1/admin/staff', {
        method: 'POST',
        body: JSON.stringify({
          email,
          handle,
          displayName: name,
          role: 'author',
          publishMode: mode,
        }),
      });
      setEmail('');
      setHandle('');
      setName('');
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not add author');
    }
  };
  const change = async (row: any, patch: any) => {
    try {
      await call(`/api/v1/admin/staff/${row.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          role: patch.role ?? row.role,
          publishMode: patch.publishMode ?? row.publishMode,
          status: patch.status ?? row.status,
          reassignTo: patch.reassignTo ?? '',
        }),
      });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Update failed');
    }
  };
  return (
    <>
      <Heading eyebrow="Superadmin" title="Authors and access" />
      <form
        onSubmit={add}
        className="mt-8 grid gap-3 border border-line bg-white p-6 md:grid-cols-2 xl:grid-cols-5"
      >
        <input
          required
          type="email"
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="border border-line px-3 py-2.5"
        />
        <input
          required
          placeholder="Handle"
          value={handle}
          onChange={(e) => setHandle(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ''))}
          className="border border-line px-3 py-2.5"
        />
        <input
          required
          placeholder="Display name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="border border-line px-3 py-2.5"
        />
        <select
          value={mode}
          onChange={(e) => setMode(e.target.value)}
          className="border border-line px-3 py-2.5"
        >
          <option value="review_required">Review required</option>
          <option value="direct_publish">Direct publish</option>
        </select>
        <button className="bg-ink px-4 py-2.5 font-semibold text-white">Add author</button>
      </form>
      {error && <p className="mt-4 text-accent">{error}</p>}
      <div className="mt-6 grid gap-4">
        {rows.map((row) => (
          <article
            key={row.id}
            className="grid gap-4 border border-line bg-white p-5 lg:grid-cols-[1fr_auto_auto_auto] lg:items-center"
          >
            <div>
              <h2 className="font-semibold">
                {row.displayName} <span className="font-normal text-muted">@{row.handle}</span>
              </h2>
              <p className="mt-1 text-sm text-muted">{row.email}</p>
            </div>
            <select
              aria-label={`Role for ${row.displayName}`}
              value={row.role}
              onChange={(e) => change(row, { role: e.target.value })}
              className="border border-line px-3 py-2"
            >
              <option value="author">Author</option>
              <option value="superadmin">Superadmin</option>
            </select>
            <select
              aria-label={`Publishing mode for ${row.displayName}`}
              value={row.publishMode}
              onChange={(e) => change(row, { publishMode: e.target.value })}
              className="border border-line px-3 py-2"
            >
              <option value="review_required">Review required</option>
              <option value="direct_publish">Direct publish</option>
            </select>
            <button
              onClick={() => {
                const status = row.status === 'active' ? 'inactive' : 'active';
                let reassignTo = '';
                if (
                  status === 'inactive' &&
                  confirm(
                    'Reassign this author’s unpublished drafts to another active staff member?',
                  )
                )
                  reassignTo =
                    prompt(
                      `Enter the destination identity ID:\n${rows
                        .filter(
                          (candidate) => candidate.id !== row.id && candidate.status === 'active',
                        )
                        .map((candidate) => `${candidate.displayName}: ${candidate.id}`)
                        .join('\n')}`,
                    ) ?? '';
                change(row, { status, reassignTo });
              }}
              className={`px-4 py-2 text-sm font-semibold ${row.status === 'active' ? 'border border-accent text-accent' : 'bg-[#207044] text-white'}`}
            >
              {row.status === 'active' ? 'Deactivate' : 'Reactivate'}
            </button>
          </article>
        ))}
      </div>
    </>
  );
}

function MediaPanel() {
  const [rows, setRows] = useState<any[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const load = () =>
    call<any[]>('/api/v1/admin/media')
      .then(setRows)
      .catch((e) => setError(e.message));
  useEffect(() => {
    void load();
  }, []);
  const upload = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setError('');
    try {
      const form = new FormData(e.currentTarget);
      await call('/api/v1/media', { method: 'POST', body: form });
      e.currentTarget.reset();
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Upload failed');
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Heading eyebrow="Library" title="Images and evidence" />
      <form
        onSubmit={upload}
        className="mt-8 grid gap-4 border border-line bg-white p-6 lg:grid-cols-2"
      >
        <label className="text-sm font-semibold">
          JPEG or PNG
          <input
            required
            name="file"
            type="file"
            accept="image/jpeg,image/png"
            className="mt-2 block w-full text-sm"
          />
        </label>
        <label className="text-sm font-semibold">
          Caption
          <input name="caption" className="mt-2 block w-full border border-line px-3 py-2" />
        </label>
        <label className="text-sm font-semibold">
          Credit
          <input name="credit" className="mt-2 block w-full border border-line px-3 py-2" />
        </label>
        <label className="text-sm font-semibold">
          Focal point X (0 to 1)
          <input
            name="focalX"
            type="number"
            min="0"
            max="1"
            step="0.01"
            className="mt-2 block w-full border border-line px-3 py-2"
          />
        </label>
        <label className="text-sm font-semibold">
          Focal point Y (0 to 1)
          <input
            name="focalY"
            type="number"
            min="0"
            max="1"
            step="0.01"
            className="mt-2 block w-full border border-line px-3 py-2"
          />
        </label>
        <label className="text-sm font-semibold">
          Alternative text
          <input
            required
            name="alt"
            className="mt-2 block w-full border border-line px-3 py-2"
            placeholder="Describe what the image shows"
          />
        </label>
        <button disabled={busy} className="self-end bg-ink px-5 py-2.5 font-semibold text-white">
          {busy ? 'Processing…' : 'Upload image'}
        </button>
      </form>
      {error && <p className="mt-4 text-accent">{error}</p>}
      <div className="mt-8 grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
        {rows.map((row) => (
          <article className="overflow-hidden border border-line bg-white" key={row.id}>
            <div className="grid aspect-[16/9] place-items-center bg-surface">
              {row.status === 'public' ? (
                <img src={row.src} alt={row.alt} className="size-full object-cover" />
              ) : (
                <span className="text-xs uppercase tracking-widest text-muted">Draft media</span>
              )}
            </div>
            <div className="p-4">
              <p className="font-semibold">{row.alt}</p>
              <p className="mt-2 text-xs text-muted">
                {row.width} × {row.height} · {Math.ceil(row.bytes / 1024)} KB · {row.status}
              </p>
              <button
                onClick={() => navigator.clipboard.writeText(row.id)}
                className="mt-3 text-xs font-semibold text-accent"
              >
                Copy asset ID
              </button>
            </div>
          </article>
        ))}
      </div>
    </>
  );
}

function Profile({ me }: { me: Me }) {
  const [name, setName] = useState(me.displayName ?? '');
  const [handle, setHandle] = useState(me.handle ?? '');
  const [bio, setBio] = useState(me.bio ?? '');
  const [avatarAssetId, setAvatarAssetId] = useState('');
  const [avatarPreview, setAvatarPreview] = useState(me.avatar ?? '');
  const [message, setMessage] = useState('');
  const save = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    setMessage('');
    try {
      await call('/api/v1/account/profile', {
        method: 'PATCH',
        body: JSON.stringify({ displayName: name, handle, bio, avatarAssetId }),
      });
      setMessage('Profile saved. Previous handles continue to redirect.');
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Save failed');
    }
  };
  return (
    <>
      <Heading eyebrow="Account" title="Profile settings" />
      <form onSubmit={save} className="mt-8 max-w-2xl border border-line bg-white p-8">
        <label className="mb-6 flex cursor-pointer items-center gap-4 border border-line p-4 text-sm font-semibold">
          {avatarPreview ? (
            <img
              src={avatarPreview}
              alt="Current profile"
              className="size-16 rounded-full object-cover"
            />
          ) : (
            <span className="grid size-16 place-items-center rounded-full bg-paper">Photo</span>
          )}
          <span>
            Upload profile image
            <input
              type="file"
              accept="image/jpeg,image/png"
              className="sr-only"
              onChange={async (event) => {
                const file = event.target.files?.[0];
                if (!file) return;
                const form = new FormData();
                form.set('file', file);
                form.set('alt', `${name} profile image`);
                const asset = await call<any>('/api/v1/media', { method: 'POST', body: form });
                setAvatarAssetId(asset.id);
                setAvatarPreview(asset.src);
                setMessage('Image ready. Save the profile to use it.');
              }}
            />
          </span>
        </label>
        <div className="grid gap-5 sm:grid-cols-2">
          <label className="text-sm font-semibold">
            Display name
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="mt-2 w-full border border-line px-3 py-2.5"
            />
          </label>
          <label className="text-sm font-semibold">
            Handle
            <input
              required
              value={handle}
              onChange={(e) => setHandle(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ''))}
              className="mt-2 w-full border border-line px-3 py-2.5"
            />
          </label>
          <label className="text-sm font-semibold sm:col-span-2">
            Biography
            <textarea
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              className="mt-2 min-h-28 w-full border border-line px-3 py-2.5"
            />
          </label>
        </div>
        <div className="mt-6 flex items-center gap-4">
          <button className="bg-ink px-5 py-2.5 font-semibold text-white">Save profile</button>
          <span className="text-sm text-muted">{message}</span>
        </div>
        <p className="mt-8 border-t border-line pt-5 text-xs capitalize text-muted">
          {me.role} · {me.publishMode.replace('_', ' ')}
        </p>
      </form>
    </>
  );
}
function Heading({
  eyebrow,
  title,
  action,
}: {
  eyebrow: string;
  title: string;
  action?: React.ReactNode;
}) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-5 border-b border-line pb-7">
      <div>
        <p className="text-xs font-bold uppercase tracking-[.18em] text-accent">{eyebrow}</p>
        <h1 className="mt-3 font-display text-[clamp(2.5rem,5vw,4.8rem)] leading-none tracking-[-.045em]">
          {title}
        </h1>
      </div>
      {action}
    </header>
  );
}
function Empty({ title, text }: { title: string; text: string }) {
  return (
    <>
      <Heading eyebrow="Newsroom" title={title} />
      <div className="mt-8 border border-line bg-white p-12 text-center text-muted">{text}</div>
    </>
  );
}
