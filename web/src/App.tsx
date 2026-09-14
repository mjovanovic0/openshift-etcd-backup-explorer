import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "./api";
import type { Backup, Kind, Namespace, Resource } from "./api";
import { useAsync, useDebounced, useLocalSetting } from "./hooks";
import { ErrorNote, Spinner } from "./components/Chrome";
import { TopBar } from "./components/TopBar";
import { SideNav } from "./components/SideNav";
import type { View } from "./components/SideNav";
import { KindList } from "./components/KindList";
import { ResourceList } from "./components/ResourceList";
import type { SortState } from "./components/ResourceList";
import { DetailPane } from "./components/DetailPane";
import { InfoView, NamespacesView, SettingsView, TypesView } from "./components/Views";
import { countLabel } from "./format";

export function App() {
  const [view, setView] = useState<View>("browse");
  const [backupId, setBackupId] = useState<string | null>(null);

  // The browser opens on Pods, the way the console does, because the
  // unfiltered keyspace is mostly machine generated names.
  const [kindId, setKindId] = useState("*");
  const [kindPicked, setKindPicked] = useState(false);
  const [namespace, setNamespace] = useState("");
  const [nameFilter, setNameFilter] = useState("");
  const [globalSearch, setGlobalSearch] = useState("");
  const [sort, setSort] = useState<SortState>({ field: "name", desc: false });
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<Resource | null>(null);

  const [pageSize, setPageSize] = useLocalSetting("etcdBrowser.pageSize", 50);
  const [fontSize, setFontSize] = useLocalSetting("etcdBrowser.fontSize", 12);
  const [deepDefault, setDeepDefault] = useLocalSetting("etcdBrowser.deepSearch", false);
  const [clean, setClean] = useLocalSetting("etcdBrowser.stripGenerated", true);
  const [deep, setDeep] = useState(deepDefault);

  const backups = useAsync<Backup[]>((s) => api.backups(s), []);

  // Select the newest backup as soon as the list arrives.
  useEffect(() => {
    if (!backupId && backups.data && backups.data.length > 0) setBackupId(backups.data[0].id);
  }, [backups.data, backupId]);

  const kinds = useAsync((s) => api.kinds(backupId!, s), [backupId], backupId !== null);
  const namespaces = useAsync<Namespace[]>((s) => api.namespaces(backupId!, s), [backupId], backupId !== null);
  const info = useAsync((s) => api.info(backupId!, s), [backupId], backupId !== null);

  // In the search view the top bar box drives the query, elsewhere the list
  // has its own name filter.
  const searching = view === "search";
  const activeSearch = useDebounced(searching ? globalSearch : nameFilter, 300);
  const activeKind = searching ? "*" : kindId;

  const resources = useAsync(
    (s) =>
      api.resources(
        backupId!,
        {
          kind: activeKind,
          namespace,
          q: activeSearch,
          deep,
          sort: sort.field,
          order: sort.desc ? "desc" : "asc",
          offset: page * pageSize,
          limit: pageSize,
        },
        s,
      ),
    [backupId, activeKind, namespace, activeSearch, deep, sort.field, sort.desc, page, pageSize],
    backupId !== null && (view === "browse" || view === "search"),
  );

  const detail = useAsync(
    (s) => api.resource(backupId!, selected!.id, s),
    [backupId, selected?.id],
    backupId !== null && selected !== null,
  );

  // Any change to the filters puts the reader back on the first page.
  useEffect(() => setPage(0), [activeKind, namespace, activeSearch, deep, pageSize, sort.field, sort.desc]);

  const kindList: Kind[] = kinds.data?.items ?? [];
  const currentKind = kindList.find((k) => k.id === kindId);

  useEffect(() => {
    if (kindPicked || kindList.length === 0) return;
    const preferred = kindList.find((k) => k.id === "v1/Pod") ?? kindList[0];
    setKindId(preferred.id);
  }, [kindList, kindPicked]);

  const listTitle = useMemo(() => {
    if (searching) return activeSearch ? `Results for "${activeSearch}"` : "Search";
    if (kindId === "*") return "All Resources";
    return currentKind?.label ?? "Resources";
  }, [searching, activeSearch, kindId, currentKind]);

  // An export must contain exactly the rows the list is showing, so it reuses
  // the same filters rather than the current page.
  const exportFilters = useMemo(
    () => ({ kind: activeKind, namespace, q: activeSearch, deep }),
    [activeKind, namespace, activeSearch, deep],
  );
  const exportLabel = useMemo(() => {
    if (activeKind === "*") return "resources";
    return (currentKind?.label ?? "resources").toLowerCase();
  }, [activeKind, currentKind]);

  const pickKind = useCallback((id: string) => {
    setKindPicked(true);
    setKindId(id);
    setSelected(null);
    setView("browse");
  }, []);

  const pickNamespace = useCallback((ns: string) => {
    setNamespace(ns);
    setSelected(null);
    setView("browse");
  }, []);

  const openResource = useCallback((r: Resource) => setSelected(r), []);

  if (backups.loading) {
    return <Splash>Reading backups</Splash>;
  }
  if (backups.error) {
    return (
      <Splash>
        <ErrorNote message={backups.error} />
      </Splash>
    );
  }
  if (!backups.data || backups.data.length === 0 || !backupId) {
    return <Splash>No backup was found. Start the server with -backup pointing at a snapshot.</Splash>;
  }

  const clusterName = info.data
    ? `${info.data.snapshot.members.find((m) => !m.removed)?.name ?? "unknown"} (etcd ${info.data.snapshot.clusterVersion})`
    : "loading";

  return (
    <div className="flex h-full min-h-0 flex-col">
      <TopBar
        backups={backups.data}
        backupId={backupId}
        onBackupChange={(id) => {
          setBackupId(id);
          setSelected(null);
        }}
        clusterName={clusterName}
        search={globalSearch}
        onSearchChange={(v) => {
          setGlobalSearch(v);
          if (v && view !== "search") setView("search");
        }}
        onSearchSubmit={() => setView("search")}
      />

      <div className="flex min-h-0 flex-1">
        <SideNav view={view} onChange={setView} />

        {view === "browse" && (
          <>
            <KindList
              kinds={kindList}
              total={kinds.data?.total ?? 0}
              selected={kindId}
              onSelect={pickKind}
              loading={kinds.loading}
            />
            <ResourceList
              title={listTitle}
              data={resources.data}
              loading={resources.loading}
              error={resources.error}
              namespaces={namespaces.data ?? []}
              namespace={namespace}
              onNamespaceChange={setNamespace}
              filter={nameFilter}
              onFilterChange={setNameFilter}
              deep={deep}
              onDeepChange={setDeep}
              sort={sort}
              onSortChange={setSort}
              page={page}
              pageSize={pageSize}
              onPageChange={setPage}
              selectedId={selected?.id ?? null}
              onSelect={openResource}
              showKindColumn={kindId === "*"}
              backupId={backupId}
              exportFilters={exportFilters}
              exportLabel={exportLabel}
              clean={clean}
            />
            <DetailPane
              backupId={backupId}
              detail={detail.data}
              loading={detail.loading}
              error={detail.error}
              takenAt={info.data?.backup.takenAt ?? null}
              fontSize={fontSize}
              clean={clean}
              onOpenResource={openResource}
            />
          </>
        )}

        {view === "search" && (
          <>
            <ResourceList
              title={listTitle}
              data={resources.data}
              loading={resources.loading}
              error={resources.error}
              namespaces={namespaces.data ?? []}
              namespace={namespace}
              onNamespaceChange={setNamespace}
              filter={globalSearch}
              onFilterChange={setGlobalSearch}
              deep={deep}
              onDeepChange={setDeep}
              sort={sort}
              onSortChange={setSort}
              page={page}
              pageSize={pageSize}
              onPageChange={setPage}
              selectedId={selected?.id ?? null}
              onSelect={openResource}
              showKindColumn
              backupId={backupId}
              exportFilters={exportFilters}
              exportLabel={exportLabel}
              clean={clean}
            />
            <DetailPane
              backupId={backupId}
              detail={detail.data}
              loading={detail.loading}
              error={detail.error}
              takenAt={info.data?.backup.takenAt ?? null}
              fontSize={fontSize}
              clean={clean}
              onOpenResource={openResource}
            />
          </>
        )}

        {view === "namespaces" && (
          <NamespacesView
            namespaces={namespaces.data ?? []}
            loading={namespaces.loading}
            onPick={pickNamespace}
          />
        )}

        {view === "types" && (
          <TypesView
            kinds={kindList}
            total={kinds.data?.total ?? 0}
            loading={kinds.loading}
            onPick={pickKind}
          />
        )}

        {view === "info" && <InfoView backupId={backupId} fontSize={fontSize} />}

        {view === "settings" && (
          <SettingsView
            pageSize={pageSize}
            onPageSize={setPageSize}
            fontSize={fontSize}
            onFontSize={setFontSize}
            deepDefault={deepDefault}
            onDeepDefault={(v) => {
              setDeepDefault(v);
              setDeep(v);
            }}
            clean={clean}
            onClean={setClean}
          />
        )}
      </div>

      <footer className="flex h-7 shrink-0 items-center gap-4 border-t border-line bg-surface px-4 text-[11px] text-muted">
        <span>{countLabel(kinds.data?.total ?? 0)} objects indexed</span>
        <span>{countLabel(kindList.length)} resource types</span>
        <span>{countLabel((namespaces.data ?? []).length)} namespaces</span>
        <span className="ml-auto">Snapshot opened read only</span>
      </footer>
    </div>
  );
}

function Splash({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="max-w-[520px] text-center text-sm text-muted">
        {typeof children === "string" ? <Spinner label={children} /> : children}
      </div>
    </div>
  );
}
