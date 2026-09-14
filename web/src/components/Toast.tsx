import { createContext, useCallback, useContext, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";

type Tone = "ok" | "error";
type Toast = { id: number; text: string; tone: Tone };

const ToastContext = createContext<(text: string, tone?: Tone) => void>(() => {});

/** useToast reports the outcome of an action that has no visible result of
 * its own, such as a copy. */
export function useToast() {
  return useContext(ToastContext);
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const next = useRef(0);

  const notify = useCallback((text: string, tone: Tone = "ok") => {
    const id = ++next.current;
    setToasts((list) => [...list, { id, text, tone }]);
    setTimeout(() => setToasts((list) => list.filter((t) => t.id !== id)), 4500);
  }, []);

  const value = useMemo(() => notify, [notify]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed right-4 bottom-20 z-50 flex flex-col gap-2" aria-live="polite">
        {toasts.map((t) => (
          <div
            key={t.id}
            role="status"
            className={`pointer-events-auto flex max-w-[420px] items-start gap-2 rounded border px-3 py-2 text-[13px] shadow-lg ${
              t.tone === "error"
                ? "border-danger/40 bg-danger/10 text-danger"
                : "border-ok/40 bg-surface text-ink"
            }`}
          >
            <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${t.tone === "error" ? "bg-danger" : "bg-ok"}`} />
            <span className="min-w-0 flex-1">{t.text}</span>
            <button
              type="button"
              onClick={() => setToasts((list) => list.filter((x) => x.id !== t.id))}
              aria-label="Dismiss"
              className="shrink-0 text-muted hover:text-ink"
            >
              ✕
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
