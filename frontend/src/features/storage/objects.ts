import { type ObjectEntry } from "~/lib/api";

export type SortKey = "name" | "size" | "modified";
export type SortDir = "asc" | "desc";

/** Filter entries by a case-insensitive substring of their display name. */
export function filterEntries(entries: ObjectEntry[], query: string): ObjectEntry[] {
  const q = query.trim().toLowerCase();
  if (!q) return entries;
  return entries.filter((e) => e.name.toLowerCase().includes(q));
}

/** Sort entries with folders always first, then by the chosen key/direction. */
export function sortEntries(entries: ObjectEntry[], key: SortKey, dir: SortDir): ObjectEntry[] {
  const sign = dir === "asc" ? 1 : -1;
  return [...entries].sort((a, b) => {
    if (a.isFolder !== b.isFolder) return a.isFolder ? -1 : 1;
    let cmp = 0;
    switch (key) {
      case "size":
        cmp = a.size - b.size;
        break;
      case "modified":
        cmp = a.lastModified - b.lastModified;
        break;
      default:
        cmp = a.name.localeCompare(b.name);
    }
    return cmp * sign;
  });
}

/** Split a prefix into breadcrumb segments with their cumulative prefixes. */
export function breadcrumbSegments(prefix: string): { name: string; prefix: string }[] {
  const parts = prefix.split("/").filter(Boolean);
  const segments: { name: string; prefix: string }[] = [];
  let acc = "";
  for (const p of parts) {
    acc += p + "/";
    segments.push({ name: p, prefix: acc });
  }
  return segments;
}

/** Normalise a user-typed path into a clean prefix ("" or ends with "/"). */
export function normalizePrefix(input: string): string {
  let p = input.trim().replace(/^\/+/, "");
  if (p && !p.endsWith("/")) p += "/";
  return p;
}
