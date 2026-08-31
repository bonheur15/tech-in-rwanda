import { useEffect, useRef, useState, type SyntheticEvent } from 'react';

declare global {
  interface Window {
    turnstile?: {
      render: (target: HTMLElement, options: Record<string, unknown>) => string;
      reset: (id?: string) => void;
    };
  }
}
const cookie = (name: string) =>
  decodeURIComponent(
    document.cookie
      .split('; ')
      .find((v) => v.startsWith(name + '='))
      ?.split('=')[1] ?? '',
  );
async function api<T = any>(path: string, init: RequestInit = {}) {
  const response = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      ...(!['GET', 'HEAD'].includes(init.method ?? 'GET')
        ? { 'X-CSRF-Token': cookie('__Host-rfs_reader_csrf') || cookie('rfs_reader_csrf') }
        : {}),
    },
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error?.message ?? 'Request failed');
  return body.data as T;
}

export default function ReaderAccount({ siteKey }: { siteKey: string }) {
  const [me, setMe] = useState<any>(null),
    [checked, setChecked] = useState(false),
    [email, setEmail] = useState(''),
    [code, setCode] = useState(''),
    [step, setStep] = useState('email'),
    [token, setToken] = useState(''),
    [message, setMessage] = useState('');
  const widget = useRef<HTMLDivElement>(null);
  useEffect(() => {
    api('/api/v1/auth/me?kind=reader')
      .then(setMe)
      .catch(() => {})
      .finally(() => setChecked(true));
  }, []);
  useEffect(() => {
    if (me || step !== 'email') return;
    const script = document.createElement('script');
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit';
    script.async = true;
    script.onload = () => {
      if (widget.current && window.turnstile)
        window.turnstile.render(widget.current, {
          sitekey: siteKey,
          callback: (value: string) => setToken(value),
          'expired-callback': () => setToken(''),
        });
    };
    document.head.appendChild(script);
    return () => script.remove();
  }, [me, step, siteKey]);
  if (!checked) return <p>Opening account…</p>;
  if (me?.onboardingRequired) return <Onboarding done={() => location.reload()} />;
  if (me) return <ReaderHome me={me} />;
  const request = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    setMessage('');
    try {
      await api('/api/v1/auth/readers/request-otp', {
        method: 'POST',
        body: JSON.stringify({ email, turnstileToken: token }),
      });
      setStep('code');
      setMessage('Check your email for a six-digit code.');
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Could not send code');
      window.turnstile?.reset();
    }
  };
  const verify = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    try {
      await api('/api/v1/auth/readers/verify-otp', {
        method: 'POST',
        body: JSON.stringify({ email, code }),
      });
      location.reload();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Invalid code');
    }
  };
  return (
    <section className="mx-auto max-w-md border border-line bg-white p-8">
      <p className="text-xs font-bold uppercase tracking-widest text-accent">Reader account</p>
      <h1 className="mt-4 font-display text-5xl leading-none">Keep what matters.</h1>
      <p className="mt-5 text-muted">
        Bookmark critiques and join moderated discussions. No password required.
      </p>
      <form onSubmit={step === 'email' ? request : verify} className="mt-8">
        {step === 'email' ? (
          <>
            <label className="text-sm font-semibold">
              Email
              <input
                required
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="mt-2 w-full border border-line px-3 py-3"
              />
            </label>
            <div ref={widget} className="mt-5 min-h-[65px]" />
          </>
        ) : (
          <label className="text-sm font-semibold">
            Six-digit code
            <input
              required
              inputMode="numeric"
              pattern="[0-9]{6}"
              maxLength={6}
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
              className="mt-2 w-full border border-line px-3 py-3 font-mono text-xl tracking-[.3em]"
            />
          </label>
        )}
        <button
          disabled={step === 'email' && !token}
          className="mt-5 w-full bg-ink px-4 py-3 font-semibold text-white disabled:opacity-40"
        >
          {step === 'email' ? 'Send sign-in code' : 'Verify and continue'}
        </button>
        {message && <p className="mt-4 text-sm text-muted">{message}</p>}
      </form>
    </section>
  );
}

