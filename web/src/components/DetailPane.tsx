import { useEffect, useRef, useState } from "react";
import { api } from "../api";
import type { Resource, ResourceDetail } from "../api";
import { copyText } from "../clipboard";
import { fullTime, humanBytes, relativeAge, shortTime } from "../format";
import { EmptyNote, ErrorNote, Pill, Spinner, StatusDot } from "./Chrome";
import { CodeView } from "./CodeView";
import { useToast } from "./Toast";
import { IconChevron, IconCopy, IconCube, IconDownload } from "./Icons";

type Tab = "yaml" | "json" | "details" | "related";

const tabs: { id: Tab; label: string }[] = [
  { id: "yaml", label: "YAML" },
  { id: "json", label: "JSON" },
  { id: "details", label: "Details" },
  { id: "related", label: "Related Resources" },
];

/** DetailPane is the right column: one object, shown four ways. */
export function DetailPane({
  backupId,
  detail,
  loading,
  error,
  takenAt,
  fontSize,
  clean,
  onOpenResource,
}: {
  backupId: string;
  detail: ResourceDetail | null;
  loading: boolean;
  error: string | null;
  takenAt: string | null;
  fontSize: number;
  clean: boolean;
  onOpenResource: (r: Resource) => void;
}) {
  const [tab, setTab] = useState<Tab>("yaml");

  if (error) {
    return (
      <section className="flex w-[300px] shrink-0 flex-col bg-surface">
        <ErrorNote message={error} />
      </section>
    );
  }
  if (!detail) {
    // Nothing is selected yet, so the pane stays narrow and gives the list room.
    return (
      <section className="flex w-[300px] shrink-0 flex-col bg-surface">
        {loading ? <Spinner label="Decoding object" /> : <EmptyNote>Select a resource to inspect it.</EmptyNote>}
      </section>
    );
  }

  const r = detail.resource;

  return (
    <section className="flex w-[520px] min-w-0 shrink-0 flex-col bg-surface">
      <header className="shrink-0 border-b border-line-soft px-4 py-3">
        <div className="flex items-start gap-2">
          <IconCube className="mt-1 h-5 w-5 shrink-0 text-muted" />
          <h2 className="min-w-0 flex-1 text-[19px] leading-tight font-medium break-all text-ink">{r.name}</h2>
          <CopyButton
            backupId={backupId}
            resource={r}
            format={tab === "json" ? "json" : "yaml"}
            clean={clean}
          />
          <DownloadMenu backupId={backupId} resourceId={r.id} clean={clean} />
        </div>

        <dl className="mt-3 space-y-1 text-[13px]">
          <Field label={r.kind}>
            {detail.status?.text ? (
              <span className="flex items-center gap-1.5">
                <StatusDot tone={detail.status.tone} />
                {detail.status.text}
              </span>
            ) : (
              <span className="text-muted">-</span>
            )}
          </Field>
          {r.namespace && <Field label="Namespace">{r.namespace}</Field>}
          {!r.namespace && <Field label="Scope">Cluster</Field>}
          <Field label="Created">
            {shortTime(r.createdAt)}
            {r.createdAt && takenAt && (
              <span className="ml-1.5 text-muted">({relativeAge(r.createdAt, takenAt)} before backup)</span>
            )}
          </Field>
          {r.uid && (
            <Field label="UID">
              <span className="font-mono text-[12px] break-all">{r.uid}</span>
            </Field>
          )}
        </dl>

        {!detail.object.decoded && detail.object.reason && (
          <p className="mt-3 rounded border border-warn/40 bg-warn/10 px-2.5 py-2 text-[12px] text-ink">
            {detail.object.reason}
          </p>
        )}
      </header>

      <nav className="flex shrink-0 gap-1 border-b border-line px-2" role="tablist">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
            className={`border-b-2 px-3 py-2.5 text-[14px] ${
              tab === t.id
                ? "border-brand font-medium text-brand"
                : "border-transparent text-ink hover:border-line hover:text-brand"
            }`}
          >
            {t.label}
          </button>
        ))}
      </nav>

      {tab === "yaml" && <CodeView text={detail.object.yaml} language="yaml" fontSize={fontSize} />}
      {tab === "json" && (
        <CodeView text={JSON.stringify(detail.object.json, null, 2)} language="json" fontSize={fontSize} />
      )}
      {tab === "details" && <DetailsTab detail={detail} />}
      {tab === "related" && <RelatedTab detail={detail} onOpenResource={onOpenResource} />}
    </section>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-2">
      <dt className="w-[92px] shrink-0 text-muted">{label}:</dt>
      <dd className="min-w-0 flex-1 break-words text-ink">{children}</dd>
    </div>
  );
}

