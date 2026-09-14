import type { ExportFilters, Namespace, Resource, ResourceList as ResourceListData } from "../api";
import { ExportMenu } from "./ExportMenu";
import { countLabel, shortTime } from "../format";
import { EmptyNote, ErrorNote, Panel, Spinner, StatusDot } from "./Chrome";
import { IconArrowLeft, IconArrowRight, IconSearch } from "./Icons";

export type SortState = { field: string; desc: boolean };

/** ResourceList is the middle column: one page of objects of the chosen type. */
export function ResourceList({
  title,
  data,
  loading,
  error,
  namespaces,
  namespace,
  onNamespaceChange,
  filter,
  onFilterChange,
  deep,
  onDeepChange,
  sort,
  onSortChange,
  page,
  pageSize,
  onPageChange,
  selectedId,
  onSelect,
  showKindColumn,
  backupId,
  exportFilters,
  exportLabel,
  clean,
}: {
  title: string;
  data: ResourceListData | null;
  loading: boolean;
  error: string | null;
  namespaces: Namespace[];
  namespace: string;
  onNamespaceChange: (ns: string) => void;
  filter: string;
  onFilterChange: (v: string) => void;
  deep: boolean;
  onDeepChange: (v: boolean) => void;
  sort: SortState;
  onSortChange: (s: SortState) => void;
  page: number;
  pageSize: number;
  onPageChange: (p: number) => void;
  selectedId: string | null;
  onSelect: (r: Resource) => void;
  showKindColumn: boolean;
  backupId: string;
  exportFilters: ExportFilters;
  exportLabel: string;
  clean: boolean;
}) {
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <Panel
      className="min-w-[380px] flex-1 border-r border-line"
      title={title}
      subtitle={loading && !data ? "Loading" : `${countLabel(total)} items`}
      right={
        <ExportMenu
          backupId={backupId}
          filters={exportFilters}
          total={total}
          label={exportLabel}
          clean={clean}
        />
      }
      bodyClassName="flex flex-col"
    >
      <div className="flex shrink-0 flex-wrap items-center gap-2 border-b border-line-soft px-4 py-3">
        <select
          value={namespace}
          onChange={(e) => onNamespaceChange(e.target.value)}
          aria-label="Namespace"
          className="max-w-[220px] rounded border border-line bg-surface px-2 py-1.5 text-[13px] focus:border-brand focus:outline-none"
        >
          <option value="">All namespaces</option>
          {namespaces.map((ns) => (
            <option key={ns.name} value={ns.name}>
              {ns.name} ({ns.count})
            </option>
          ))}
        </select>

        <div className="relative min-w-[180px] flex-1">
          <IconSearch className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-muted" />
          <input
            value={filter}
            onChange={(e) => onFilterChange(e.target.value)}
            placeholder="Filter by name..."
            aria-label="Filter by name"
            className="w-full rounded border border-line py-1.5 pr-2 pl-8 text-[13px] focus:border-brand focus:outline-none"
          />
        </div>

        <label
          className="flex shrink-0 cursor-pointer items-center gap-1.5 text-[12px] text-muted"
          title="Also search inside the stored object bodies. This reads every matching object, so it is slower."
        >
          <input type="checkbox" checked={deep} onChange={(e) => onDeepChange(e.target.checked)} />
          Search contents
        </label>
      </div>

      {error && <ErrorNote message={error} />}

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full min-w-[540px] border-separate border-spacing-0 text-[13px]">
          <thead className="sticky top-0 z-10 bg-surface">
            <tr className="border-b border-line">
              <HeadCell label="Name" field="name" sort={sort} onSortChange={onSortChange} />
              {showKindColumn && <HeadCell label="Kind" field="kind" sort={sort} onSortChange={onSortChange} />}
              <HeadCell label="Namespace" field="namespace" sort={sort} onSortChange={onSortChange} />
              <th className="border-b border-line px-3 py-2 text-left font-medium text-ink">Status</th>
              <HeadCell label="Created At" field="created" sort={sort} onSortChange={onSortChange} />
            </tr>
          </thead>
          <tbody>
            {data?.items.map((r) => {
              const active = r.id === selectedId;
              return (
                <tr
                  key={r.id}
                  onClick={() => onSelect(r)}
                  className={`cursor-pointer ${active ? "bg-brand-soft" : "hover:bg-surface-alt"}`}
                >
                  <td className="max-w-[200px] border-b border-line-soft px-3 py-2">
                    <span
                      className={`block truncate ${active ? "font-medium text-brand" : "text-brand"}`}
                      title={r.name}
                    >
                      {r.name}
                    </span>
                  </td>
                  {showKindColumn && (
                    <td
                      className="max-w-[130px] border-b border-line-soft px-3 py-2 text-muted"
                      title={r.apiVersion}
                    >
                      <span className="block truncate">{r.kind}</span>
                    </td>
                  )}
                  <td className="max-w-[140px] border-b border-line-soft px-3 py-2">
                    <span className="block truncate text-ink" title={r.namespace}>
                      {r.namespace || <span className="text-muted">-</span>}
                    </span>
                  </td>
                  <td className="max-w-[120px] border-b border-line-soft px-3 py-2">
                    {r.status?.text ? (
                      <span className="flex items-center gap-1.5" title={r.status.text}>
                        <StatusDot tone={r.status.tone} />
                        <span className="truncate">{r.status.text}</span>
                      </span>
                    ) : (
                      <span className="text-muted">-</span>
                    )}
                  </td>
                  <td className="border-b border-line-soft px-3 py-2 whitespace-nowrap text-muted">
                    {shortTime(r.createdAt)}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>

        {loading && !data && <Spinner />}
        {data && data.items.length === 0 && !loading && (
          <EmptyNote>
            No objects match this filter.
            {deep ? "" : " Try enabling Search contents to look inside the objects."}
          </EmptyNote>
        )}
      </div>

      {pages > 1 && (
        <Pagination page={page} pages={pages} onChange={onPageChange} total={total} pageSize={pageSize} />
      )}
    </Panel>
  );
}

function HeadCell({
  label,
  field,
  sort,
  onSortChange,
}: {
  label: string;
  field: string;
  sort: SortState;
  onSortChange: (s: SortState) => void;
}) {
  const active = sort.field === field;
  return (
    <th className="border-b border-line px-3 py-2 text-left font-medium text-ink">
      <button
        type="button"
        onClick={() => onSortChange({ field, desc: active ? !sort.desc : false })}
        className="flex items-center gap-1 hover:text-brand"
      >
        {label}
        <span className={`text-[10px] ${active ? "text-brand" : "text-transparent"}`}>
          {active && sort.desc ? "▼" : "▲"}
        </span>
      </button>
    </th>
  );
}

function Pagination({
  page,
  pages,
  onChange,
  total,
  pageSize,
}: {
  page: number;
  pages: number;
  onChange: (p: number) => void;
  total: number;
  pageSize: number;
}) {
  const from = page * pageSize + 1;
  const to = Math.min(total, (page + 1) * pageSize);

  return (
    <nav
      aria-label="Pagination"
      className="flex shrink-0 items-center justify-center gap-1 border-t border-line-soft px-4 py-3"
    >
      <span className="mr-auto text-[12px] text-muted">
        {countLabel(from)} to {countLabel(to)} of {countLabel(total)}
      </span>

      <PageButton disabled={page === 0} onClick={() => onChange(page - 1)} label="Previous page">
        <IconArrowLeft className="h-3.5 w-3.5" />
      </PageButton>

      {pageNumbers(page, pages).map((p, i) =>
        p === null ? (
          <span key={`gap-${i}`} className="px-1 text-muted">
            ...
          </span>
        ) : (
          <button
            key={p}
            type="button"
            onClick={() => onChange(p)}
            aria-current={p === page ? "page" : undefined}
            className={`min-w-7 rounded px-2 py-1 text-[13px] tabular-nums ${
              p === page ? "bg-brand font-medium text-white" : "text-ink hover:bg-surface-alt"
            }`}
          >
            {p + 1}
          </button>
        ),
      )}

      <PageButton disabled={page >= pages - 1} onClick={() => onChange(page + 1)} label="Next page">
        <IconArrowRight className="h-3.5 w-3.5" />
      </PageButton>
    </nav>
  );
}

function PageButton({
  disabled,
  onClick,
  label,
  children,
}: {
  disabled: boolean;
  onClick: () => void;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      className="rounded p-1.5 text-muted enabled:hover:bg-surface-alt enabled:hover:text-ink disabled:opacity-30"
    >
      {children}
    </button>
  );
}

/** pageNumbers builds the 1 2 3 ... 63 style page strip. */
function pageNumbers(page: number, pages: number): (number | null)[] {
  if (pages <= 7) return Array.from({ length: pages }, (_, i) => i);
  const out: (number | null)[] = [];
  const window = new Set([0, pages - 1, page - 1, page, page + 1]);
  if (page <= 2) [1, 2, 3, 4].forEach((p) => window.add(p));
  if (page >= pages - 3) [pages - 2, pages - 3, pages - 4, pages - 5].forEach((p) => window.add(p));
  const sorted = [...window].filter((p) => p >= 0 && p < pages).sort((a, b) => a - b);
  let prev = -1;
  for (const p of sorted) {
    if (prev >= 0 && p - prev > 1) out.push(null);
    out.push(p);
    prev = p;
  }
  return out;
}
