import { createSignal, createEffect, on } from "solid-js";
import { AppService, windowID, type TabSession } from "~/lib/api";

/** A browser-like session tab within the storage page. */
export type Tab = {
  id: string;
  connectionId: string;
  title: string;
  /** null => show the bucket list; otherwise the open bucket. */
  bucket: string | null;
  /** Current folder prefix within the bucket (always "" or ends with "/"). */
  prefix: string;
};

let counter = 0;
const newId = () => `tab-${++counter}-${Date.now()}`;

const wid = windowID();

const [tabs, setTabs] = createSignal<Tab[]>([]);
const [activeTabId, setActiveTabId] = createSignal<string>("");
export { tabs, activeTabId };

export function activeTab(): Tab | undefined {
  return tabs().find((t) => t.id === activeTabId());
}

/** Open a new tab for a connection and focus it. */
export function openTab(connectionId: string, title: string, bucket: string | null = null): Tab {
  const tab: Tab = { id: newId(), connectionId, title, bucket, prefix: "" };
  setTabs((prev) => [...prev, tab]);
  setActiveTabId(tab.id);
  return tab;
}

export function closeTab(id: string) {
  const list = tabs();
  const idx = list.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const next = list.filter((t) => t.id !== id);
  setTabs(next);
  if (activeTabId() === id && next.length) {
    setActiveTabId(next[Math.max(0, idx - 1)].id);
  } else if (!next.length) {
    setActiveTabId("");
  }
}

export function focusTab(id: string) {
  setActiveTabId(id);
}

export function updateTab(id: string, patch: Partial<Tab>) {
  setTabs((prev) => prev.map((t) => (t.id === id ? { ...t, ...patch } : t)));
}

/** Navigate a tab into a bucket (or back to the bucket list when null). */
export function openBucket(id: string, bucket: string | null) {
  updateTab(id, { bucket, prefix: "" });
}

/** Navigate a tab to a folder prefix. */
export function navigatePrefix(id: string, prefix: string) {
  updateTab(id, { prefix });
}

/* ------------------------------ persistence ------------------------------- */

// Tab strips are restored per window id. Window ids are handed out in the same
// order every run ("win-1", "win-2", …), so the first window reopens the
// workspace the first window had.

// Reactive so the mirroring effect re-runs once the restore completes.
const [restored, setRestored] = createSignal(false);

/** Load the tab strip this window had when the app last closed. */
export async function restoreTabs(): Promise<void> {
  try {
    const session = await AppService.Session(wid);
    const saved = session?.tabs ?? [];
    if (!saved.length) return;
    const list: Tab[] = saved.map((s) => ({
      id: newId(),
      connectionId: s.connectionId,
      title: s.title,
      bucket: s.bucket || null,
      prefix: s.prefix ?? "",
    }));
    setTabs(list);
    const active = saved.findIndex((s) => s.active);
    setActiveTabId(list[active >= 0 ? active : list.length - 1].id);
  } catch (e) {
    console.error("restoreTabs", e);
  } finally {
    setRestored(true);
  }
}

/**
 * Mirror every tab change back to the backend. Persisting starts only once the
 * restore has run, so an empty initial store never overwrites a saved session.
 */
export function persistTabs() {
  createEffect(
    on([tabs, activeTabId, restored], ([list, active, ready]) => {
      if (!ready) return;
      const payload: TabSession[] = list.map((t) => ({
        connectionId: t.connectionId,
        title: t.title,
        bucket: t.bucket ?? "",
        prefix: t.prefix,
        active: t.id === active,
      }));
      void AppService.SaveSession(wid, payload);
    })
  );
}
