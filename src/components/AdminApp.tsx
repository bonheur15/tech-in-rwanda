import Image from '@tiptap/extension-image';
import { EditorContent, useEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import { type SyntheticEvent, useCallback, useEffect, useMemo, useState } from 'react';

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
  ['articles', 'Articles'],
  ['editor', 'New article'],
  ['reviews', 'Reviews'],
  ['library', 'Fork library'],
  ['media', 'Media'],
  ['comments', 'Comments'],
  ['reports', 'Reports'],
  ['people', 'Authors'],
  ['readers', 'Readers'],
  ['sessions', 'Sessions'],
  ['audit', 'Audit log'],
  ['profile', 'Profile'],
];
const routeFor = (tab: Tab) => `/admin/${tab === 'editor' ? 'write' : tab}`;
const tabFromPath = (path: string): Tab => {
  if (/^\/admin\/articles\/[^/]+\/edit\/?$/.test(path)) return 'editor';
  const segment = path.replace(/^\/admin\/?/, '').split('/')[0];
  if (!segment) return 'articles';
  if (segment === 'write') return 'editor';
  return tabs.some(([id]) => id === segment) ? (segment as Tab) : 'articles';
};
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
  const [tab, setTab] = useState<Tab>(() =>
    typeof window === 'undefined' ? 'articles' : tabFromPath(window.location.pathname),
  );
  const [theme, setTheme] = useState<'light' | 'dark'>(() =>
    typeof window !== 'undefined' && localStorage.getItem('rfs-admin-theme') === 'light'
      ? 'light'
      : 'dark',
  );
  const [refresh, setRefresh] = useState(0);
  useEffect(() => {
    call<Me>('/api/v1/auth/me')
      .then((v) => v.kind === 'staff' && setMe(v))
      .catch(() => setMe(null))
      .finally(() => setLoading(false));
  }, [refresh]);
  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark');
    document.documentElement.style.colorScheme = theme;
    localStorage.setItem('rfs-admin-theme', theme);
  }, [theme]);
  useEffect(() => {
    const onPopState = () => setTab(tabFromPath(window.location.pathname));
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);
  const navigate = useCallback((next: Tab) => {
    const route = routeFor(next);
    if (window.location.pathname !== route) window.history.pushState({}, '', route);
    setTab(next);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }, []);
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
      setTab={navigate}
      theme={theme}
      setTheme={setTheme}
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
            className="admin-neutral-action mt-6 w-full justify-center px-5 py-3.5 font-semibold disabled:opacity-50"
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
  theme,
  setTheme,
  signOut,
}: {
  me: Me;
  tab: Tab;
  setTab: (v: Tab) => void;
  theme: 'light' | 'dark';
  setTheme: (v: 'light' | 'dark') => void;
  signOut: () => void;
}) {
  const visible = tabs.filter(
    ([id]) =>
      id !== 'editor' &&
      (me.role === 'superadmin' ||
        !['comments', 'reports', 'people', 'readers', 'audit'].includes(id)),
  );
  return (
    <div className="admin-shell min-h-screen lg:grid lg:grid-cols-[15.5rem_1fr]">
      <aside className="admin-sidebar m-3 rounded-[1.4rem] p-3 text-white shadow-2xl shadow-black/20 lg:fixed lg:inset-y-0 lg:w-[15.5rem]">
        <a href="/" className="flex items-center gap-3 px-2 py-2 text-sm font-semibold">
          <img src="/logo.svg" className="size-8" alt="" />
          <span>Rwanda Free Space</span>
        </a>
        <button
          onClick={() => setTab('editor')}
          className="sidebar-create-action mt-5 flex w-full items-center justify-center gap-2 rounded-xl px-3 py-3 text-sm font-bold shadow-sm transition hover:-translate-y-0.5 hover:shadow-md"
        >
          <Icon name="plus" /> New article
        </button>
        <p className="mb-2 mt-6 px-3 text-[.65rem] font-bold uppercase tracking-[.16em] text-white/40">
          Publishing
        </p>
        <nav className="grid grid-cols-2 gap-1 lg:grid-cols-1">
          {visible.map(([id, label]) => (
            <button
              key={id}
              onClick={() => setTab(id)}
              className={`flex items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm transition ${tab === id ? 'bg-white/12 font-semibold text-white shadow-inner' : 'text-white/60 hover:bg-white/7 hover:text-white'}`}
            >
              <Icon name={id} /> {label}
            </button>
          ))}
        </nav>
        <div className="mt-6 border-t border-white/10 pt-3 lg:absolute lg:inset-x-3 lg:bottom-3">
          <button
            onClick={() => setTab('profile')}
            className="flex w-full items-center gap-3 rounded-xl p-2 text-left hover:bg-white/7"
          >
            <span className="grid size-8 place-items-center rounded-lg bg-white/10 text-xs font-bold">
              {me.displayName?.slice(0, 2).toUpperCase()}
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-xs font-semibold">{me.displayName}</span>
              <span className="block text-[.65rem] capitalize text-white/45">{me.role}</span>
            </span>
          </button>
          <div className="mt-1 grid grid-cols-2 gap-1">
            <button
              onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
              className="rounded-lg px-2 py-2 text-left text-xs text-white/55 hover:bg-white/7 hover:text-white"
            >
              <Icon name={theme === 'dark' ? 'sun' : 'moon'} />{' '}
              <span className="ml-1">{theme === 'dark' ? 'Light' : 'Dark'}</span>
            </button>
            <button
              onClick={signOut}
              className="rounded-lg px-2 py-2 text-left text-xs text-white/55 hover:bg-white/7 hover:text-white"
            >
              <Icon name="logout" /> <span className="ml-1">Sign out</span>
            </button>
          </div>
        </div>
      </aside>
      <main className="min-w-0 px-[clamp(1rem,3vw,2.5rem)] py-6 lg:col-start-2">
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
  const rows = Array.isArray(data) ? data : [];
  if (tab === 'articles') return <ArticlesPanel rows={rows} go={go} reload={load} />;
  return (
    <>
      <Heading
        eyebrow="Administration"
        title={title ?? 'Dashboard'}
        action={
          tab === 'sessions' ? (
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
            <thead className="admin-table-head text-xs uppercase tracking-wider">
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
                            window.history.replaceState({}, '', `/admin/articles/${fork.id}/edit`);
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

function ArticlesPanel({
  rows,
  go,
  reload,
}: {
  rows: any[];
  go: (v: Tab) => void;
  reload: () => void;
}) {
  const [query, setQuery] = useState('');
  const [filter, setFilter] = useState('all');
  const [menu, setMenu] = useState('');
  const filtered = useMemo(
    () =>
      rows.filter(
        (row) =>
          (filter === 'all' || row.state === filter) &&
          `${row.title} ${row.excerpt} ${row.author}`.toLowerCase().includes(query.toLowerCase()),
      ),
    [rows, query, filter],
  );
  const open = (id: string) => {
    sessionStorage.setItem('rfs-edit-post', id);
    go('editor');
    window.history.replaceState({}, '', `/admin/articles/${id}/edit`);
  };
  const remove = async (row: any) => {
    const typed = prompt(`Type the exact title to permanently delete this article:\n${row.title}`);
    if (
      typed !== row.title ||
      !confirm(
        'This permanently deletes the article, versions, comments, bookmarks, and references. Continue?',
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
    reload();
  };
  return (
    <>
      <Heading
        eyebrow="Editorial workspace"
        title="Articles"
        description="Draft, refine, review and publish from one focused workspace."
        action={
          <button onClick={() => go('editor')} className="admin-primary">
            <Icon name="plus" /> New article
          </button>
        }
      />
      <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:items-center">
        <label className="admin-search flex min-w-0 flex-1 items-center gap-2 rounded-xl px-3.5 py-2.5">
          <Icon name="search" />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search title, excerpt or author…"
            className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted"
          />
        </label>
        <div className="admin-segment flex w-fit rounded-xl p-1" aria-label="Filter articles">
          {['all', 'draft', 'in_review', 'published'].map((value) => (
            <button
              key={value}
              onClick={() => setFilter(value)}
              className={`rounded-lg px-3 py-1.5 text-xs font-semibold capitalize transition ${filter === value ? 'admin-segment-active shadow-sm' : 'text-muted hover:text-ink'}`}
            >
              {value.replace('_', ' ')}
            </button>
          ))}
        </div>
      </div>
      {filtered.length === 0 ? (
        <div className="admin-card mt-6 rounded-2xl border border-dashed p-12 text-center">
          <span className="mx-auto grid size-11 place-items-center rounded-xl bg-surface text-muted">
            <Icon name="articles" />
          </span>
          <h2 className="mt-4 font-semibold">No articles found</h2>
          <p className="mt-1 text-sm text-muted">Try another filter or start a new article.</p>
        </div>
      ) : (
        <div className="mt-6 grid gap-4 xl:grid-cols-2 2xl:grid-cols-3">
          {filtered.map((row) => (
            <article
              key={row.id}
              onClick={() => open(row.id)}
              className="admin-card group relative cursor-pointer rounded-2xl p-5 transition duration-200 hover:-translate-y-0.5 hover:shadow-lg"
            >
              <div className="flex items-start justify-between gap-4">
                <span className={`status-pill status-${row.state}`}>
                  {String(row.state).replace('_', ' ')}
                </span>
                <div className="relative">
                  <button
                    aria-label={`More actions for ${row.title}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      setMenu(menu === row.id ? '' : row.id);
                    }}
                    className="admin-icon-button"
                  >
                    <Icon name="more" />
                  </button>
                  {menu === row.id && (
                    <div className="admin-menu absolute right-0 top-9 z-10 w-40 rounded-xl p-1.5 shadow-xl">
                      {row.state === 'published' && (
                        <a
                          onClick={(event) => event.stopPropagation()}
                          href={`/critiques/${row.slug}/`}
                          target="_blank"
                          className="admin-menu-item"
                        >
                          <Icon name="external" /> View live
                        </a>
                      )}
                      <button
                        className="admin-menu-item"
                        onClick={async (event) => {
                          event.stopPropagation();
                          const fork = await call<any>(`/api/v1/posts/${row.id}/fork`, {
                            method: 'POST',
                            body: '{}',
                          });
                          open(fork.id);
                        }}
                      >
                        <Icon name="copy" /> Duplicate
                      </button>
                      <button
                        className="admin-menu-item text-red-600"
                        onClick={(event) => {
                          event.stopPropagation();
                          void remove(row);
                        }}
                      >
                        <Icon name="trash" /> Delete
                      </button>
                    </div>
                  )}
                </div>
              </div>
              <h2 className="mt-5 line-clamp-2 font-display text-2xl font-semibold leading-tight tracking-[-.025em] group-hover:text-accent">
                {row.title || 'Untitled article'}
              </h2>
              <p className="mt-2 line-clamp-2 min-h-10 text-sm leading-relaxed text-muted">
                {row.excerpt ||
                  'No excerpt yet. Open the article to add a clear promise for readers.'}
              </p>
              <div className="mt-6 flex items-center justify-between border-t border-line pt-4 text-xs text-muted">
                <span className="flex items-center gap-2">
                  <span className="grid size-6 place-items-center rounded-full bg-surface font-bold text-ink">
                    {row.author?.slice(0, 1) || 'A'}
                  </span>
                  {row.author || 'Editorial team'}
                </span>
                <span>
                  {row.publishedAt
                    ? `Published ${date(row.publishedAt)}`
                    : `Edited ${date(row.updatedAt)}`}
                </span>
              </div>
            </article>
          ))}
        </div>
      )}
    </>
  );
}

function Editor({ me }: { me: Me }) {
  const [id, setId] = useState('');
  const [title, setTitle] = useState('Untitled article');
  const [excerpt, setExcerpt] = useState('');
  const [revision, setRevision] = useState(1);
  const [state, setState] = useState('Not saved');
  const [error, setError] = useState('');
  const [versions, setVersions] = useState<any[]>([]);
  const [compare, setCompare] = useState<string[]>([]);
  const [mode, setMode] = useState<'write' | 'preview'>('write');
  const [wordCount, setWordCount] = useState(11);
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
    onUpdate: ({ editor: current }) => {
      setWordCount(current.getText().trim().split(/\s+/).filter(Boolean).length);
      setState(navigator.onLine ? 'Waiting to save' : 'Offline');
    },
  });
  useEffect(() => {
    call<any[]>('/api/v1/categories').then(setCategories);
    call<any[]>('/api/v1/tags').then(setTags);
  }, []);
  useEffect(() => {
    const routeMatch = window.location.pathname.match(/^\/admin\/articles\/([^/]+)\/edit\/?$/);
    const editing = routeMatch?.[1] ?? sessionStorage.getItem('rfs-edit-post');
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
      window.history.replaceState({}, '', `/admin/articles/${p.id}/edit`);
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
      <Heading
        eyebrow={id ? 'Article editor' : 'Fresh draft'}
        title={id ? 'Edit article' : 'Write something worth reading'}
        description={
          id
            ? `${state} · Revision ${revision} · ${wordCount} words · ${Math.max(1, Math.ceil(wordCount / 220))} min read`
            : 'Shape the idea, add evidence, then prepare it for publication.'
        }
        action={
          <div className="admin-segment flex rounded-xl p-1">
            <button
              onClick={() => setMode('write')}
              className={`rounded-lg px-3 py-1.5 text-xs font-semibold ${mode === 'write' ? 'admin-segment-active shadow-sm' : 'text-muted'}`}
            >
              <Icon name="edit" /> <span className="ml-1">Write</span>
            </button>
            <button
              onClick={() => setMode('preview')}
              className={`rounded-lg px-3 py-1.5 text-xs font-semibold ${mode === 'preview' ? 'admin-segment-active shadow-sm' : 'text-muted'}`}
            >
              <Icon name="eye" /> <span className="ml-1">Preview</span>
            </button>
          </div>
        }
      />
      {compare.length === 2 && (
        <section className="admin-card mt-6 grid gap-4 rounded-2xl p-5 md:grid-cols-2">
          {compare.map((versionId) => {
            const version = versions.find((item) => item.id === versionId);
            return (
              <article key={versionId}>
                <p className="text-xs font-bold uppercase tracking-widest text-accent">
                  Version {version?.number}
                </p>
                <h2 className="mt-2 font-display text-3xl">{version?.title}</h2>
                <p className="mt-3 text-sm text-muted">{version?.excerpt}</p>
                <pre className="mt-4 max-h-72 overflow-auto whitespace-pre-wrap rounded-xl bg-surface p-3 text-xs">
                  {JSON.stringify(version?.content, null, 2)}
                </pre>
              </article>
            );
          })}
        </section>
      )}
      <div className="mt-6 grid gap-5 xl:grid-cols-[minmax(0,1fr)_17rem]">
        <section className="admin-editor overflow-hidden rounded-2xl">
          <div className="px-[clamp(1.25rem,5vw,4.5rem)] pt-[clamp(1.5rem,5vw,4rem)]">
            <input
              value={title}
              onChange={(e) => {
                setTitle(e.target.value);
                if (id) setState('Waiting to save');
              }}
              aria-label="Article title"
              placeholder="Article title"
              className="w-full border-0 bg-transparent font-display text-[clamp(2.35rem,5vw,4.5rem)] font-semibold leading-[1.02] tracking-[-.045em] outline-none placeholder:text-muted/35"
            />
            <textarea
              value={excerpt}
              onChange={(e) => {
                setExcerpt(e.target.value);
                if (id) setState('Waiting to save');
              }}
              placeholder="Write a concise summary that tells readers why this matters…"
              maxLength={240}
              className="mt-4 min-h-16 w-full resize-none border-0 bg-transparent text-lg leading-relaxed text-muted outline-none placeholder:text-muted/45"
            />
            <div className="flex justify-end text-[.65rem] text-muted">{excerpt.length}/240</div>
          </div>
          {mode === 'write' ? (
            <>
              <div className="admin-toolbar sticky top-0 z-[5] mt-5 flex flex-wrap items-center gap-1 border-y border-line px-3 py-2">
                {[
                  ['undo', 'Undo', () => editor?.chain().focus().undo().run(), false],
                  ['redo', 'Redo', () => editor?.chain().focus().redo().run(), false],
                  [
                    'bold',
                    'Bold',
                    () => editor?.chain().focus().toggleBold().run(),
                    editor?.isActive('bold'),
                  ],
                  [
                    'italic',
                    'Italic',
                    () => editor?.chain().focus().toggleItalic().run(),
                    editor?.isActive('italic'),
                  ],
                  [
                    'strike',
                    'Strike',
                    () => editor?.chain().focus().toggleStrike().run(),
                    editor?.isActive('strike'),
                  ],
                  [
                    'h2',
                    'Heading',
                    () => editor?.chain().focus().toggleHeading({ level: 2 }).run(),
                    editor?.isActive('heading', { level: 2 }),
                  ],
                  [
                    'quote',
                    'Quote',
                    () => editor?.chain().focus().toggleBlockquote().run(),
                    editor?.isActive('blockquote'),
                  ],
                  [
                    'list',
                    'Bullets',
                    () => editor?.chain().focus().toggleBulletList().run(),
                    editor?.isActive('bulletList'),
                  ],
                  [
                    'ordered',
                    'Numbered list',
                    () => editor?.chain().focus().toggleOrderedList().run(),
                    editor?.isActive('orderedList'),
                  ],
                  [
                    'code',
                    'Code',
                    () => editor?.chain().focus().toggleCodeBlock().run(),
                    editor?.isActive('codeBlock'),
                  ],
                  [
                    'rule',
                    'Divider',
                    () => editor?.chain().focus().setHorizontalRule().run(),
                    false,
                  ],
                ].map(([icon, label, fn, active]) => (
                  <button
                    key={String(label)}
                    title={String(label)}
                    aria-label={String(label)}
                    onClick={fn as () => void}
                    className={`admin-tool ${active ? 'admin-tool-active' : ''}`}
                  >
                    <Icon name={String(icon)} />
                  </button>
                ))}
                <span className="mx-1 h-5 w-px bg-line" />
                <label className="admin-tool cursor-pointer" title="Add image">
                  <Icon name="image" />
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
                className="article-body min-h-[36rem] px-[clamp(1.25rem,5vw,4.5rem)] py-10 [&_.tiptap]:min-h-[36rem] [&_.tiptap]:outline-none"
              />
            </>
          ) : (
            <div className="min-h-[36rem] px-[clamp(1.25rem,5vw,4.5rem)] py-10">
              <p className="mb-8 text-sm text-muted">Reader preview</p>
              <h1 className="font-display text-[clamp(2.5rem,6vw,5rem)] font-semibold leading-none tracking-[-.045em]">
                {title}
              </h1>
              <p className="mt-5 border-l-2 border-accent pl-4 text-lg leading-relaxed text-muted">
                {excerpt}
              </p>
              <div
                className="article-body mt-10"
                dangerouslySetInnerHTML={{ __html: editor?.getHTML() ?? '' }}
              />
            </div>
          )}
        </section>
        <aside className="space-y-3 xl:sticky xl:top-6 xl:self-start">
          <div className="admin-card rounded-2xl p-4">
            <div className="flex items-center justify-between">
              <p className="text-xs font-bold uppercase tracking-widest text-muted">Publication</p>
              <span
                className={`size-2 rounded-full ${state === 'Saved' || state.includes('Published') ? 'bg-emerald-500' : 'bg-amber-500'}`}
              />
            </div>
            <p className="mt-3 text-xs leading-relaxed text-muted">
              {me.publishMode === 'review_required'
                ? 'This article will be sent to an editor for approval.'
                : 'You can publish this article directly.'}
            </p>
            {!id ? (
              <button onClick={create} className="admin-primary mt-4 w-full justify-center">
                <Icon name="save" /> Save draft
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
                  className="admin-secondary mt-4 w-full justify-center"
                >
                  <Icon name="checkpoint" /> Checkpoint
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
                  className="admin-primary mt-2 w-full justify-center"
                >
                  <Icon name="send" />{' '}
                  {me.publishMode === 'review_required' ? 'Submit for review' : 'Publish now'}
                </button>
              </>
            )}
          </div>
          {id && (
            <TaxonomyPanel
              postId={id}
              canCreateCategory={me.role === 'superadmin'}
              categories={categories}
              setCategories={setCategories}
              tags={tags}
              setTags={setTags}
              category={category}
              setCategory={setCategory}
              selectedTags={selectedTags}
              setSelectedTags={setSelectedTags}
              onSaved={setState}
              onError={setError}
            />
          )}
          <div className="admin-card rounded-2xl p-4">
            <p className="text-xs font-bold uppercase tracking-widest text-muted">Autosave</p>
            <p className="mt-3 flex items-center gap-2 text-sm">
              <span
                className={`size-2 rounded-full ${state === 'Saved' ? 'bg-emerald-500' : state === 'Offline' ? 'bg-red-500' : 'bg-amber-500'}`}
              />
              {state}
            </p>
            <p className="mt-1 text-xs text-muted">
              Revision {revision} · {wordCount} words
            </p>
          </div>
          {id && (
            <div className="admin-card rounded-2xl p-4">
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

function TaxonomyPanel({
  postId,
  canCreateCategory,
  categories,
  setCategories,
  tags,
  setTags,
  category,
  setCategory,
  selectedTags,
  setSelectedTags,
  onSaved,
  onError,
}: {
  postId: string;
  canCreateCategory: boolean;
  categories: any[];
  setCategories: React.Dispatch<React.SetStateAction<any[]>>;
  tags: any[];
  setTags: React.Dispatch<React.SetStateAction<any[]>>;
  category: string;
  setCategory: (value: string) => void;
  selectedTags: string[];
  setSelectedTags: (value: string[]) => void;
  onSaved: (value: string) => void;
  onError: (value: string) => void;
}) {
  const [creating, setCreating] = useState<'category' | 'tag' | null>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('Changes save automatically');
  const visibleTags = tags.filter((tag) => tag.name.toLowerCase().includes(query.toLowerCase()));

  const saveMetadata = async (nextCategory: string, nextTags: string[]) => {
    setMessage('Saving…');
    onError('');
    try {
      await call(`/api/v1/posts/${postId}/metadata`, {
        method: 'PATCH',
        body: JSON.stringify({ categoryId: nextCategory, tagIds: nextTags }),
      });
      setMessage('Saved');
      onSaved('Metadata saved');
    } catch (error) {
      setMessage('Could not save');
      onError(error instanceof Error ? error.message : 'Could not save classification');
    }
  };

  const chooseCategory = (value: string) => {
    setCategory(value);
    void saveMetadata(value, selectedTags);
  };

  const toggleTag = (tagId: string) => {
    const next = selectedTags.includes(tagId)
      ? selectedTags.filter((id) => id !== tagId)
      : [...selectedTags, tagId];
    setSelectedTags(next);
    void saveMetadata(category, next);
  };

  const closeCreator = () => {
    setCreating(null);
    setName('');
    setDescription('');
  };

  const createTaxonomy = async (event: SyntheticEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!name.trim()) return;
    setBusy(true);
    onError('');
    try {
      const endpoint = creating === 'category' ? '/api/v1/categories' : '/api/v1/tags';
      const created = await call<{ id: string }>(endpoint, {
        method: 'POST',
        body: JSON.stringify({ name: name.trim(), description: description.trim() }),
      });
      if (creating === 'category') {
        const nextCategories = await call<any[]>('/api/v1/categories');
        setCategories(nextCategories);
        setCategory(created.id);
        await saveMetadata(created.id, selectedTags);
      } else {
        const nextTags = await call<any[]>('/api/v1/tags');
        const nextSelected = [...selectedTags, created.id];
        setTags(nextTags);
        setSelectedTags(nextSelected);
        await saveMetadata(category, nextSelected);
      }
      closeCreator();
    } catch (error) {
      onError(error instanceof Error ? error.message : `Could not create ${creating}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="taxonomy-panel overflow-hidden rounded-2xl">
      <header className="flex items-start justify-between border-b border-line px-4 py-4">
        <div className="flex items-center gap-3">
          <span className="taxonomy-icon grid size-9 place-items-center rounded-xl">
            <Icon name="tag" />
          </span>
          <div>
            <h2 className="text-sm font-bold">Organize article</h2>
            <p className="mt-0.5 text-[.68rem] text-muted">Help readers discover this story</p>
          </div>
        </div>
        <span
          className={`mt-1 flex items-center gap-1.5 text-[.65rem] ${message === 'Could not save' ? 'text-amber-600' : 'text-muted'}`}
        >
          <span
            className={`size-1.5 rounded-full ${message === 'Saving…' ? 'animate-pulse bg-amber-500' : message === 'Saved' ? 'bg-teal-500' : 'bg-line'}`}
          />
          {message}
        </span>
      </header>

      <div className="space-y-5 p-4">
        <div>
          <div className="mb-2 flex items-center justify-between">
            <label htmlFor="article-category" className="text-xs font-bold">
              Category
            </label>
            {canCreateCategory && creating !== 'category' && (
              <button
                onClick={() => {
                  setCreating('category');
                  setName('');
                }}
                className="taxonomy-link"
              >
                <Icon name="plus" /> New
              </button>
            )}
          </div>
          <div className="relative">
            <select
              id="article-category"
              value={category}
              onChange={(event) => chooseCategory(event.target.value)}
              className="taxonomy-select w-full appearance-none rounded-xl px-3 py-2.5 pr-9 text-xs font-semibold outline-none"
            >
              <option value="">No category</option>
              {categories.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
            <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted">
              <Icon name="chevron" />
            </span>
          </div>
          {categories.length === 0 && !creating && (
            <p className="mt-2 text-[.68rem] leading-relaxed text-muted">
              No categories yet. Articles can still be published without one.
            </p>
          )}
        </div>

        {creating === 'category' && (
          <form onSubmit={createTaxonomy} className="taxonomy-creator rounded-xl p-3">
            <div className="flex items-center justify-between">
              <p className="text-xs font-bold">Create category</p>
              <button
                type="button"
                onClick={closeCreator}
                className="admin-icon-button -mr-1 -mt-1"
                aria-label="Cancel"
              >
                <Icon name="close" />
              </button>
            </div>
            <input
              autoFocus
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Category name"
              className="taxonomy-field mt-3 w-full rounded-lg px-3 py-2 text-xs outline-none"
            />
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder="Short description (optional)"
              className="taxonomy-field mt-2 min-h-16 w-full resize-none rounded-lg px-3 py-2 text-xs outline-none"
            />
            <button disabled={busy} className="taxonomy-action mt-2 w-full justify-center">
              <Icon name="plus" /> {busy ? 'Creating…' : 'Create and select'}
            </button>
          </form>
        )}

        <div>
          <div className="mb-2 flex items-center justify-between">
            <label htmlFor="tag-search" className="text-xs font-bold">
              Topics{' '}
              <span className="font-normal text-muted">
                {selectedTags.length ? `· ${selectedTags.length} selected` : ''}
              </span>
            </label>
            {creating !== 'tag' && (
              <button
                onClick={() => {
                  setCreating('tag');
                  setName('');
                }}
                className="taxonomy-link"
              >
                <Icon name="plus" /> New
              </button>
            )}
          </div>
          {tags.length > 5 && (
            <label className="taxonomy-search mb-2 flex items-center gap-2 rounded-lg px-2.5 py-2">
              <Icon name="search" />
              <input
                id="tag-search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Find a topic…"
                className="min-w-0 flex-1 bg-transparent text-xs outline-none"
              />
            </label>
          )}
          {tags.length === 0 ? (
            <button
              onClick={() => setCreating('tag')}
              className="taxonomy-empty w-full rounded-xl border border-dashed p-4 text-center"
            >
              <span className="mx-auto grid size-8 place-items-center rounded-full">
                <Icon name="tag" />
              </span>
              <strong className="mt-2 block text-xs">Add the first topic</strong>
              <span className="mt-1 block text-[.68rem] text-muted">
                Topics connect related articles.
              </span>
            </button>
          ) : (
            <div className="flex max-h-36 flex-wrap gap-1.5 overflow-auto">
              {visibleTags.map((tag) => {
                const active = selectedTags.includes(tag.id);
                return (
                  <button
                    key={tag.id}
                    onClick={() => toggleTag(tag.id)}
                    aria-pressed={active}
                    className={`taxonomy-chip ${active ? 'taxonomy-chip-active' : ''}`}
                  >
                    {active && <Icon name="check" />}
                    {tag.name}
                  </button>
                );
              })}
              {visibleTags.length === 0 && (
                <p className="py-3 text-xs text-muted">No matching topics.</p>
              )}
            </div>
          )}
        </div>

        {creating === 'tag' && (
          <form onSubmit={createTaxonomy} className="taxonomy-creator rounded-xl p-3">
            <div className="flex items-center justify-between">
              <p className="text-xs font-bold">Create topic</p>
              <button
                type="button"
                onClick={closeCreator}
                className="admin-icon-button -mr-1 -mt-1"
                aria-label="Cancel"
              >
                <Icon name="close" />
              </button>
            </div>
            <input
              autoFocus
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Topic name"
              className="taxonomy-field mt-3 w-full rounded-lg px-3 py-2 text-xs outline-none"
            />
            <button disabled={busy} className="taxonomy-action mt-2 w-full justify-center">
              <Icon name="plus" /> {busy ? 'Creating…' : 'Create and select'}
            </button>
          </form>
        )}
      </div>
    </section>
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
        <button className="admin-neutral-action justify-center px-4 py-2.5 font-semibold">
          Add author
        </button>
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
        <button
          disabled={busy}
          className="admin-neutral-action self-end justify-center px-5 py-2.5 font-semibold"
        >
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
          <button className="admin-neutral-action justify-center px-5 py-2.5 font-semibold">
            Save profile
          </button>
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
  description,
  action,
}: {
  eyebrow: string;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-5 border-b border-line pb-5">
      <div>
        <p className="text-xs font-bold uppercase tracking-[.18em] text-accent">{eyebrow}</p>
        <h1 className="mt-2 font-display text-[clamp(2.25rem,4vw,3.5rem)] font-semibold leading-none tracking-[-.045em]">
          {title}
        </h1>
        {description && <p className="mt-2 text-sm text-muted">{description}</p>}
      </div>
      {action}
    </header>
  );
}

function Icon({ name }: { name: string }) {
  const paths: Record<string, React.ReactNode> = {
    plus: <path d="M12 5v14M5 12h14" />,
    articles: (
      <>
        <path d="M5 4h11a3 3 0 0 1 3 3v13H7a2 2 0 0 1-2-2V4Z" />
        <path d="M8 8h7M8 12h7M8 16h4" />
      </>
    ),
    library: (
      <>
        <path d="m4 19.5 5-2.5 5 2.5 6-3V4l-6 3-5-2.5L4 7v12.5Z" />
        <path d="M9 4.5V17M14 7v12.5" />
      </>
    ),
    editor: (
      <>
        <path d="M4 20h4l11-11-4-4L4 16v4Z" />
        <path d="m13.5 6.5 4 4" />
      </>
    ),
    reviews: (
      <>
        <path d="M9 11l2 2 4-5" />
        <path d="M5 3h14v18H5z" />
      </>
    ),
    media: (
      <>
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <path d="m3 16 5-5 4 4 2-2 7 6M15 9h.01" />
      </>
    ),
    comments: <path d="M4 5h16v11H8l-4 4V5Z" />,
    reports: (
      <>
        <path d="M5 3v18M5 4h12l-2 4 2 4H5" />
      </>
    ),
    people: (
      <>
        <circle cx="9" cy="8" r="3" />
        <path d="M3 20c0-4 2-6 6-6s6 2 6 6M16 6a3 3 0 0 1 0 6M17 14c2.7.5 4 2.5 4 6" />
      </>
    ),
    readers: (
      <>
        <path d="M4 5h7a3 3 0 0 1 3 3v11H7a3 3 0 0 0-3 2V5Z" />
        <path d="M20 5h-3" />
      </>
    ),
    sessions: (
      <>
        <rect x="3" y="4" width="18" height="14" rx="2" />
        <path d="M8 21h8M12 18v3" />
      </>
    ),
    audit: (
      <>
        <circle cx="12" cy="12" r="9" />
        <path d="M12 7v5l3 2" />
      </>
    ),
    profile: (
      <>
        <circle cx="12" cy="8" r="4" />
        <path d="M4 21a8 8 0 0 1 16 0" />
      </>
    ),
    sun: (
      <>
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
      </>
    ),
    moon: <path d="M20 15.5A8.5 8.5 0 0 1 8.5 4 8.5 8.5 0 1 0 20 15.5Z" />,
    logout: (
      <>
        <path d="M10 4H4v16h6M14 8l4 4-4 4M8 12h10" />
      </>
    ),
    search: (
      <>
        <circle cx="11" cy="11" r="7" />
        <path d="m20 20-4-4" />
      </>
    ),
    more: (
      <>
        <circle cx="5" cy="12" r="1" fill="currentColor" />
        <circle cx="12" cy="12" r="1" fill="currentColor" />
        <circle cx="19" cy="12" r="1" fill="currentColor" />
      </>
    ),
    external: (
      <>
        <path d="M14 4h6v6M20 4l-9 9" />
        <path d="M18 13v7H4V6h7" />
      </>
    ),
    copy: (
      <>
        <rect x="8" y="8" width="11" height="12" rx="2" />
        <path d="M16 8V4H5a2 2 0 0 0-2 2v11h5" />
      </>
    ),
    trash: (
      <>
        <path d="M4 7h16M9 7V4h6v3M7 7l1 13h8l1-13" />
      </>
    ),
    edit: (
      <>
        <path d="M4 20h4l11-11-4-4L4 16v4Z" />
        <path d="m13.5 6.5 4 4" />
      </>
    ),
    eye: (
      <>
        <path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Z" />
        <circle cx="12" cy="12" r="3" />
      </>
    ),
    undo: (
      <>
        <path d="m9 7-5 5 5 5" />
        <path d="M4 12h10a6 6 0 0 1 6 6" />
      </>
    ),
    redo: (
      <>
        <path d="m15 7 5 5-5 5" />
        <path d="M20 12H10a6 6 0 0 0-6 6" />
      </>
    ),
    bold: (
      <>
        <path d="M7 4h6a4 4 0 0 1 0 8H7zM7 12h7a4 4 0 0 1 0 8H7z" />
      </>
    ),
    italic: (
      <>
        <path d="M10 4h8M6 20h8M14 4 10 20" />
      </>
    ),
    strike: (
      <>
        <path d="M6 8c0-3 2-4 6-4 3 0 5 1 6 3M6 16c1 3 3 4 6 4 4 0 6-2 6-4M3 12h18" />
      </>
    ),
    h2: (
      <>
        <path d="M4 5v14M12 5v14M4 12h8M15 10c0-2 1-3 3-3s3 1 3 3c0 3-6 4-6 9h6" />
      </>
    ),
    quote: (
      <>
        <path d="M5 8h5v5H7c0 3-1 5-3 6M14 8h5v5h-3c0 3-1 5-3 6" />
      </>
    ),
    list: (
      <>
        <path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01" />
      </>
    ),
    ordered: (
      <>
        <path d="M10 6h11M10 12h11M10 18h11M4 4v4M3 8h3M3 11h2a1 1 0 0 1 0 2H3l3 3H3M3 18h3l-3 3h3" />
      </>
    ),
    code: (
      <>
        <path d="m8 9-4 3 4 3M16 9l4 3-4 3M14 5l-4 14" />
      </>
    ),
    rule: <path d="M4 12h16" />,
    image: (
      <>
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <path d="m3 16 5-5 4 4 2-2 7 6" />
      </>
    ),
    save: (
      <>
        <path d="M5 3h12l2 2v16H5z" />
        <path d="M8 3v6h8V3M8 21v-7h8v7" />
      </>
    ),
    checkpoint: (
      <>
        <circle cx="12" cy="12" r="9" />
        <path d="M12 7v5l4 2" />
      </>
    ),
    send: (
      <>
        <path d="m3 11 18-8-8 18-2-8-8-2Z" />
        <path d="m11 13 4-4" />
      </>
    ),
    tag: (
      <>
        <path d="M20 13 13 20 4 11V4h7l9 9Z" />
        <circle cx="8.5" cy="8.5" r="1" />
      </>
    ),
    chevron: <path d="m8 10 4 4 4-4" />,
    close: <path d="m6 6 12 12M18 6 6 18" />,
    check: <path d="m5 12 4 4L19 6" />,
  };
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      className="inline-block size-4 shrink-0"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {paths[name] ?? paths.articles}
    </svg>
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