function Onboarding({ done }: { done: () => void }) {
  const [username, setUsername] = useState(''),
    [avatar, setAvatar] = useState('sunrise'),
    [message, setMessage] = useState('');
  const avatars = ['sunrise', 'hills', 'ink', 'agaseke', 'volcano', 'coffee'];
  const submit = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    try {
      await api('/api/v1/reader/onboarding', {
        method: 'POST',
        body: JSON.stringify({ username, avatar, emailVisible: false }),
      });
      done();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Could not finish profile');
    }
  };
  return (
    <form onSubmit={submit} className="mx-auto max-w-xl border border-line bg-white p-8">
      <p className="text-xs font-bold uppercase tracking-widest text-accent">One last step</p>
      <h1 className="mt-4 font-display text-5xl">Choose your public identity.</h1>
      <label className="mt-8 block text-sm font-semibold">
        Permanent username
        <input
          required
          minLength={3}
          maxLength={24}
          value={username}
          onChange={(e) => setUsername(e.target.value.toLowerCase().replace(/[^a-z0-9_]/g, ''))}
          className="mt-2 w-full border border-line px-3 py-3"
        />
      </label>
      <fieldset className="mt-6">
        <legend className="text-sm font-semibold">Built-in avatar</legend>
        <div className="mt-3 grid grid-cols-3 gap-3">
          {avatars.map((x) => (
            <label
              key={x}
              className={`cursor-pointer border p-3 text-center text-sm capitalize ${avatar === x ? 'border-accent bg-[#fff4f1]' : 'border-line'}`}
            >
              <input
                type="radio"
                className="sr-only"
                checked={avatar === x}
                onChange={() => setAvatar(x)}
              />
              {x}
            </label>
          ))}
        </div>
      </fieldset>
      <button className="mt-6 w-full bg-ink px-4 py-3 font-semibold text-white">
        Create reader profile
      </button>
      {message && <p className="mt-4 text-accent">{message}</p>}
    </form>
  );
}

function ReaderHome({ me }: { me: any }) {
  const [bookmarks, setBookmarks] = useState<any[]>([]),
    [sessions, setSessions] = useState<any[]>([]);
  useEffect(() => {
    api('/api/v1/bookmarks').then(setBookmarks);
    api('/api/v1/sessions?kind=reader').then(setSessions);
  }, []);
  return (
    <>
      <header className="border-b border-line pb-8">
        <p className="text-xs font-bold uppercase tracking-widest text-accent">Reader account</p>
        <h1 className="mt-3 font-display text-5xl">@{me.username}</h1>
        <p className="mt-3 text-muted">
          Avatar: {me.avatar} · Email is {me.emailVisible ? 'public' : 'private'}
        </p>
      </header>
      <div className="mt-8 grid gap-6 md:grid-cols-2">
        <section className="border border-line bg-white p-6">
          <h2 className="font-display text-3xl">Bookmarks</h2>
          <div className="mt-5 grid gap-4">
            {bookmarks.length ? (
              bookmarks.map((p) => (
                <a
                  key={p.id}
                  href={`/critiques/${p.slug}/`}
                  className="border-t border-line pt-3 font-semibold"
                >
                  {p.title}
                </a>
              ))
            ) : (
              <p className="text-muted">No saved critiques yet.</p>
            )}
          </div>
        </section>
        <section className="border border-line bg-white p-6">
          <h2 className="font-display text-3xl">Sessions</h2>
          <div className="mt-5 grid gap-4">
            {sessions.map((s) => (
              <div key={s.id} className="border-t border-line pt-3 text-sm">
                <p className="truncate font-semibold">{s.device}</p>
                <p className="text-muted">{s.ipAddress}</p>
                <button
                  onClick={async () => {
                    await api(`/api/v1/sessions/${s.id}?kind=reader`, {
                      method: 'DELETE',
                      body: '{}',
                    });
                    setSessions((v) => v.filter((x) => x.id !== s.id));
                  }}
                  className="mt-2 text-accent"
                >
                  Revoke
                </button>
              </div>
            ))}
          </div>
        </section>
      </div>
      <button
        onClick={async () => {
          await api('/api/v1/auth/logout?kind=reader', { method: 'POST', body: '{}' });
          location.reload();
        }}
        className="mt-8 text-sm font-semibold text-accent"
      >
        Sign out
      </button>
    </>
  );
}
