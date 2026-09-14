import { useMemo, useState } from "react";
import type { Kind } from "../api";
import { countLabel } from "../format";
import { CollapsibleHeader, Spinner } from "./Chrome";
import { IconCube, IconList, IconSearch } from "./Icons";

/** KindList is the resource type column: every type in the backup with counts. */
export function KindList({
  kinds,
  total,
  selected,
  onSelect,
  loading,
}: {
  kinds: Kind[];
  total: number;
  selected: string;
  onSelect: (kindId: string) => void;
  loading: boolean;
}) {
  const [open, setOpen] = useState(true);
  const [filter, setFilter] = useState("");
  const [showAll, setShowAll] = useState(false);

  const matched = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    if (!needle) return kinds;
    return kinds.filter(
      (k) => k.kind.toLowerCase().includes(needle) || k.apiVersion.toLowerCase().includes(needle),
    );
  }, [kinds, filter]);

  // The console shows the common types and keeps the long tail behind a
  // "more" affordance, which matters here because a cluster has ~170 types.
  const collapsedCount = 24;
  const visible = showAll || filter ? matched : matched.slice(0, collapsedCount);
  const remaining = matched.length - visible.length;

  return (
    <section className="flex w-[252px] min-w-0 shrink-0 flex-col border-r border-line bg-surface">
      <CollapsibleHeader title="Resource Types" open={open} onToggle={() => setOpen((v) => !v)} />

      {open && (
        <>
          <div className="relative shrink-0 border-b border-line-soft px-3 py-2">
            <IconSearch className="pointer-events-none absolute top-1/2 left-5 h-3.5 w-3.5 -translate-y-1/2 text-muted" />
            <input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter types..."
              aria-label="Filter resource types"
              className="w-full rounded border border-line py-1 pr-2 pl-7 text-[13px] focus:border-brand focus:outline-none"
            />
          </div>

          <div className="min-h-0 flex-1 overflow-auto">
            {loading && <Spinner label="Reading snapshot" />}

            {!loading && (
              <ul>
                <Row
                  icon={<IconList className="h-4 w-4" />}
                  label="All Resources"
                  count={total}
                  active={selected === "*"}
                  onClick={() => onSelect("*")}
                />
                {visible.map((k) => (
                  <Row
                    key={k.id}
                    icon={<IconCube className="h-4 w-4" />}
                    label={k.label}
                    hint={k.group || undefined}
                    count={k.count}
                    active={selected === k.id}
                    onClick={() => onSelect(k.id)}
                  />
                ))}
              </ul>
            )}

            {!loading && matched.length === 0 && (
              <p className="px-4 py-6 text-center text-[13px] text-muted">No type matches "{filter}"</p>
            )}

            {!loading && remaining > 0 && (
              <button
                type="button"
                onClick={() => setShowAll(true)}
                className="w-full px-4 py-3 text-left text-[13px] font-medium text-brand hover:bg-brand-soft"
              >
                ... {countLabel(remaining)} more types
              </button>
            )}

            {!loading && showAll && !filter && (
              <button
                type="button"
                onClick={() => setShowAll(false)}
                className="w-full px-4 py-3 text-left text-[13px] font-medium text-brand hover:bg-brand-soft"
              >
                Show fewer types
              </button>
            )}
          </div>
        </>
      )}
    </section>
  );
}

function Row({
  icon,
  label,
  hint,
  count,
  active,
  onClick,
}: {
  icon: React.ReactNode;
  label: string;
  hint?: string;
  count: number;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <li>
      <button
        type="button"
        onClick={onClick}
        aria-current={active ? "true" : undefined}
        title={hint ? `${label} (${hint})` : label}
        className={`flex w-full items-center gap-2.5 border-l-[3px] py-2 pr-3 pl-[13px] text-left text-[13px] ${
          active
            ? "border-brand bg-brand-soft font-medium text-brand"
            : "border-transparent text-ink hover:bg-surface-alt"
        }`}
      >
        <span className={active ? "text-brand" : "text-muted"}>{icon}</span>
        <span className="min-w-0 flex-1 truncate">{label}</span>
        <span className={`shrink-0 text-[12px] tabular-nums ${active ? "text-brand" : "text-muted"}`}>
          {countLabel(count)}
        </span>
      </button>
    </li>
  );
}