function DetailsTab({ detail }: { detail: ResourceDetail }) {
  const r = detail.resource;
  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <dl className="divide-y divide-line-soft">
        {detail.details.map((f, i) => (
          <div key={`${f.label}-${i}`} className="flex gap-3 px-4 py-2 text-[13px]">
            <dt
              className={`w-[150px] shrink-0 break-all text-muted ${f.label.startsWith("  ") ? "pl-3" : ""}`}
            >
              {f.label.trim()}
            </dt>
            <dd className="min-w-0 flex-1 break-all text-ink">
              {f.label === "etcd key" || f.label === "UID" ? (
                <span className="font-mono text-[12px]">{f.value}</span>
              ) : (
                f.value
              )}
            </dd>
          </div>
        ))}
      </dl>

      <div className="border-t border-line-soft px-4 py-3">
        <h3 className="mb-2 text-[13px] font-medium text-ink">How this object is stored</h3>
        <div className="flex flex-wrap gap-2">
          <Pill tone={r.encoding === "protobuf" ? "blue" : "green"}>{r.encoding}</Pill>
          <Pill>{humanBytes(r.size)}</Pill>
          <Pill>revision {r.modRevision.toLocaleString("en-US")}</Pill>
        </div>
        <p className="mt-2 text-[12px] text-muted">
          {r.encoding === "protobuf"
            ? "The API server wrote this object with the Kubernetes protobuf serializer, which is used for built in and OpenShift types."
            : "Custom resources are stored as JSON, so this object is shown exactly as etcd holds it."}
        </p>
      </div>

      {detail.owners && detail.owners.length > 0 && (
        <div className="border-t border-line-soft px-4 py-3">
          <h3 className="mb-2 text-[13px] font-medium text-ink">Owner references</h3>
          <ul className="space-y-1.5 text-[13px]">
            {detail.owners.map((o) => (
              <li key={o.uid} className="flex flex-wrap items-center gap-2">
                <span className="text-muted">{o.kind}</span>
                <span className="text-ink">{o.name}</span>
                {o.controller && <Pill tone="blue">controller</Pill>}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function RelatedTab({
  detail,
  onOpenResource,
}: {
  detail: ResourceDetail;
  onOpenResource: (r: Resource) => void;
}) {
  const related = detail.related ?? [];
  if (related.length === 0) {
    return <EmptyNote>No owners, owned objects or namespace found for this resource.</EmptyNote>;
  }
  return (
    <div className="min-h-0 flex-1 overflow-auto">
      <p className="border-b border-line-soft px-4 py-2 text-[12px] text-muted">
        Objects connected through owner references, plus the namespace this object lives in.
      </p>
      <ul className="divide-y divide-line-soft">
        {related.map((r) => (
          <li key={r.id}>
            <button
              type="button"
              onClick={() => onOpenResource(r)}
              className="flex w-full items-center gap-3 px-4 py-2.5 text-left hover:bg-surface-alt"
            >
              <IconCube className="h-4 w-4 shrink-0 text-muted" />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-[13px] text-brand">{r.name}</span>
                <span className="block truncate text-[11px] text-muted">
                  {r.kind}
                  {r.namespace ? ` in ${r.namespace}` : " (cluster scoped)"}
                </span>
              </span>
              <span className="shrink-0 text-[11px] text-muted">{fullTime(r.createdAt).slice(0, 10)}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

function DownloadMenu({
  backupId,
  resourceId,
  clean,
}: {
  backupId: string;
  resourceId: string;
  clean: boolean;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const formats: { id: "yaml" | "json" | "raw"; label: string; hint?: string }[] = [
    { id: "yaml", label: "Download YAML", hint: clean ? "generated fields removed" : undefined },
    { id: "json", label: "Download JSON", hint: clean ? "generated fields removed" : undefined },
    { id: "raw", label: "Download raw etcd value", hint: "exactly as the snapshot stores it" },
  ];

  return (
    <div ref={ref} className="relative shrink-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex items-center gap-1.5 rounded border border-line bg-surface px-2.5 py-1.5 text-[13px] text-ink hover:border-brand hover:text-brand"
      >
        <IconDownload className="h-3.5 w-3.5" />
        Download
        <IconChevron className={`h-3.5 w-3.5 transition-transform ${open ? "rotate-180" : ""}`} />
      </button>
      {open && (
        <ul className="absolute top-full right-0 z-30 mt-1 w-[210px] overflow-hidden rounded border border-line bg-surface py-1 shadow-lg">
          {formats.map((f) => (
            <li key={f.id}>
              <a
                href={api.downloadURL(backupId, resourceId, f.id, clean)}
                onClick={() => setOpen(false)}
                className="block px-3 py-2 text-[13px] text-ink hover:bg-brand-soft hover:text-brand"
              >
                <span className="block">{f.label}</span>
                {f.hint && <span className="block text-[11px] text-muted">{f.hint}</span>}
              </a>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** CopyButton copies the object in whichever format the open tab shows. */
function CopyButton({
  backupId,
  resource,
  format,
  clean,
}: {
  backupId: string;
  resource: Resource;
  format: "yaml" | "json";
  clean: boolean;
}) {
  const notify = useToast();
  const [busy, setBusy] = useState(false);

  const copy = async () => {
    setBusy(true);
    try {
      const text = await api.fetchText(api.downloadURL(backupId, resource.id, format, clean));
      await copyText(text);
      notify(
        `Copied ${resource.kind} ${resource.name} as ${format.toUpperCase()}` +
          (clean ? ", generated fields removed" : ""),
      );
    } catch (err) {
      notify(`Could not copy: ${err instanceof Error ? err.message : String(err)}`, "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <button
      type="button"
      onClick={copy}
      disabled={busy}
      title={`Copy this object as ${format.toUpperCase()}`}
      className="flex shrink-0 items-center gap-1.5 rounded border border-line bg-surface px-2.5 py-1.5 text-[13px] text-ink hover:border-brand hover:text-brand disabled:opacity-50"
    >
      {busy ? (
        <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-line border-t-brand" />
      ) : (
        <IconCopy className="h-3.5 w-3.5" />
      )}
      Copy
    </button>
  );
}
