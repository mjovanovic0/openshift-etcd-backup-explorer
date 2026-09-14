import { useEffect, useMemo, useState } from "react";

/** Objects such as a CustomResourceDefinition run to thousands of lines, so
 * the viewer renders a first chunk and reveals the rest on request. */
const chunkSize = 1500;

/** CodeView shows a manifest with line numbers and light highlighting. */
export function CodeView({
  text,
  language,
  fontSize,
}: {
  text: string;
  language: "yaml" | "json";
  fontSize: number;
}) {
  const lines = useMemo(() => text.replace(/\n$/, "").split("\n"), [text]);
  const [shown, setShown] = useState(chunkSize);
  useEffect(() => setShown(chunkSize), [text]);

  const visible = lines.slice(0, shown);
  const hidden = lines.length - visible.length;
  const gutter = String(lines.length).length;

  return (
    <div className="min-h-0 flex-1 overflow-auto bg-surface">
      <table className="w-full border-separate border-spacing-0 font-mono" style={{ fontSize }}>
        <tbody>
          {visible.map((line, i) => (
            <tr key={i} className="group">
              <td
                className="sticky left-0 z-10 w-px border-r border-line-soft bg-surface-alt px-2 text-right align-top text-muted select-none"
                style={{ minWidth: `${gutter + 1.5}ch` }}
              >
                {i + 1}
              </td>
              <td className="px-3 align-top whitespace-pre text-ink group-hover:bg-brand-soft/40">
                {language === "yaml" ? highlightYaml(line) : highlightJson(line)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {hidden > 0 && (
        <div className="border-t border-line-soft bg-surface-alt px-4 py-3 text-center">
          <button
            type="button"
            onClick={() => setShown((n) => n + chunkSize * 4)}
            className="rounded border border-line bg-surface px-3 py-1.5 text-[13px] font-medium text-brand hover:bg-brand-soft"
          >
            Show {Math.min(hidden, chunkSize * 4).toLocaleString("en-US")} more lines
          </button>
          <p className="mt-1 text-[11px] text-muted">
            {hidden.toLocaleString("en-US")} of {lines.length.toLocaleString("en-US")} lines hidden
          </p>
        </div>
      )}
    </div>
  );
}

/**
 * A deliberately small highlighter. It colours the parts an operator scans for
 * (keys, strings, numbers, booleans) without pulling in a syntax library.
 */
function highlightYaml(line: string) {
  const comment = line.indexOf("#");
  if (comment === 0 || (comment > 0 && /^\s*#/.test(line))) {
    return <span className="text-muted italic">{line}</span>;
  }
  const m = /^(\s*(?:-\s+)?)([A-Za-z0-9_.\-/"']+)(:)(\s*)(.*)$/.exec(line);
  if (!m) {
    const item = /^(\s*-\s+)(.*)$/.exec(line);
    if (item) {
      return (
        <>
          <span className="text-muted">{item[1]}</span>
          {value(item[2])}
        </>
      );
    }
    return <span>{line}</span>;
  }
  const [, indent, key, colon, space, rest] = m;
  return (
    <>
      <span>{indent}</span>
      <span className="text-[#004080]">{key}</span>
      <span className="text-muted">{colon}</span>
      <span>{space}</span>
      {value(rest)}
    </>
  );
}

function highlightJson(line: string) {
  const parts: React.ReactNode[] = [];
  const re = /("(?:[^"\\]|\\.)*")(\s*:)?|(\btrue\b|\bfalse\b|\bnull\b)|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let i = 0;
  while ((m = re.exec(line)) !== null) {
    if (m.index > last) parts.push(<span key={i++}>{line.slice(last, m.index)}</span>);
    if (m[1] && m[2]) {
      parts.push(
        <span key={i++} className="text-[#004080]">
          {m[1]}
        </span>,
        <span key={i++} className="text-muted">
          {m[2]}
        </span>,
      );
    } else if (m[1]) {
      parts.push(
        <span key={i++} className="text-[#0f6100]">
          {m[1]}
        </span>,
      );
    } else if (m[3]) {
      parts.push(
        <span key={i++} className="text-[#8b4cb8]">
          {m[3]}
        </span>,
      );
    } else if (m[4]) {
      parts.push(
        <span key={i++} className="text-[#a35200]">
          {m[4]}
        </span>,
      );
    }
    last = m.index + m[0].length;
  }
  if (last < line.length) parts.push(<span key={i++}>{line.slice(last)}</span>);
  return <>{parts}</>;
}

function value(rest: string) {
  if (rest === "") return null;
  if (/^(true|false|null|~)$/.test(rest)) return <span className="text-[#8b4cb8]">{rest}</span>;
  if (/^-?\d+(\.\d+)?$/.test(rest)) return <span className="text-[#a35200]">{rest}</span>;
  if (/^["'].*["']$/.test(rest)) return <span className="text-[#0f6100]">{rest}</span>;
  if (/^[|>]/.test(rest)) return <span className="text-muted">{rest}</span>;
  return <span className="text-[#0f6100]">{rest}</span>;
}
