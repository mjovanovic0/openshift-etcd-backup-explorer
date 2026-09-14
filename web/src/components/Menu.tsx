import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { IconChevron } from "./Icons";

export type MenuItem = {
  id: string;
  label: string;
  hint?: string;
  href?: string;
  onSelect?: () => void;
  disabled?: boolean;
};

/** Menu is the small dropdown used by the download and export buttons. */
export function Menu({
  label,
  icon,
  items,
  align = "right",
  busy,
}: {
  label: string;
  icon?: ReactNode;
  items: MenuItem[];
  align?: "left" | "right";
  busy?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={ref} className="relative shrink-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        disabled={busy}
        className="flex items-center gap-1.5 rounded border border-line bg-surface px-2.5 py-1.5 text-[13px] text-ink hover:border-brand hover:text-brand disabled:opacity-50"
      >
        {busy ? <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-line border-t-brand" /> : icon}
        {label}
        <IconChevron className={`h-3.5 w-3.5 transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {open && (
        <ul
          className={`absolute top-full z-30 mt-1 w-[268px] overflow-hidden rounded border border-line bg-surface py-1 shadow-lg ${
            align === "right" ? "right-0" : "left-0"
          }`}
        >
          {items.map((item) => {
            const content = (
              <>
                <span className="block">{item.label}</span>
                {item.hint && <span className="block text-[11px] text-muted">{item.hint}</span>}
              </>
            );
            const className =
              "block w-full px-3 py-2 text-left text-[13px] text-ink hover:bg-brand-soft hover:text-brand";
            return (
              <li key={item.id}>
                {item.href && !item.disabled ? (
                  <a href={item.href} className={className} onClick={() => setOpen(false)}>
                    {content}
                  </a>
                ) : (
                  <button
                    type="button"
                    disabled={item.disabled}
                    className={`${className} disabled:opacity-40 disabled:hover:bg-transparent disabled:hover:text-ink`}
                    onClick={() => {
                      setOpen(false);
                      item.onSelect?.();
                    }}
                  >
                    {content}
                  </button>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
