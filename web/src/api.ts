// Types mirror the Go DTOs in internal/api.

export type Backup = {
  id: string;
  label: string;
  takenAt: string;
  size: number;
  snapshotPath: string;
  staticPath?: string;
  loaded: boolean;
  resources: number;
};

export type Kind = {
  id: string;
  kind: string;
  label: string;
  apiVersion: string;
  group: string;
  version: string;
  count: number;
  namespaced: boolean;
  builtIn: boolean;
};

export type KindsResponse = { total: number; items: Kind[] };

export type Namespace = { name: string; count: number };

export type Status = { text: string; tone: "ok" | "warn" | "error" | "info" | "neutral" | "" };

export type Resource = {
  id: string;
  key: string;
  apiVersion: string;
  kind: string;
  kindId: string;
  group: string;
  version: string;
  namespace: string;
  name: string;
  uid: string;
  createdAt: string | null;
  encoding: "protobuf" | "json" | "unknown";
  size: number;
  modRevision: number;
  status?: Status;
};

export type ResourceList = { total: number; offset: number; limit: number; items: Resource[] };

export type KubeObject = {
  apiVersion: string;
  kind: string;
  encoding: string;
  json: unknown;
  yaml: string;
  decoded: boolean;
  reason?: string;
};

export type OwnerRef = {
  apiVersion: string;
  kind: string;
  name: string;
  uid: string;
  controller: boolean;
};

export type DetailField = { label: string; value: string };

export type ResourceDetail = {
  resource: Resource;
  status?: Status;
  object: KubeObject;
  owners?: OwnerRef[];
  related?: Resource[];
  details: DetailField[];
};

export type Member = {
  id: string;
  name: string;
  peerURLs?: string[];
  clientURLs?: string[];
  removed?: boolean;
};

export type SnapshotInfo = {
  file: string;
  size: number;
  modTime: string;
  clusterVersion: string;
  revision: number;
  compactRevision: number;
  consistentIndex: number;
  totalKeys: number;
  liveKeys: number;
  tombstones: number;
  leases: number;
  members: Member[];
};

export type BackupInfo = {
  backup: Backup;
  snapshot: SnapshotInfo;
  resources: number;
  kinds: number;
  namespaces: number;
  staticFiles: number;
  indexedIn: string;
};

export type ExportFormat = "yaml" | "json" | "zip";

/** ExportFilters mirrors the resource list filters, so an export contains
 * exactly the rows on screen. */
export type ExportFilters = {
  kind?: string;
  namespace?: string;
  q?: string;
  deep?: boolean;
};

export type StaticFile = { path: string; size: number; mode: string; isDir: boolean };
export type StaticFileContent = { path: string; size: number; mode: string; text: string; binary: boolean };

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(path, { signal });
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // The response was not JSON, so keep the status line.
    }
    throw new Error(message);
  }
  return (await res.json()) as T;
}

export const api = {
  backups: (signal?: AbortSignal) => get<Backup[]>("/api/backups", signal),

  info: (backup: string, signal?: AbortSignal) =>
    get<BackupInfo>(`/api/backups/${backup}/info`, signal),

  kinds: (backup: string, signal?: AbortSignal) =>
    get<KindsResponse>(`/api/backups/${backup}/kinds`, signal),

  namespaces: (backup: string, signal?: AbortSignal) =>
    get<Namespace[]>(`/api/backups/${backup}/namespaces`, signal),

  resources: (
    backup: string,
    params: {
      kind?: string;
      namespace?: string;
      q?: string;
      deep?: boolean;
      sort?: string;
      order?: string;
      offset?: number;
      limit?: number;
      status?: boolean;
    },
    signal?: AbortSignal,
  ) => {
    const qs = new URLSearchParams();
    if (params.kind) qs.set("kind", params.kind);
    if (params.namespace) qs.set("namespace", params.namespace);
    if (params.q) qs.set("q", params.q);
    if (params.deep) qs.set("deep", "true");
    if (params.sort) qs.set("sort", params.sort);
    if (params.order) qs.set("order", params.order);
    if (params.offset) qs.set("offset", String(params.offset));
    if (params.limit) qs.set("limit", String(params.limit));
    if (params.status === false) qs.set("status", "false");
    return get<ResourceList>(`/api/backups/${backup}/resources?${qs}`, signal);
  },

  resource: (backup: string, id: string, signal?: AbortSignal) =>
    get<ResourceDetail>(`/api/backups/${backup}/resources/${id}`, signal),

  downloadURL: (backup: string, id: string, format: "yaml" | "json" | "raw", clean = true) =>
    `/api/backups/${backup}/resources/${id}/download?format=${format}&clean=${clean}`,

  exportURL: (backup: string, filters: ExportFilters, format: ExportFormat, clean: boolean) => {
    const qs = new URLSearchParams({ format, clean: String(clean) });
    if (filters.kind) qs.set("kind", filters.kind);
    if (filters.namespace) qs.set("namespace", filters.namespace);
    if (filters.q) qs.set("q", filters.q);
    if (filters.deep) qs.set("deep", "true");
    return `/api/backups/${backup}/export?${qs}`;
  },

  /** fetchText reads an export or download as text, for the clipboard. */
  fetchText: async (url: string, signal?: AbortSignal): Promise<string> => {
    const res = await fetch(url, { signal });
    if (!res.ok) {
      let message = `${res.status} ${res.statusText}`;
      try {
        const body = (await res.json()) as { error?: string };
        if (body.error) message = body.error;
      } catch {
        // Not JSON, so keep the status line.
      }
      throw new Error(message);
    }
    return res.text();
  },

  staticFiles: (backup: string, signal?: AbortSignal) =>
    get<StaticFile[]>(`/api/backups/${backup}/static`, signal),

  staticFile: (backup: string, path: string, signal?: AbortSignal) =>
    get<StaticFileContent>(`/api/backups/${backup}/static/file?path=${encodeURIComponent(path)}`, signal),
};
