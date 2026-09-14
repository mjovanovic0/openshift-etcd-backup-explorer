import type { ReactNode } from "react";
import { IconFolder, IconGear, IconGlobe, IconGrid, IconInfo, IconSearch } from "./Icons";

export type View = "browse" | "namespaces" | "types" | "search" | "info" | "settings";

const primary: { id: View; label: string; icon: ReactNode }[] = [
  { id: "browse", label: "Browse", icon: <IconFolder /> },
  { id: "namespaces", label: "Namespaces", icon: <IconGlobe /> },
  { id: "types", label: "Resource Types", icon: <IconGrid /> },
  { id: "search", label: "Search", icon: <IconSearch /> },
];

const secondary: { id: View; label: string; icon: ReactNode }[] = [
  { id: "info", label: "Backup Info", icon: <IconInfo /> },
  { id: "settings", label: "Settings", icon: <IconGear /> },
];

export function SideNav({ view, onChange }: { view: View; onChange: (v: View) => void }) {
  return (
    <nav aria-label="Sections" className="flex w-[200px] shrink-0 flex-col bg-chrome py-2 text-chrome-text">
      <ul>
        {primary.map((item) => (
          <NavItem key={item.id} {...item} active={view === item.id} onClick={() => onChange(item.id)} />
        ))}
      </ul>
      <ul className="mt-auto">
        {secondary.map((item) => (
          <NavItem key={item.id} {...item} active={view === item.id} onClick={() => onChange(item.id)} />
        ))}
      </ul>
    </nav>
  );
}

function NavItem({
  label,
  icon,
  active,
  onClick,
}: {
  label: string;
  icon: ReactNode;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <li>
      <button
        type="button"
        onClick={onClick}
        aria-current={active ? "page" : undefined}
        className={`flex w-full items-center gap-3 border-l-[3px] py-2.5 pr-3 pl-[13px] text-left text-[14px] transition-colors ${
          active
            ? "border-info bg-chrome-hover font-medium text-white"
            : "border-transparent text-chrome-text hover:bg-chrome-hover hover:text-white"
        }`}
      >
        {icon}
        <span className="truncate">{label}</span>
      </button>
    </li>
  );
}
