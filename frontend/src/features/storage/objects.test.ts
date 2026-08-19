import { describe, it, expect } from "vitest";
import { filterEntries, sortEntries, breadcrumbSegments, normalizePrefix } from "./objects";
import type { ObjectEntry } from "~/lib/api";

const entry = (over: Partial<ObjectEntry>): ObjectEntry =>
  ({
    key: "k",
    name: "k",
    isFolder: false,
    size: 0,
    lastModified: 0,
    etag: "",
    storageClass: "STANDARD",
    ...over,
  }) as ObjectEntry;

describe("filterEntries", () => {
  const entries = [
    entry({ name: "Report.pdf" }),
    entry({ name: "notes.txt" }),
    entry({ name: "archive/", isFolder: true }),
  ];

  it("returns everything for a blank query", () => {
    expect(filterEntries(entries, "   ")).toHaveLength(3);
  });

  it("matches case-insensitively on the display name", () => {
    expect(filterEntries(entries, "REPORT").map((e) => e.name)).toEqual(["Report.pdf"]);
  });

  it("matches substrings, not just prefixes", () => {
    expect(filterEntries(entries, "ote").map((e) => e.name)).toEqual(["notes.txt"]);
  });

  it("returns an empty list when nothing matches", () => {
    expect(filterEntries(entries, "zzz")).toEqual([]);
  });
});

describe("sortEntries", () => {
  const entries = [
    entry({ name: "b.txt", size: 30, lastModified: 300 }),
    entry({ name: "zeta/", isFolder: true }),
    entry({ name: "a.txt", size: 10, lastModified: 100 }),
    entry({ name: "alpha/", isFolder: true }),
    entry({ name: "c.txt", size: 20, lastModified: 200 }),
  ];

  it("always puts folders first, in both directions", () => {
    for (const dir of ["asc", "desc"] as const) {
      const sorted = sortEntries(entries, "name", dir);
      expect(sorted.slice(0, 2).every((e) => e.isFolder)).toBe(true);
      expect(sorted.slice(2).some((e) => e.isFolder)).toBe(false);
    }
  });

  it("sorts by name ascending and descending", () => {
    expect(
      sortEntries(entries, "name", "asc")
        .filter((e) => !e.isFolder)
        .map((e) => e.name)
    ).toEqual(["a.txt", "b.txt", "c.txt"]);
    expect(
      sortEntries(entries, "name", "desc")
        .filter((e) => !e.isFolder)
        .map((e) => e.name)
    ).toEqual(["c.txt", "b.txt", "a.txt"]);
  });

  it("sorts by size and by modification time", () => {
    expect(
      sortEntries(entries, "size", "desc")
        .filter((e) => !e.isFolder)
        .map((e) => e.size)
    ).toEqual([30, 20, 10]);
    expect(
      sortEntries(entries, "modified", "asc")
        .filter((e) => !e.isFolder)
        .map((e) => e.lastModified)
    ).toEqual([100, 200, 300]);
  });

  it("does not mutate its input", () => {
    const before = entries.map((e) => e.name);
    sortEntries(entries, "size", "desc");
    expect(entries.map((e) => e.name)).toEqual(before);
  });
});

describe("breadcrumbSegments", () => {
  it("returns nothing at the bucket root", () => {
    expect(breadcrumbSegments("")).toEqual([]);
  });

  it("accumulates each segment's own prefix", () => {
    expect(breadcrumbSegments("a/b/c/")).toEqual([
      { name: "a", prefix: "a/" },
      { name: "b", prefix: "a/b/" },
      { name: "c", prefix: "a/b/c/" },
    ]);
  });

  it("ignores duplicate and leading separators", () => {
    expect(breadcrumbSegments("//a//b/")).toEqual([
      { name: "a", prefix: "a/" },
      { name: "b", prefix: "a/b/" },
    ]);
  });
});

describe("normalizePrefix", () => {
  it.each([
    ["", ""],
    ["   ", ""],
    ["a/b", "a/b/"],
    ["a/b/", "a/b/"],
    ["/a/b", "a/b/"],
    ["///a", "a/"],
    ["  a/b  ", "a/b/"],
  ])("normalises %j to %j", (input, expected) => {
    expect(normalizePrefix(input)).toBe(expected);
  });
});
