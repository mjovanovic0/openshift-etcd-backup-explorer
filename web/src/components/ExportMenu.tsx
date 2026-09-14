import { useState } from "react";
import { api } from "../api";
import type { ExportFilters, ExportFormat } from "../api";
import { countLabel, humanBytes } from "../format";
import { Menu } from "./Menu";
import type { MenuItem } from "./Menu";
import { useToast } from "./Toast";
import { IconDownload } from "./Icons";

/**
 * ExportMenu exports everything matching the current filters, not just the
 * page on screen, so "all PersistentVolumes" means all of them.
 */
export function ExportMenu({
  backupId,
  filters,
  total,
  label,
  clean,
}: {
  backupId: string;
  filters: ExportFilters;
  total: number;
  label: string;
  clean: boolean;
}) {
  const notify = useToast();
  const [busy, setBusy] = useState(false);

  // The server refuses to build a single response beyond this, so say so here
  // rather than letting the request fail.
  const limit = 20000;
  const tooMany = total > limit;
  const tooManyHint = `${countLabel(total)} is more than the ${countLabel(limit)} that can be exported at once, filter by namespace or name first`;

  const url = (format: ExportFormat) => api.exportURL(backupId, filters, format, clean);

  const copyAll = async () => {
    setBusy(true);
    try {
      const text = await api.fetchText(url("yaml"));
      const { copyText } = await import("../clipboard");
      await copyText(text);
      notify(
        `Copied ${countLabel(total)} ${label} as YAML (${humanBytes(new Blob([text]).size)})` +
          (clean ? ", generated fields removed" : ""),
      );
    } catch (err) {
      notify(`Could not copy: ${err instanceof Error ? err.message : String(err)}`, "error");
    } finally {
      setBusy(false);
    }
  };

  const blocked = total === 0 || tooMany;
  const hint = (normal: string) => (tooMany ? tooManyHint : normal);

  const items: MenuItem[] = [
    {
      id: "copy",
      label: "Copy all as YAML",
      hint: hint(`${countLabel(total)} ${label} onto the clipboard`),
      onSelect: copyAll,
      disabled: blocked,
    },
    {
      id: "zip",
      label: "Download as a folder (zip)",
      hint: hint("one file per resource, laid out by kind and namespace"),
      href: url("zip"),
      disabled: blocked,
    },
    {
      id: "yaml",
      label: "Download as one YAML file",
      hint: hint("a single document stream, ready for kubectl apply"),
      href: url("yaml"),
      disabled: blocked,
    },
    {
      id: "json",
      label: "Download as one JSON file",
      hint: hint("a v1 List holding every object"),
      href: url("json"),
      disabled: blocked,
    },
  ];

  return <Menu label="Export all" icon={<IconDownload className="h-3.5 w-3.5" />} items={items} busy={busy} />;
}
