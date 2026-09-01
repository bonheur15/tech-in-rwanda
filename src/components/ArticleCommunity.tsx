import { type SyntheticEvent, useEffect, useState } from 'react';
import { useModalDialog } from './ModalDialog';

const cookie = (name: string) =>
  decodeURIComponent(
    document.cookie
      .split('; ')
      .find((v) => v.startsWith(name + '='))
      ?.split('=')[1] ?? '',
  );
async function api<T = any>(path: string, init: RequestInit = {}) {
  const r = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      ...(!['GET', 'HEAD'].includes(init.method ?? 'GET')
        ? { 'X-CSRF-Token': cookie('__Host-rfs_reader_csrf') || cookie('rfs_reader_csrf') }
        : {}),
    },
  });
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error?.message ?? 'Request failed');
  return b.data as T;
}
export default function ArticleCommunity({ postId }: { postId: string }) {
  const modal = useModalDialog();
  const [reader, setReader] = useState<any>(null);
  const [comments, setComments] = useState<any[]>([]);
  const [body, setBody] = useState('');
  const [parentId, setParentId] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [message, setMessage] = useState('');
  const load = () =>
    api<any[]>(`/api/v1/articles/${postId}/comments`)
      .then(setComments)
      .catch(() => {});
  useEffect(() => {
    api('/api/v1/auth/me?kind=reader')
      .then((v) => v.kind === 'reader' && setReader(v))
      .catch(() => {});
    load();
  }, [postId]);
  const submit = async (e: SyntheticEvent<HTMLFormElement>) => {
    e.preventDefault();
    try {
      await api(
        editingId ? `/api/v1/comments/${editingId}` : `/api/v1/articles/${postId}/comments`,
        {
          method: editingId ? 'PATCH' : 'POST',
          body: JSON.stringify(editingId ? { body } : { body, parentId }),
        },
      );
      setBody('');
      setParentId(null);
      setEditingId(null);
      setMessage('Your comment is awaiting moderation.');
      load();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Could not submit comment');
    }
  };
  return (
    <section className="mx-auto mt-20 max-w-3xl border-t border-line pt-12">
      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="text-xs font-bold uppercase tracking-widest text-accent">Discussion</p>
          <h2 className="mt-2 font-display text-4xl">Reader responses</h2>
        </div>
        {reader ? (
          <button
            onClick={async () => {
              await api(`/api/v1/articles/${postId}/bookmarks`, {
                method: 'POST',
                body: '{}',
              });
              setMessage('Saved to your bookmarks.');
            }}
            className="border border-ink px-4 py-2 text-sm font-semibold"
          >
            Bookmark
          </button>
        ) : (
          <a href="/account/" className="border-b border-ink pb-1 text-sm font-semibold">
            Reader sign in
          </a>
        )}
      </div>
      <div className="mt-8 grid gap-5">
        {comments.length ? (
          comments.map((c) => (
            <article
              key={c.id}
              className="border-l-2 border-line pl-5"
              style={{ marginLeft: `${Math.min(c.depth, 2) * 1.25}rem` }}
            >
              <p className="text-sm leading-relaxed">{c.body}</p>
              <p className="mt-2 text-xs text-muted">
                <a href={`/readers/${c.username}/`}>@{c.username}</a> ·{' '}
                {new Date(c.createdAt).toLocaleDateString('en-RW')}
                {c.status === 'pending' && ' · Awaiting moderation'}
              </p>
              {reader && (
                <div className="mt-2 flex gap-4 text-xs font-semibold text-accent">
                  {c.status === 'approved' && c.depth < 2 && (
                    <button
                      onClick={() => {
                        setParentId(c.id);
                        setEditingId(null);
                        setBody('');
                      }}
                    >
                      Reply
                    </button>
                  )}
                  {c.mine && (
                    <button
                      onClick={() => {
                        setEditingId(c.id);
                        setParentId(null);
                        setBody(c.body);
                      }}
                    >
                      Edit
                    </button>
                  )}
                  {!c.mine && c.status === 'approved' && (
                    <button
                      onClick={async () => {
                        const result = await modal.open({
                          title: 'Report this comment',
                          description: 'Tell the moderation team what needs their attention.',
                          confirmLabel: 'Send report',
                          fields: [
                            {
                              name: 'reason',
                              label: 'Reason for reporting',
                              type: 'textarea',
                              required: true,
                            },
                          ],
                        });
                        if (!result) return;
                        await api(`/api/v1/comments/${c.id}/reports`, {
                          method: 'POST',
                          body: JSON.stringify({ reason: result.reason }),
                        });
                        setMessage('Report sent to the moderation team.');
                      }}
                    >
                      Report
                    </button>
                  )}
                </div>
              )}
            </article>
          ))
        ) : (
          <p className="text-muted">No approved comments yet.</p>
        )}
      </div>
      {reader ? (
        <form onSubmit={submit} className="mt-10 border border-line bg-white p-5">
          {(parentId || editingId) && (
            <div className="mb-4 flex items-center justify-between bg-paper p-3 text-sm">
              <span>{editingId ? 'Editing your comment' : 'Writing a reply'}</span>
              <button
                type="button"
                onClick={() => {
                  setParentId(null);
                  setEditingId(null);
                  setBody('');
                }}
                className="font-semibold text-accent"
              >
                Cancel
              </button>
            </div>
          )}
          <label className="text-sm font-semibold">
            Add a thoughtful comment
            <textarea
              required
              minLength={2}
              maxLength={3000}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              className="mt-2 min-h-28 w-full border border-line p-3"
            />
          </label>
          <button className="mt-3 bg-ink px-4 py-2.5 text-sm font-semibold text-white">
            {editingId
              ? 'Save edit for moderation'
              : parentId
                ? 'Submit reply for moderation'
                : 'Submit for moderation'}
          </button>
        </form>
      ) : (
        <p className="mt-10 text-sm text-muted">
          Sign in as a reader to bookmark this critique or join the discussion.
        </p>
      )}
      {message && <p className="mt-4 text-sm text-accent">{message}</p>}
      {modal.dialog}
    </section>
  );
}
