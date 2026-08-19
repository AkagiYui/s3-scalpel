import { describe, it, expect, beforeEach } from "vitest";
import {
  tabs,
  activeTabId,
  activeTab,
  openTab,
  closeTab,
  focusTab,
  updateTab,
  openBucket,
  navigatePrefix,
} from "./tabs";

/** Close every tab so each test starts from a clean store. */
function reset() {
  for (const tab of [...tabs()]) closeTab(tab.id);
}

describe("tab store", () => {
  beforeEach(reset);

  it("focuses a newly opened tab", () => {
    const a = openTab("conn-1", "One");
    expect(tabs()).toHaveLength(1);
    expect(activeTabId()).toBe(a.id);
    expect(activeTab()?.connectionId).toBe("conn-1");
  });

  it("starts a tab at the bucket list with an empty prefix", () => {
    const tab = openTab("conn-1", "One");
    expect(tab.bucket).toBeNull();
    expect(tab.prefix).toBe("");
  });

  it("gives every tab a distinct id", () => {
    const ids = [openTab("c", "a").id, openTab("c", "b").id, openTab("c", "c").id];
    expect(new Set(ids).size).toBe(3);
  });

  it("focuses the previous tab when the active one closes", () => {
    const a = openTab("c", "a");
    const b = openTab("c", "b");
    const c = openTab("c", "c");
    expect(activeTabId()).toBe(c.id);

    closeTab(c.id);
    expect(activeTabId()).toBe(b.id);

    closeTab(b.id);
    expect(activeTabId()).toBe(a.id);
  });

  it("keeps the focus when a non-active tab closes", () => {
    const a = openTab("c", "a");
    const b = openTab("c", "b");
    focusTab(b.id);
    closeTab(a.id);
    expect(activeTabId()).toBe(b.id);
    expect(tabs()).toHaveLength(1);
  });

  it("focuses the first remaining tab when the leftmost one closes", () => {
    const a = openTab("c", "a");
    const b = openTab("c", "b");
    focusTab(a.id);
    closeTab(a.id);
    expect(activeTabId()).toBe(b.id);
  });

  it("clears the focus when the last tab closes", () => {
    const a = openTab("c", "a");
    closeTab(a.id);
    expect(tabs()).toEqual([]);
    expect(activeTabId()).toBe("");
    expect(activeTab()).toBeUndefined();
  });

  it("ignores closing an unknown id", () => {
    const a = openTab("c", "a");
    closeTab("nope");
    expect(tabs()).toHaveLength(1);
    expect(activeTabId()).toBe(a.id);
  });

  it("resets the prefix when navigating into or out of a bucket", () => {
    const tab = openTab("c", "a");
    navigatePrefix(tab.id, "deep/path/");
    expect(activeTab()?.prefix).toBe("deep/path/");

    openBucket(tab.id, "my-bucket");
    expect(activeTab()?.bucket).toBe("my-bucket");
    expect(activeTab()?.prefix).toBe("");

    navigatePrefix(tab.id, "x/");
    openBucket(tab.id, null);
    expect(activeTab()?.bucket).toBeNull();
    expect(activeTab()?.prefix).toBe("");
  });

  it("patches only the named tab", () => {
    const a = openTab("c", "a");
    const b = openTab("c", "b");
    updateTab(a.id, { title: "renamed" });
    expect(tabs().find((t) => t.id === a.id)?.title).toBe("renamed");
    expect(tabs().find((t) => t.id === b.id)?.title).toBe("b");
  });
});
