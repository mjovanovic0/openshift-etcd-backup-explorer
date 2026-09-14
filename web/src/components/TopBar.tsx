import { useEffect, useRef, useState } from "react";
import type { Backup } from "../api";
import { IconChevron, IconSearch, RedHatMark } from "./Icons";
import { countLabel } from "../format";

export function TopBar({
  backups,
  backupId,
  onBackupChange,
  clusterName,
  search,
  onSearchChange,
  onSearchSubmit,
}: {
  backups: Backup[];
  backupId: string;
  onBackupChange: (id: string) => void;
  clusterName: string;
  search: string;
  onSearchChange: (v: string) => void;
  onSearchSubmit: () => void;
}) {
  const current = backups.find((b) => b.id === backupId);

  return (
    <header className="flex h-16 shrink-0 items-center gap-4 bg-chrome pr-4 text-chrome-text">
      <div className="flex h-16 w-[200px] shrink-0 items-center gap-3 px-4">
        <RedHatMark />
        <div className="min-w-0 leading-tight">
          <div className="truncate text-[15px] font-semibold text-white">OpenShift</div>
          <div className="truncate text-[11px] text-chrome-text/70">etcd Backup Browser</div>
        </div>
      </div>

      <Dropdown
        label={`Backup: ${current ? current.label : "none"}`}
        items={backups.map((b) => ({
          id: b.id,
          label: b.label,
          hint: `${countLabel(b.resources)} objects`,
        }))}
        selected={backupId}
        onSelect={onBackupChange}
      />

      <div className="hidden items-center gap-2 rounded border border-chrome-border px-3 py-1.5 text-[13px] lg:flex">
        <span className="text-chrome-text/60">Cluster:</span>
        <span className="max-w-[220px] truncate text-white">{clusterName}</span>
      </div>

      <form
        className="relative ml-auto w-full max-w-[460px]"
        onSubmit={(e) => {
          e.preventDefault();
          onSearchSubmit();
        }}
      >
        <IconSearch className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-chrome-text/50" />
        <input
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Search resources, names, or content..."
          aria-label="Search resources"
          className="w-full rounded border border-chrome-border bg-chrome py-1.5 pr-3 pl-9 text-[13px] text-white placeholder:text-chrome-text/50 focus:border-info focus:outline-none"
        />
      </form>
    </header>
  );
}

type Item = { id: string; label: string; hint?: string };

function Dropdown({
  label,
  items,
  selected,
  onSelect,
}: {
  label: string;
  items: Item[];
  selected: string;
  onSelect: (id: string) => void;
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
        className="flex items-center gap-3 rounded border border-chrome-border bg-chrome-hover px-3 py-1.5 text-[13px] font-medium text-white hover:border-info/60"
      >
        <span className="max-w-[300px] truncate">{label}</span>
        <IconChevron className={`h-4 w-4 text-chrome-text/60 transition-transform ${open ? "rotate-180" : ""}`} />
      </button>

      {open && (
        <ul
          role="listbox"
          className="absolute top-full left-0 z-30 mt-1 max-h-80 w-[340px] overflow-auto rounded border border-line bg-surface py-1 shadow-lg"
        >
          {items.length === 0 && <li className="px-3 py-2 text-[13px] text-muted">No backups found</li>}
          {items.map((it) => (
            <li key={it.id}>
              <button
                type="button"
                role="option"
                aria-selected={it.id === selected}
                onClick={() => {
                  onSelect(it.id);
                  setOpen(false);
                }}
                className={`flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-[13px] hover:bg-brand-soft ${
                  it.id === selected ? "bg-brand-soft font-medium text-brand" : "text-ink"
                }`}
              >
                <span className="truncate">{it.label}</span>
                {it.hint && <span className="shrink-0 text-[11px] text-muted">{it.hint}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
