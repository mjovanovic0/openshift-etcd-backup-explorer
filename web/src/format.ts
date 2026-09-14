/** Formatting helpers shared by the panels. */

export function humanBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let v = n;
  for (const u of units) {
    v /= 1024;
    if (v < 1024) return `${v.toFixed(1)} ${u}`;
  }
  return `${v.toFixed(1)} PiB`;
}

export function countLabel(n: number): string {
  return n.toLocaleString("en-US");
}

export function shortTime(iso: string | null): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return `${d.toISOString().slice(0, 10)} ${d.toISOString().slice(11, 16)}`;
}

export function fullTime(iso: string | null): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toISOString().replace("T", " ").replace(".000Z", "Z");
}

/** relativeAge describes how long before the backup an object was created. */
export function relativeAge(iso: string | null, reference?: string): string {
  if (!iso) return "";
  const then = new Date(iso).getTime();
  const now = reference ? new Date(reference).getTime() : Date.now();
  if (Number.isNaN(then) || Number.isNaN(now)) return "";
  const secs = Math.max(0, Math.round((now - then) / 1000));
  const days = Math.floor(secs / 86400);
  if (days >= 365) return `${Math.floor(days / 365)}y`;
  if (days >= 1) return `${days}d`;
  const hours = Math.floor(secs / 3600);
  if (hours >= 1) return `${hours}h`;
  const mins = Math.floor(secs / 60);
  if (mins >= 1) return `${mins}m`;
  return `${secs}s`;
}

export const toneColor: Record<string, string> = {
  ok: "bg-ok",
  warn: "bg-warn",
  error: "bg-danger",
  info: "bg-info",
  neutral: "bg-muted",
};
