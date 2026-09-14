import { useCallback, useEffect, useRef, useState } from "react";

type AsyncState<T> = { data: T | null; error: string | null; loading: boolean };

/**
 * useAsync runs a fetch whenever its dependencies change and cancels the
 * previous request, so fast typing or clicking cannot show a stale answer.
 */
export function useAsync<T>(
  fn: (signal: AbortSignal) => Promise<T>,
  deps: unknown[],
  enabled = true,
): AsyncState<T> & { reload: () => void } {
  const [state, setState] = useState<AsyncState<T>>({ data: null, error: null, loading: enabled });
  const [nonce, setNonce] = useState(0);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  useEffect(() => {
    if (!enabled) {
      setState({ data: null, error: null, loading: false });
      return;
    }
    const ctl = new AbortController();
    setState((s) => ({ data: s.data, error: null, loading: true }));
    fnRef
      .current(ctl.signal)
      .then((data) => {
        if (!ctl.signal.aborted) setState({ data, error: null, loading: false });
      })
      .catch((err: unknown) => {
        if (ctl.signal.aborted) return;
        setState({ data: null, error: err instanceof Error ? err.message : String(err), loading: false });
      });
    return () => ctl.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, enabled, nonce]);

  const reload = useCallback(() => setNonce((n) => n + 1), []);
  return { ...state, reload };
}

/** useDebounced delays a fast changing value, used for the search inputs. */
export function useDebounced<T>(value: T, ms = 250): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return debounced;
}

/** useLocalSetting persists a small preference in the browser. */
export function useLocalSetting<T>(key: string, initial: T): [T, (v: T) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key);
      return raw === null ? initial : (JSON.parse(raw) as T);
    } catch {
      return initial;
    }
  });
  const set = useCallback(
    (v: T) => {
      setValue(v);
      try {
        localStorage.setItem(key, JSON.stringify(v));
      } catch {
        // Storage can be unavailable in a private window, which is fine.
      }
    },
    [key],
  );
  return [value, set];
}
