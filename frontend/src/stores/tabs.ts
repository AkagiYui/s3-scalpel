import { createSignal } from "solid-js";

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
