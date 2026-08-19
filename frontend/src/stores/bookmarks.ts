import { createSignal } from "solid-js";
import { BookmarkService, type Bookmark, onEvent } from "~/lib/api";

const [bookmarks, setBookmarks] = createSignal<Bookmark[]>([]);
export { bookmarks };

/** Load every saved location from the backend. */
export async function loadBookmarks() {
  try {
    setBookmarks((await BookmarkService.List()) ?? []);
  } catch (e) {
    console.error("loadBookmarks", e);
  }
}

/** Save (or relabel) a location and refresh the list. */
export async function addBookmark(bookmark: Partial<Bookmark>): Promise<Bookmark> {
  const saved = await BookmarkService.Add(bookmark as Bookmark);
  await loadBookmarks();
  return saved;
}

export async function removeBookmark(id: string) {
  await BookmarkService.Delete(id);
  await loadBookmarks();
}

/** Whether a location is already bookmarked, matching the stored prefix shape. */
export function isBookmarked(connectionId: string, bucket: string, prefix: string): boolean {
  return bookmarks().some(
    (b) => b.connectionId === connectionId && b.bucket === bucket && b.prefix === prefix
  );
}

export function findBookmark(
  connectionId: string,
  bucket: string,
  prefix: string
): Bookmark | undefined {
  return bookmarks().find(
    (b) => b.connectionId === connectionId && b.bucket === bucket && b.prefix === prefix
  );
}

// Bookmarks are shared across windows: reload when any window changes them.
onEvent("bookmarks:changed", () => loadBookmarks());
