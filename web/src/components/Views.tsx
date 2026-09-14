import { useMemo, useState } from "react";
import { api } from "../api";
import type { BackupInfo, Kind, Namespace, StaticFile } from "../api";
import { useAsync } from "../hooks";
import { countLabel, fullTime, humanBytes } from "../format";
import { EmptyNote, ErrorNote, Panel, Pill, Spinner } from "./Chrome";
import { CodeView } from "./CodeView";
import { IconFile, IconFolder, IconSearch } from "./Icons";

/** NamespacesView lists every namespace so an operator can jump straight in. */
export function NamespacesView({
  namespaces,
  loading,
  onPick,
}: {
  namespaces: Namespace[];
  loading: boolean;
  onPick: (ns: string) => void;
}) {
  const [filter, setFilter] = useState("");
  const matched = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    return needle ? namespaces.filter((n) => n.name.toLowerCase().includes(needle)) : namespaces;
  }, [namespaces, filter]);

  return (
    <Panel
      className="flex-1"
      title="Namespaces"
      subtitle={`${countLabel(namespaces.length)} namespaces hold objects in this backup`}
      right={<FilterBox value={filter} onChange={setFilter} placeholder="Filter namespaces..." />}
    >
      {loading && <Spinner />}
      <ul className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3 p-4">
        {matched.map((ns) => (
          <li key={ns.name}>
            <button
              type="button"
              onClick={() => onPick(ns.name)}
              className="flex w-full items-center gap-3 rounded border border-line bg-surface px-3 py-3 text-left hover:border-brand hover:bg-brand-soft"
            >
              <IconFolder className="h-4 w-4 shrink-0 text-muted" />
              <span className="min-w-0 flex-1 truncate text-[13px] text-brand" title={ns.name}>
                {ns.name}
              </span>
              <span className="shrink-0 text-[12px] text-muted tabular-nums">{countLabel(ns.count)}</span>
            </button>
          </li>
        ))}
      </ul>
      {!loading && matched.length === 0 && <EmptyNote>No namespace matches "{filter}".</EmptyNote>}
    </Panel>
  );
}

