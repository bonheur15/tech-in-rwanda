import { useEffect, useId, useRef, useState } from 'react';

export type DialogField = {
  name: string;
  label: string;
  type?: 'text' | 'textarea' | 'number' | 'select';
  value?: string;
  required?: boolean;
  min?: number;
  options?: { label: string; value: string }[];
  hint?: string;
};
export type DialogRequest = {
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  fields?: DialogField[];
};

export function useModalDialog() {
  const [request, setRequest] = useState<DialogRequest | null>(null);
  const resolver = useRef<((value: Record<string, string> | null) => void) | null>(null);
  const open = (next: DialogRequest) =>
    new Promise<Record<string, string> | null>((resolve) => {
      resolver.current?.(null);
      resolver.current = resolve;
      setRequest(next);
    });
  const close = (value: Record<string, string> | null) => {
    resolver.current?.(value);
    resolver.current = null;
    setRequest(null);
  };
  return { open, dialog: request ? <ModalDialog request={request} close={close} /> : null };
}

function ModalDialog({
  request,
  close,
}: {
  request: DialogRequest;
  close: (value: Record<string, string> | null) => void;
}) {
  const titleId = useId();
  const descriptionId = useId();
  const panel = useRef<HTMLDivElement>(null);
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries((request.fields ?? []).map((field) => [field.name, field.value ?? ''])),
  );
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    document.body.style.overflow = 'hidden';
    panel.current?.querySelector<HTMLElement>('input, textarea, select, button')?.focus();
    const keydown = (event: KeyboardEvent) => event.key === 'Escape' && close(null);
    document.addEventListener('keydown', keydown);
    return () => {
      document.body.style.overflow = '';
      document.removeEventListener('keydown', keydown);
      previous?.focus();
    };
  }, []);
  return (
    <div
      className="fixed inset-0 z-[100] grid place-items-center bg-black/60 p-4 backdrop-blur-sm"
      onMouseDown={(event) => event.target === event.currentTarget && close(null)}
    >
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={request.description ? descriptionId : undefined}
        className="w-full max-w-lg rounded-2xl border border-line bg-white p-6 text-ink shadow-2xl dark:bg-[#171a18] dark:text-white"
      >
        <h2 id={titleId} className="font-display text-2xl font-semibold">
          {request.title}
        </h2>
        {request.description && (
          <p
            id={descriptionId}
            className="mt-2 whitespace-pre-line text-sm leading-relaxed text-muted"
          >
            {request.description}
          </p>
        )}
        <form
          className="mt-5"
          onSubmit={(event) => {
            event.preventDefault();
            close(values);
          }}
        >
          <div className="grid gap-4">
            {(request.fields ?? []).map((field) => (
              <label key={field.name} className="text-sm font-semibold">
                {field.label}
                {field.type === 'select' ? (
                  <select
                    required={field.required}
                    value={values[field.name]}
                    onChange={(event) =>
                      setValues((current) => ({ ...current, [field.name]: event.target.value }))
                    }
                    className="mt-2 w-full rounded-lg border border-line bg-white p-3 text-ink dark:bg-[#202421] dark:text-white"
                  >
                    {field.options?.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                ) : field.type === 'textarea' ? (
                  <textarea
                    required={field.required}
                    value={values[field.name]}
                    onChange={(event) =>
                      setValues((current) => ({ ...current, [field.name]: event.target.value }))
                    }
                    className="mt-2 min-h-28 w-full rounded-lg border border-line bg-white p-3 text-ink dark:bg-[#202421] dark:text-white"
                  />
                ) : (
                  <input
                    type={field.type ?? 'text'}
                    required={field.required}
                    min={field.min}
                    value={values[field.name]}
                    onChange={(event) =>
                      setValues((current) => ({ ...current, [field.name]: event.target.value }))
                    }
                    className="mt-2 w-full rounded-lg border border-line bg-white p-3 text-ink dark:bg-[#202421] dark:text-white"
                  />
                )}
                {field.hint && (
                  <span className="mt-1 block text-xs font-normal text-muted">{field.hint}</span>
                )}
              </label>
            ))}
          </div>
          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => close(null)}
              className="rounded-lg border border-line px-4 py-2.5 text-sm font-semibold"
            >
              {request.cancelLabel ?? 'Cancel'}
            </button>
            <button
              type="submit"
              className={`rounded-lg px-4 py-2.5 text-sm font-semibold text-white ${request.danger ? 'bg-red-700' : 'bg-ink dark:bg-white dark:text-ink'}`}
            >
              {request.confirmLabel ?? 'Continue'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
