import type { ReactNode } from "react";
import { IconChevron } from "./Icons";

/** Panel is one vertical column of the browser, with a fixed header. */
export function Panel({
  title,
  subtitle,
  right,
  children,
  className = "",
  bodyClassName = "",
}: {
  title?: ReactNode;
  subtitle?: ReactNode;
  right?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <section className={`flex min-h-0 min-w-0 flex-col bg-surface ${className}`}>
      {title !== undefined && (
        <header className="flex shrink-0 items-start justify-between gap-3 border-b border-line-soft px-4 py-3">
          <div className="min-w-0">
            <h2 className="truncate text-[17px] font-medium text-ink">{title}</h2>
            {subtitle !== undefined && <p className="mt-0.5 text-xs text-muted">{subtitle}</p>}
          </div>
          {right}
        </header>
      )}
      <div className={`min-h-0 flex-1 overflow-auto ${bodyClassName}`}>{children}</div>
    </section>
  );
}

/** CollapsibleHeader matches the console's "Resource Types" panel header. */
export function CollapsibleHeader({
  title,
  open,
  onToggle,
}: {
  title: string;
  open: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="flex w-full shrink-0 items-center justify-between gap-2 border-b border-line-soft px-4 py-3 text-left hover:bg-surface-alt"
    >
      <h2 className="text-[17px] font-medium text-ink">{title}</h2>
      <IconChevron className={`h-4 w-4 text-muted transition-transform ${open ? "" : "-rotate-90"}`} />
    </button>
  );
}

export function StatusDot({ tone }: { tone: string }) {
  const color =
    { ok: "bg-ok", warn: "bg-warn", error: "bg-danger", info: "bg-info", neutral: "bg-muted" }[tone] ?? "bg-muted";
  return <span className={`inline-block h-2 w-2 shrink-0 rounded-full ${color}`} />;
}

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 px-4 py-6 text-sm text-muted">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-line border-t-brand" />
      {label ?? "Loading"}
    </div>
  );
}

export function ErrorNote({ message }: { message: string }) {
  return (
    <div className="m-4 rounded border border-danger/30 bg-danger/5 px-3 py-2 text-sm text-danger">{message}</div>
  );
}

export function EmptyNote({ children }: { children: ReactNode }) {
  return <div className="px-4 py-10 text-center text-sm text-muted">{children}</div>;
}

/** Pill is the small grey chip used for encodings and scopes. */
export function Pill({ children, tone = "grey" }: { children: ReactNode; tone?: "grey" | "blue" | "green" }) {
  const styles = {
    grey: "bg-surface-alt text-muted border-line",
    blue: "bg-brand-soft text-brand border-brand/20",
    green: "bg-ok/10 text-ok border-ok/20",
  }[tone];
  return (
    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] leading-4 ${styles}`}>
      {children}
    </span>
  );
}