/** TypesView is the full resource type table, with scope and storage format. */
export function TypesView({
  kinds,
  total,
  loading,
  onPick,
}: {
  kinds: Kind[];
  total: number;
  loading: boolean;
  onPick: (kindId: string) => void;
}) {
  const [filter, setFilter] = useState("");
  const matched = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    if (!needle) return kinds;
    return kinds.filter(
      (k) => k.kind.toLowerCase().includes(needle) || k.apiVersion.toLowerCase().includes(needle),
    );
  }, [kinds, filter]);

  return (
    <Panel
      className="flex-1"
      title="Resource Types"
      subtitle={`${countLabel(kinds.length)} types, ${countLabel(total)} objects`}
      right={<FilterBox value={filter} onChange={setFilter} placeholder="Filter types..." />}
    >
      {loading && <Spinner />}
      <table className="w-full border-separate border-spacing-0 text-[13px]">
        <thead className="sticky top-0 z-10 bg-surface">
          <tr>
            {["Kind", "API version", "Scope", "Go type", "Objects"].map((h) => (
              <th
                key={h}
                className={`border-b border-line px-4 py-2 font-medium text-ink ${
                  h === "Objects" ? "text-right" : "text-left"
                }`}
              >
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {matched.map((k) => (
            <tr key={k.id} className="cursor-pointer hover:bg-surface-alt" onClick={() => onPick(k.id)}>
              <td className="border-b border-line-soft px-4 py-2 text-brand">{k.kind}</td>
              <td className="border-b border-line-soft px-4 py-2 font-mono text-[12px] text-muted">{k.apiVersion}</td>
              <td className="border-b border-line-soft px-4 py-2 text-ink">
                {k.namespaced ? "Namespaced" : "Cluster"}
              </td>
              <td className="border-b border-line-soft px-4 py-2">
                {k.builtIn ? <Pill tone="blue">compiled in</Pill> : <Pill>custom resource</Pill>}
              </td>
              <td className="border-b border-line-soft px-4 py-2 text-right tabular-nums text-ink">
                {countLabel(k.count)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {!loading && matched.length === 0 && <EmptyNote>No type matches "{filter}".</EmptyNote>}
    </Panel>
  );
}

/** InfoView shows what etcd itself recorded, plus the static resources archive. */
export function InfoView({ backupId, fontSize }: { backupId: string; fontSize: number }) {
  const { data, error, loading } = useAsync<BackupInfo>((s) => api.info(backupId, s), [backupId]);

  return (
    <div className="flex min-h-0 flex-1">
      <Panel className="min-w-[420px] flex-1 border-r border-line" title="Backup Info" subtitle={backupId}>
        {loading && <Spinner />}
        {error && <ErrorNote message={error} />}
        {data && (
          <div className="space-y-6 p-4">
            <Section title="Backup files">
              <Row label="Snapshot" mono>
                {data.snapshot.file}
              </Row>
              <Row label="Static resources" mono>
                {data.backup.staticPath || "(none)"}
              </Row>
              <Row label="Taken at">{fullTime(data.backup.takenAt)}</Row>
              <Row label="Snapshot size">{humanBytes(data.snapshot.size)}</Row>
              <Row label="Indexed in">{data.indexedIn}</Row>
            </Section>

            <Section title="etcd keyspace">
              <Row label="etcd version">{data.snapshot.clusterVersion}</Row>
              <Row label="Revision">{countLabel(data.snapshot.revision)}</Row>
              <Row label="Compact revision">{countLabel(data.snapshot.compactRevision)}</Row>
              <Row label="Consistent index">{countLabel(data.snapshot.consistentIndex)}</Row>
              <Row label="Keys stored">{countLabel(data.snapshot.totalKeys)}</Row>
              <Row label="Live keys">{countLabel(data.snapshot.liveKeys)}</Row>
              <Row label="Tombstones">{countLabel(data.snapshot.tombstones)}</Row>
              <Row label="Leases">{countLabel(data.snapshot.leases)}</Row>
            </Section>

            <Section title="Cluster contents">
              <Row label="Kubernetes objects">{countLabel(data.resources)}</Row>
              <Row label="Resource types">{countLabel(data.kinds)}</Row>
              <Row label="Namespaces">{countLabel(data.namespaces)}</Row>
              <Row label="Static files">{countLabel(data.staticFiles)}</Row>
            </Section>

            <Section title="etcd members">
              <ul className="space-y-2">
                {data.snapshot.members.map((m) => (
                  <li key={m.id} className="rounded border border-line-soft px-3 py-2">
                    <div className="flex items-center gap-2">
                      <span className="text-[13px] text-ink">{m.name}</span>
                      {m.removed && <Pill>removed</Pill>}
                    </div>
                    <div className="mt-0.5 font-mono text-[11px] text-muted">id {m.id}</div>
                    {m.peerURLs && m.peerURLs.length > 0 && (
                      <div className="font-mono text-[11px] text-muted">{m.peerURLs.join(", ")}</div>
                    )}
                  </li>
                ))}
              </ul>
            </Section>
          </div>
        )}
      </Panel>

      <StaticResources backupId={backupId} fontSize={fontSize} />
    </div>
  );
}

/** StaticResources browses the static_kuberesources tarball beside the snapshot. */
function StaticResources({ backupId, fontSize }: { backupId: string; fontSize: number }) {
  const [selected, setSelected] = useState<string | null>(null);
  const [filter, setFilter] = useState("");
  const files = useAsync<StaticFile[]>((s) => api.staticFiles(backupId, s), [backupId]);
  const content = useAsync(
    (s) => api.staticFile(backupId, selected!, s),
    [backupId, selected],
    selected !== null,
  );

  const matched = useMemo(() => {
    const all = (files.data ?? []).filter((f) => !f.isDir);
    const needle = filter.trim().toLowerCase();
    return needle ? all.filter((f) => f.path.toLowerCase().includes(needle)) : all;
  }, [files.data, filter]);

  return (
    <div className="flex min-h-0 w-[640px] shrink-0 flex-col">
      <Panel
        className="min-h-0 flex-1"
        title="Static pod resources"
        subtitle={`${countLabel(matched.length)} files in the archive taken with this snapshot`}
        right={<FilterBox value={filter} onChange={setFilter} placeholder="Filter files..." />}
        bodyClassName="flex flex-col"
      >
        {files.loading && <Spinner />}
        {files.error && <ErrorNote message={files.error} />}
        {files.data && files.data.length === 0 && (
          <EmptyNote>This backup has no static resources archive.</EmptyNote>
        )}

        <div className="min-h-0 flex-1 overflow-auto">
          <ul className="divide-y divide-line-soft">
            {matched.map((f) => (
              <li key={f.path}>
                <button
                  type="button"
                  onClick={() => setSelected(f.path)}
                  className={`flex w-full items-center gap-2 px-4 py-2 text-left ${
                    selected === f.path ? "bg-brand-soft" : "hover:bg-surface-alt"
                  }`}
                >
                  <IconFile className="h-3.5 w-3.5 shrink-0 text-muted" />
                  <span className="flex min-w-0 flex-1 font-mono text-[12px]" title={f.path}>
                    <span className="truncate text-muted">{dirOf(f.path)}</span>
                    <span className={`shrink-0 ${selected === f.path ? "text-brand" : "text-ink"}`}>
                      {baseOf(f.path)}
                    </span>
                  </span>
                  <span className="shrink-0 text-[11px] text-muted">{humanBytes(f.size)}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>

        {selected && (
          <div className="flex min-h-0 flex-[1.4] flex-col border-t border-line">
            <div className="flex shrink-0 items-center justify-between gap-2 bg-surface-alt px-4 py-2">
              <span className="min-w-0 truncate font-mono text-[12px] text-ink" title={selected}>
                {selected.split("/").pop()}
              </span>
              <button
                type="button"
                onClick={() => setSelected(null)}
                className="shrink-0 text-[12px] text-brand hover:underline"
              >
                Close
              </button>
            </div>
            {content.loading && <Spinner />}
            {content.error && <ErrorNote message={content.error} />}
            {content.data && (
              <CodeView
                text={content.data.text}
                language={content.data.path.endsWith(".json") ? "json" : "yaml"}
                fontSize={fontSize}
              />
            )}
          </div>
        )}
      </Panel>
    </div>
  );
}

/** SettingsView holds the few preferences that change how the browser reads. */
export function SettingsView({
  pageSize,
  onPageSize,
  fontSize,
  onFontSize,
  deepDefault,
  onDeepDefault,
  clean,
  onClean,
}: {
  pageSize: number;
  onPageSize: (n: number) => void;
  fontSize: number;
  onFontSize: (n: number) => void;
  deepDefault: boolean;
  onDeepDefault: (v: boolean) => void;
  clean: boolean;
  onClean: (v: boolean) => void;
}) {
  return (
    <Panel className="flex-1" title="Settings" subtitle="Stored in this browser only">
      <div className="max-w-[620px] space-y-6 p-4">
        <Setting
          label="Remove generated fields when copying or exporting"
          hint="Takes out metadata.uid, metadata.creationTimestamp and managedFields, plus managedFields and empty timestamps inside nested templates. What you see in the YAML and JSON tabs is always the object as the backup holds it, this only changes what leaves the browser. Downloading the raw etcd value is never changed."
        >
          <label className="flex cursor-pointer items-center gap-2 text-[13px]">
            <input type="checkbox" checked={clean} onChange={(e) => onClean(e.target.checked)} />
            Enabled
          </label>
        </Setting>

        <Setting
          label="Rows per page"
          hint="How many objects the resource list asks for at a time. Each row needs its object decoded for the status column, so a smaller page responds faster."
        >
          <select
            value={pageSize}
            onChange={(e) => onPageSize(Number(e.target.value))}
            className="rounded border border-line bg-surface px-2 py-1.5 text-[13px] focus:border-brand focus:outline-none"
          >
            {[20, 50, 100, 200].map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </Setting>

        <Setting label="Manifest font size" hint="Applies to the YAML, JSON and static file viewers.">
          <div className="flex items-center gap-3">
            <input
              type="range"
              min={10}
              max={18}
              step={1}
              value={fontSize}
              onChange={(e) => onFontSize(Number(e.target.value))}
              className="w-48"
            />
            <span className="w-12 text-[13px] tabular-nums text-muted">{fontSize}px</span>
          </div>
        </Setting>

        <Setting
          label="Search inside object contents by default"
          hint="A content search reads every candidate object out of the snapshot, so it is slower than matching names. Leave this off unless you often search for values."
        >
          <label className="flex cursor-pointer items-center gap-2 text-[13px]">
            <input type="checkbox" checked={deepDefault} onChange={(e) => onDeepDefault(e.target.checked)} />
            Enabled
          </label>
        </Setting>

        <div className="rounded border border-line-soft bg-surface-alt px-3 py-3 text-[12px] text-muted">
          This tool opens the snapshot file read only and never writes to it. Secrets in an etcd backup are stored
          unencrypted unless the cluster enabled encryption at rest, so treat anything you read here as sensitive.
        </div>
      </div>
    </Panel>
  );
}

function Setting({
  label,
  hint,
  children,
}: {
  label: string;
  hint: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <h3 className="text-[14px] font-medium text-ink">{label}</h3>
      <p className="mt-0.5 mb-2 text-[12px] text-muted">{hint}</p>
      {children}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h3 className="mb-2 border-b border-line-soft pb-1 text-[13px] font-medium text-ink">{title}</h3>
      <dl className="space-y-1">{children}</dl>
    </section>
  );
}

function Row({ label, children, mono }: { label: string; children: React.ReactNode; mono?: boolean }) {
  return (
    <div className="flex gap-3 text-[13px]">
      <dt className="w-[150px] shrink-0 text-muted">{label}</dt>
      <dd className={`min-w-0 flex-1 break-all text-ink ${mono ? "font-mono text-[12px]" : "tabular-nums"}`}>
        {children}
      </dd>
    </div>
  );
}

function FilterBox({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
}) {
  return (
    <div className="relative w-[240px] shrink-0">
      <IconSearch className="pointer-events-none absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2 text-muted" />
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        className="w-full rounded border border-line py-1.5 pr-2 pl-8 text-[13px] focus:border-brand focus:outline-none"
      />
    </div>
  );
}

/** dirOf and baseOf split an archive path so the file name always stays
 * visible and only the directory is truncated. */
function dirOf(path: string): string {
  const i = path.lastIndexOf("/");
  return i < 0 ? "" : path.slice(0, i + 1);
}

function baseOf(path: string): string {
  const i = path.lastIndexOf("/");
  return i < 0 ? path : path.slice(i + 1);
}
