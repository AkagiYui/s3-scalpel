import {
  createSignal,
  createEffect,
  createMemo,
  on,
  For,
  Show,
  onMount,
  onCleanup,
  type Component,
} from "solid-js";
import {
  ChevronRight,
  Home,
  RefreshCw,
  FolderPlus,
  Upload,
  Folder,
  File as FileIcon,
  MoreVertical,
  Download,
  Eye,
  Info,
  Link as LinkIcon,
  Tag as TagIcon,
  History,
  Copy,
  FolderInput,
  Trash2,
  ArrowUpDown,
  Search,
  ChevronLeft,
  ShieldCheck,
  SlidersHorizontal,
  BarChart3,
  X as XIcon,
  Boxes,
  PenLine,
  ClipboardCopy,
  Star,
  BookMarked,
} from "lucide-solid";
import { Button, buttonVariants } from "~/components/ui/button";
import { Input, Checkbox, Spinner } from "~/components/ui/primitives";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "~/components/ui/dropdown-menu";
import {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
} from "~/components/ui/context-menu";
import { ConfirmDialog } from "~/components/ConfirmDialog";
import { PromptDialog } from "./PromptDialog";
import { DownloadDialog } from "./DownloadDialog";
import { DestinationDialog } from "./DestinationDialog";
import { PropertiesDialog } from "./PropertiesDialog";
import { PresignDialog } from "./PresignDialog";
import { ObjectSettingsDialog } from "./ObjectSettingsDialog";
import { StatsDialog } from "./StatsDialog";
import { TagsDialog } from "./TagsDialog";
import { VersionsDialog } from "./VersionsDialog";
import { PreviewDialog } from "./PreviewDialog";
import { CapabilitiesDialog } from "./CapabilitiesDialog";
import { MultipartCleanupDialog } from "./MultipartCleanupDialog";
import {
  S3Service,
  QueueService,
  AppService,
  windowID,
  onEvent,
  type ObjectEntry,
  type BucketInfo,
} from "~/lib/api";
import { navigatePrefix, openBucket, type Tab } from "~/stores/tabs";
import {
  filterEntries,
  sortEntries,
  breadcrumbSegments,
  normalizePrefix,
  type SortKey,
  type SortDir,
} from "./objects";
import { formatBytes, formatDate, keyBasename } from "~/lib/utils";
import { effectiveLocale, updateSettings } from "~/stores/settings";
import { toast } from "~/components/ui/toast";
import { cn } from "~/lib/utils";
import * as bus from "~/lib/bus";
import { createVirtualizer } from "@tanstack/solid-virtual";
import { bookmarks, addBookmark, removeBookmark, findBookmark } from "~/stores/bookmarks";
import { createCancellableOp } from "~/lib/operation";
import { t } from "~/i18n";

const wid = windowID();

/** Fixed row height (px) the virtualiser measures against. */
const ROW_HEIGHT = 34;

export const FileBrowser: Component<{ tab: Tab }> = (props) => {
  const [entries, setEntries] = createSignal<ObjectEntry[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [nextToken, setNextToken] = createSignal("");
  const [filter, setFilter] = createSignal("");
  const [searchResults, setSearchResults] = createSignal<ObjectEntry[] | null>(null);
  const [searchTruncated, setSearchTruncated] = createSignal(false);
  const searchOp = createCancellableOp();
  const searching = searchOp.running;
  const [statsOpen, setStatsOpen] = createSignal(false);
  const [sortKey, setSortKey] = createSignal<SortKey>("name");
  const [sortDir, setSortDir] = createSignal<SortDir>("asc");
  const [selected, setSelected] = createSignal<Set<string>>(new Set<string>());
  const [focusIndex, setFocusIndex] = createSignal(-1);
  const [buckets, setBuckets] = createSignal<BucketInfo[]>([]);
  let filterInput: HTMLInputElement | undefined;
  // A signal, not a plain ref: the virtualiser has to react when it is attached.
  const [listEl, setListEl] = createSignal<HTMLDivElement>();
  // Anchor row for shift-range selection; -1 until the user clicks a row.
  let anchor = -1;

  // Dialog state.
  const [newFolderOpen, setNewFolderOpen] = createSignal(false);
  const [jumpOpen, setJumpOpen] = createSignal(false);
  const [previewKey, setPreviewKey] = createSignal<string | null>(null);
  const [propsKey, setPropsKey] = createSignal<string | null>(null);
  const [presignKey, setPresignKey] = createSignal<string | null>(null);
  const [tagsKey, setTagsKey] = createSignal<string | null>(null);
  const [objSettingsKey, setObjSettingsKey] = createSignal<string | null>(null);
  const [versionsKey, setVersionsKey] = createSignal<string | null>(null);
  const [downloadKeys, setDownloadKeys] = createSignal<string[] | null>(null);
  const [copyMove, setCopyMove] = createSignal<{ keys: string[]; move: boolean } | null>(null);
  const [deleteKeys, setDeleteKeys] = createSignal<string[] | null>(null);
  const [capsOpen, setCapsOpen] = createSignal(false);
  const [renameEntry, setRenameEntry] = createSignal<ObjectEntry | null>(null);
  const [batchTagKeys, setBatchTagKeys] = createSignal<string[] | null>(null);
  const [multipartOpen, setMultipartOpen] = createSignal(false);

  const bucket = () => props.tab.bucket!;
  const prefix = () => props.tab.prefix;

  const load = async (reset: boolean) => {
    setLoading(true);
    try {
      const token = reset ? "" : nextToken();
      const res = await S3Service.ListObjects(props.tab.connectionId, bucket(), prefix(), token);
      const list = res.entries ?? [];
      setEntries((prev) => (reset ? list : [...prev, ...list]));
      setNextToken(res.nextToken ?? "");
    } catch (e: any) {
      toast.error(t("errors.loadObjects") + " " + String(e?.message ?? e));
    } finally {
      setLoading(false);
    }
  };

  // Reload when the connection / bucket / prefix changes.
  createEffect(
    on(
      () => [props.tab.connectionId, props.tab.bucket, props.tab.prefix],
      () => {
        clearSelection();
        setFilter("");
        setSearchResults(null);
        setSearchTruncated(false);
        load(true);
      }
    )
  );

  // Load buckets for copy/move destination selection.
  createEffect(
    on(
      () => props.tab.connectionId,
      async (connId) => {
        try {
          setBuckets((await S3Service.ListBuckets(connId)) ?? []);
        } catch {
          /* ignore */
        }
      }
    )
  );

  // Refresh when an operation finishes that affects this view.
  onMount(() => {
    const offDone = onEvent<any>("operation:done", (d) => {
      if (d?.bucket === bucket() || d?.destBucket === bucket()) load(true);
    });
    const offRefresh = bus.on("refresh", () => load(true));
    const offFind = bus.on("find", () => filterInput?.focus());
    onCleanup(() => {
      offDone();
      offRefresh();
      offFind();
    });
  });

  const visible = createMemo(() => {
    const sr = searchResults();
    if (sr !== null) return sortEntries(sr, sortKey(), sortDir());
    return sortEntries(filterEntries(entries(), filter()), sortKey(), sortDir());
  });

  const rowVirtualizer = createVirtualizer({
    get count() {
      return visible().length;
    },
    getScrollElement: () => listEl() ?? null,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  });

  const runSearch = async () => {
    const q = filter().trim();
    if (!q) {
      setSearchResults(null);
      return;
    }
    try {
      const res = await searchOp.run((opID) =>
        S3Service.Search(opID, props.tab.connectionId, bucket(), prefix(), q, 1000)
      );
      if (!res) return; // cancelled or superseded
      setSearchResults(res.entries ?? []);
      setSearchTruncated(res.truncated ?? false);
      clearSelection();
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const clearSearch = () => {
    setSearchResults(null);
    setSearchTruncated(false);
  };

  /* -------------------------------- selection ------------------------------ */

  const clearSelection = () => {
    setSelected(new Set<string>());
    setFocusIndex(-1);
    anchor = -1;
  };

  const allSelected = () => visible().length > 0 && visible().every((e) => selected().has(e.key));

  const toggleAll = () => {
    if (allSelected()) {
      setSelected(new Set<string>());
    } else {
      setSelected(new Set(visible().map((e) => e.key)));
    }
  };

  const toggle = (key: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  /** Select exactly the rows between the anchor and `index` (inclusive). */
  const selectRange = (index: number) => {
    const from = anchor < 0 ? index : anchor;
    const [lo, hi] = from <= index ? [from, index] : [index, from];
    setSelected(
      new Set(
        visible()
          .slice(lo, hi + 1)
          .map((e) => e.key)
      )
    );
  };

  /**
   * Row click semantics match a native file browser: a plain click replaces the
   * selection, cmd/ctrl-click toggles a single row and shift-click extends from
   * the anchor. Opening a row is double-click (or Enter).
   */
  const onRowClick = (index: number, ev: MouseEvent) => {
    const entry = visible()[index];
    if (!entry) return;
    setFocusIndex(index);
    if (ev.shiftKey) {
      selectRange(index);
      return;
    }
    if (ev.metaKey || ev.ctrlKey) {
      toggle(entry.key);
      anchor = index;
      return;
    }
    setSelected(new Set([entry.key]));
    anchor = index;
  };

  /** Move the keyboard focus, optionally extending the selection with shift. */
  const moveFocus = (delta: number, extend: boolean) => {
    const rows = visible();
    if (!rows.length) return;
    const cur = focusIndex();
    const next = Math.min(rows.length - 1, Math.max(0, cur < 0 ? 0 : cur + delta));
    setFocusIndex(next);
    if (extend) {
      if (anchor < 0) anchor = cur < 0 ? next : cur;
      selectRange(next);
    } else {
      setSelected(new Set([rows[next].key]));
      anchor = next;
    }
    // Off-window rows have no DOM node, so scrolling goes through the
    // virtualiser rather than through the element.
    rowVirtualizer.scrollToIndex(next, { align: "auto" });
  };

  const onListKeyDown = (ev: KeyboardEvent) => {
    const rows = visible();
    switch (ev.key) {
      case "ArrowDown":
        ev.preventDefault();
        moveFocus(1, ev.shiftKey);
        break;
      case "ArrowUp":
        ev.preventDefault();
        moveFocus(-1, ev.shiftKey);
        break;
      case "Home":
        ev.preventDefault();
        setFocusIndex(-1);
        moveFocus(1, ev.shiftKey);
        break;
      case "End":
        ev.preventDefault();
        setFocusIndex(rows.length);
        moveFocus(-1, ev.shiftKey);
        break;
      case "Enter": {
        const entry = rows[focusIndex()];
        if (entry) {
          ev.preventDefault();
          openEntry(entry);
        }
        break;
      }
      case " ": {
        const entry = rows[focusIndex()];
        if (entry) {
          ev.preventDefault();
          toggle(entry.key);
        }
        break;
      }
      case "a":
        if (ev.metaKey || ev.ctrlKey) {
          ev.preventDefault();
          setSelected(new Set(rows.map((e) => e.key)));
        }
        break;
      case "Escape":
        ev.preventDefault();
        clearSelection();
        break;
    }
  };

  const setSort = (key: SortKey) => {
    if (sortKey() === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const openEntry = (e: ObjectEntry) => {
    if (e.isFolder) {
      navigatePrefix(props.tab.id, e.key);
    } else {
      setPreviewKey(e.key);
    }
  };

  /* ------------------------------- operations ------------------------------ */

  const enqueueUpload = async (paths: string[]) => {
    if (!paths.length) return;
    try {
      const n = await QueueService.EnqueueUpload(
        wid,
        props.tab.connectionId,
        bucket(),
        prefix(),
        paths,
        0
      );
      toast.success(t("storage.enqueued", { count: n }));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const uploadFiles = async () => {
    const paths = await AppPickFiles();
    await enqueueUpload(paths);
  };
  const uploadFolders = async () => {
    const paths = await AppPickFolders();
    await enqueueUpload(paths);
  };

  const doDownload = async (destDir: string, setDefault: boolean) => {
    const keys = downloadKeys();
    if (!keys) return;
    try {
      const n = await QueueService.EnqueueDownload(
        wid,
        props.tab.connectionId,
        bucket(),
        keys,
        destDir,
        0
      );
      if (setDefault) {
        updateSettings({ defaultDownloadDir: destDir });
      }
      toast.success(t("storage.enqueued", { count: n }));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const doCopyMove = async (destConnId: string, destBucket: string, destPrefix: string) => {
    const cm = copyMove();
    if (!cm) return;
    try {
      const n = await QueueService.EnqueueCopy(
        wid,
        props.tab.connectionId,
        bucket(),
        cm.keys,
        destConnId,
        destBucket,
        destPrefix,
        cm.move,
        0
      );
      toast.success(t("storage.enqueued", { count: n }));
      clearSelection();
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const doDelete = async () => {
    const keys = deleteKeys();
    if (!keys) return;
    try {
      const n = await QueueService.EnqueueDelete(wid, props.tab.connectionId, bucket(), keys, 0);
      toast.success(t("storage.enqueued", { count: n }));
      clearSelection();
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setDeleteKeys(null);
    }
  };

  /**
   * S3 has no rename, so this enqueues a move: the object (or every object under
   * a folder prefix) is copied to the new name and the original deleted.
   */
  const doRename = async (newName: string) => {
    const entry = renameEntry();
    if (!entry) return;
    try {
      const n = await QueueService.EnqueueRename(
        wid,
        props.tab.connectionId,
        bucket(),
        entry.key,
        newName,
        0
      );
      if (n > 0) toast.success(t("storage.enqueued", { count: n }));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setRenameEntry(null);
    }
  };

  const createFolder = async (name: string) => {
    try {
      await S3Service.CreateFolder(props.tab.connectionId, bucket(), prefix(), name);
      toast.success(t("common.success"));
      load(true);
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const selectedKeys = () => Array.from(selected());

  /** Copy text through the native clipboard and confirm it visibly. */
  const copy = async (text: string) => {
    try {
      await AppService.CopyToClipboard(text);
      toast.success(t("storage.copied"));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  /* ------------------------------- bookmarks ------------------------------- */

  const currentBookmark = () => findBookmark(props.tab.connectionId, bucket(), prefix());

  const toggleBookmark = async () => {
    try {
      const existing = currentBookmark();
      if (existing) {
        await removeBookmark(existing.id);
        return;
      }
      await addBookmark({
        connectionId: props.tab.connectionId,
        bucket: bucket(),
        prefix: prefix(),
        label: "",
      });
      toast.success(t("storage.bookmarkAdded"));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  /** Bookmarks that belong to the connection this tab is browsing. */
  const tabBookmarks = () => bookmarks().filter((b) => b.connectionId === props.tab.connectionId);

  // Row action menu items, reused by the kebab dropdown and the context menu.
  const RowActions: Component<{ entry: ObjectEntry; Item: any; Separator: any }> = (p) => (
    <>
      <Show when={!p.entry.isFolder}>
        <p.Item onSelect={() => setPreviewKey(p.entry.key)}>
          <Eye class="h-4 w-4" />
          {t("storage.preview")}
        </p.Item>
      </Show>
      <p.Item onSelect={() => setDownloadKeys([p.entry.key])}>
        <Download class="h-4 w-4" />
        {t("common.download")}
      </p.Item>
      <Show when={!p.entry.isFolder}>
        <p.Item onSelect={() => setPropsKey(p.entry.key)}>
          <Info class="h-4 w-4" />
          {t("storage.properties")}
        </p.Item>
        <p.Item onSelect={() => setPresignKey(p.entry.key)}>
          <LinkIcon class="h-4 w-4" />
          {t("storage.presign")}
        </p.Item>
        <p.Item onSelect={() => setTagsKey(p.entry.key)}>
          <TagIcon class="h-4 w-4" />
          {t("storage.tags")}
        </p.Item>
        <p.Item onSelect={() => setObjSettingsKey(p.entry.key)}>
          <SlidersHorizontal class="h-4 w-4" />
          {t("storage.objectSettings")}
        </p.Item>
        <p.Item onSelect={() => setVersionsKey(p.entry.key)}>
          <History class="h-4 w-4" />
          {t("storage.versions")}
        </p.Item>
      </Show>
      <p.Item onSelect={() => copy(p.entry.key)}>
        <ClipboardCopy class="h-4 w-4" />
        {t("storage.copyKey")}
      </p.Item>
      <p.Item onSelect={() => copy(`s3://${bucket()}/${p.entry.key}`)}>
        <ClipboardCopy class="h-4 w-4" />
        {t("storage.copyUri")}
      </p.Item>
      <p.Separator />
      <p.Item onSelect={() => setRenameEntry(p.entry)}>
        <PenLine class="h-4 w-4" />
        {t("common.rename")}
      </p.Item>
      <p.Item onSelect={() => setCopyMove({ keys: [p.entry.key], move: false })}>
        <Copy class="h-4 w-4" />
        {t("storage.copyTo")}
      </p.Item>
      <p.Item onSelect={() => setCopyMove({ keys: [p.entry.key], move: true })}>
        <FolderInput class="h-4 w-4" />
        {t("storage.moveTo")}
      </p.Item>
      <p.Separator />
      <p.Item destructive onSelect={() => setDeleteKeys([p.entry.key])}>
        <Trash2 class="h-4 w-4" />
        {t("common.delete")}
      </p.Item>
    </>
  );

  const SortHeader: Component<{ label: string; k: SortKey; class?: string }> = (p) => (
    <button
      class={cn("flex items-center gap-1 hover:text-foreground", p.class)}
      onClick={() => setSort(p.k)}
    >
      {p.label}
      <Show when={sortKey() === p.k}>
        <ArrowUpDown class="h-3 w-3" />
      </Show>
    </button>
  );

  return (
    <div class="relative flex h-full flex-col" data-file-drop-target>
      {/* Shown by the Wails runtime, which toggles .file-drop-target-active on
          the element carrying data-file-drop-target while files are dragged. */}
      <div class="drop-overlay">
        <div class="drop-overlay-card">
          <Upload class="h-6 w-6" />
          {t("storage.dropHint")}
        </div>
      </div>
      {/* Toolbar */}
      <div class="flex flex-wrap items-center gap-2 border-b px-3 py-2">
        <Button
          size="icon-sm"
          variant="ghost"
          onClick={() => openBucket(props.tab.id, null)}
          title={t("storage.buckets")}
        >
          <ChevronLeft class="h-4 w-4" />
        </Button>

        {/* Breadcrumb */}
        <div class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto no-scrollbar text-sm">
          <button
            class="flex items-center gap-1 font-medium hover:text-primary"
            onClick={() => navigatePrefix(props.tab.id, "")}
          >
            <Home class="h-3.5 w-3.5" />
            {bucket()}
          </button>
          <For each={breadcrumbSegments(prefix())}>
            {(seg) => (
              <>
                <ChevronRight class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <button
                  class="shrink-0 hover:text-primary"
                  onClick={() => navigatePrefix(props.tab.id, seg.prefix)}
                >
                  {seg.name}
                </button>
              </>
            )}
          </For>
        </div>

        <div class="flex items-center gap-2">
          <div class="relative">
            <Search class="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              ref={filterInput}
              class="h-8 w-44 pl-7"
              placeholder={t("storage.filter")}
              value={filter()}
              onInput={(e) => {
                setFilter(e.currentTarget.value);
                if (searchResults() !== null) clearSearch();
              }}
              onKeyDown={(e) => e.key === "Enter" && runSearch()}
            />
          </div>
          <Show
            when={!searching()}
            fallback={
              <Button
                size="icon-sm"
                variant="outline"
                onClick={searchOp.cancel}
                title={t("common.cancel")}
              >
                <XIcon class="h-3.5 w-3.5 animate-pulse" />
              </Button>
            }
          >
            <Button
              size="icon-sm"
              variant="outline"
              onClick={runSearch}
              title={t("storage.deepSearch")}
            >
              <Search class="h-3.5 w-3.5" />
            </Button>
          </Show>
          <Button
            size="icon-sm"
            variant="outline"
            onClick={() => setStatsOpen(true)}
            title={t("storage.stats")}
          >
            <BarChart3 class="h-3.5 w-3.5" />
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setJumpOpen(true)}
            title={t("storage.goToPath")}
          >
            {t("storage.goToPath")}
          </Button>
          <Button
            size="icon-sm"
            variant="outline"
            onClick={() => load(true)}
            title={t("storage.refreshList")}
          >
            <RefreshCw class={cn("h-3.5 w-3.5", loading() && "animate-spin")} />
          </Button>
          <Button
            size="icon-sm"
            variant="outline"
            onClick={() => setCapsOpen(true)}
            title={t("capabilities.title")}
          >
            <ShieldCheck class="h-3.5 w-3.5" />
          </Button>
          <Button
            size="icon-sm"
            variant="outline"
            onClick={() => setMultipartOpen(true)}
            title={t("multipart.title")}
          >
            <Boxes class="h-3.5 w-3.5" />
          </Button>
          <Button
            size="icon-sm"
            variant="outline"
            onClick={toggleBookmark}
            title={currentBookmark() ? t("storage.bookmarkRemove") : t("storage.bookmarkAdd")}
          >
            <Star class={cn("h-3.5 w-3.5", currentBookmark() && "fill-current text-amber-500")} />
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger
              class={cn(buttonVariants({ size: "icon-sm", variant: "outline" }))}
              title={t("storage.bookmarks")}
            >
              <BookMarked class="h-3.5 w-3.5" />
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              <Show
                when={tabBookmarks().length}
                fallback={
                  <div class="px-2 py-3 text-center text-xs text-muted-foreground">
                    {t("storage.bookmarkEmpty")}
                  </div>
                }
              >
                <For each={tabBookmarks()}>
                  {(b) => (
                    <DropdownMenuItem
                      onSelect={() => {
                        if (b.bucket !== bucket()) openBucket(props.tab.id, b.bucket);
                        navigatePrefix(props.tab.id, b.prefix);
                      }}
                    >
                      <BookMarked class="h-4 w-4" />
                      <span class="truncate">{b.label}</span>
                    </DropdownMenuItem>
                  )}
                </For>
              </Show>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button size="sm" variant="outline" onClick={() => setNewFolderOpen(true)}>
            <FolderPlus class="h-3.5 w-3.5" />
            {t("storage.newFolder")}
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger class={cn(buttonVariants({ size: "sm" }), "gap-1.5")}>
              <Upload class="h-3.5 w-3.5" />
              {t("common.upload")}
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              <DropdownMenuItem onSelect={uploadFiles}>
                <FileIcon class="h-4 w-4" />
                {t("storage.uploadFiles")}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={uploadFolders}>
                <Folder class="h-4 w-4" />
                {t("storage.uploadFolders")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Search-results banner */}
      <Show when={searchResults() !== null}>
        <div class="flex items-center gap-2 border-b bg-primary/10 px-3 py-1.5 text-sm">
          <Search class="h-3.5 w-3.5" />
          <span>{t("storage.searchResults", { count: visible().length })}</span>
          <Show when={searchTruncated()}>
            <span class="text-xs text-amber-500">{t("storage.searchTruncated")}</span>
          </Show>
          <Button
            size="icon-sm"
            variant="ghost"
            class="ml-auto"
            onClick={clearSearch}
            title={t("common.close")}
          >
            <XIcon class="h-3.5 w-3.5" />
          </Button>
        </div>
      </Show>

      {/* Bulk action bar */}
      <Show when={selected().size > 0}>
        <div class="flex items-center gap-2 border-b bg-accent/40 px-3 py-1.5 text-sm">
          <span class="font-medium">{t("common.selected", { count: selected().size })}</span>
          <div class="ml-auto flex items-center gap-2">
            <Button size="sm" variant="outline" onClick={() => setDownloadKeys(selectedKeys())}>
              <Download class="h-3.5 w-3.5" />
              {t("common.download")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setBatchTagKeys(selectedKeys())}>
              <TagIcon class="h-3.5 w-3.5" />
              {t("storage.tagSelected")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setCopyMove({ keys: selectedKeys(), move: false })}
            >
              <Copy class="h-3.5 w-3.5" />
              {t("storage.copyTo")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => setCopyMove({ keys: selectedKeys(), move: true })}
            >
              <FolderInput class="h-3.5 w-3.5" />
              {t("storage.moveTo")}
            </Button>
            <Button size="sm" variant="destructive" onClick={() => setDeleteKeys(selectedKeys())}>
              <Trash2 class="h-3.5 w-3.5" />
              {t("storage.deleteSelected")}
            </Button>
          </div>
        </div>
      </Show>

      {/* Column header */}
      <div class="grid grid-cols-[2rem_1fr_7rem_11rem_6rem_2.5rem] items-center gap-2 border-b px-3 py-1.5 text-xs text-muted-foreground">
        <Checkbox
          aria-label={t("storage.selectAll")}
          checked={allSelected()}
          onChange={toggleAll}
        />
        <SortHeader label={t("storage.colName")} k="name" />
        <SortHeader label={t("storage.colSize")} k="size" class="justify-end" />
        <SortHeader label={t("storage.colModified")} k="modified" />
        <span>{t("storage.colStorage")}</span>
        <span />
      </div>

      {/* Rows */}
      <div
        ref={setListEl}
        class="flex-1 overflow-y-auto outline-none"
        tabindex="0"
        role="listbox"
        aria-multiselectable="true"
        aria-label={t("storage.objects")}
        onKeyDown={onListKeyDown}
      >
        <Show
          when={!loading() || entries().length > 0}
          fallback={
            <div class="flex justify-center py-12">
              <Spinner class="h-6 w-6" />
            </div>
          }
        >
          <Show
            when={visible().length}
            fallback={
              <div class="py-16 text-center text-sm text-muted-foreground">
                {t("storage.empty")}
              </div>
            }
          >
            {/* Virtualised: a bucket page holds up to 1000 rows and "load more"
                keeps appending, so only the visible window is ever in the DOM. */}
            <div
              style={{
                height: `${rowVirtualizer.getTotalSize()}px`,
                position: "relative",
                width: "100%",
              }}
            >
              <For each={rowVirtualizer.getVirtualItems()}>
                {(virtualRow) => {
                  const entry = () => visible()[virtualRow.index];
                  const index = () => virtualRow.index;
                  return (
                    <Show when={entry()}>
                      <ContextMenu>
                        <ContextMenuTrigger
                          as="div"
                          data-row-index={index()}
                          role="option"
                          aria-selected={selected().has(entry().key)}
                          style={{
                            position: "absolute",
                            top: 0,
                            left: 0,
                            width: "100%",
                            height: `${ROW_HEIGHT}px`,
                            transform: `translateY(${virtualRow.start}px)`,
                          }}
                          class={cn(
                            "grid cursor-default select-none grid-cols-[2rem_1fr_7rem_11rem_6rem_2.5rem] items-center gap-2 border-b px-3 text-sm hover:bg-accent/40",
                            selected().has(entry().key) && "bg-accent/60",
                            focusIndex() === index() && "ring-1 ring-inset ring-ring"
                          )}
                          onClick={(ev: MouseEvent) => onRowClick(index(), ev)}
                          onDblClick={() => openEntry(entry())}
                          onContextMenu={() => {
                            // Right-clicking outside the current selection targets
                            // just that row, matching every native file manager.
                            if (!selected().has(entry().key)) {
                              setSelected(new Set([entry().key]));
                              setFocusIndex(index());
                              anchor = index();
                            }
                          }}
                        >
                          <div onClick={(ev: MouseEvent) => ev.stopPropagation()}>
                            <Checkbox
                              aria-label={entry().name}
                              checked={selected().has(entry().key)}
                              onChange={() => toggle(entry().key)}
                            />
                          </div>
                          <div class="flex min-w-0 items-center gap-2">
                            <Show
                              when={entry().isFolder}
                              fallback={<FileIcon class="h-4 w-4 shrink-0 text-muted-foreground" />}
                            >
                              <Folder class="h-4 w-4 shrink-0 text-primary" />
                            </Show>
                            <span class="truncate" title={entry().key}>
                              {entry().name}
                            </span>
                          </div>
                          <span class="text-right text-muted-foreground">
                            {entry().isFolder ? "—" : formatBytes(entry().size)}
                          </span>
                          <span class="text-muted-foreground">
                            {entry().isFolder
                              ? ""
                              : formatDate(entry().lastModified, effectiveLocale())}
                          </span>
                          <span class="truncate text-xs text-muted-foreground">
                            {entry().storageClass}
                          </span>
                          <DropdownMenu>
                            <DropdownMenuTrigger
                              class="flex h-7 w-7 items-center justify-center rounded hover:bg-accent"
                              onClick={(ev: MouseEvent) => ev.stopPropagation()}
                            >
                              <MoreVertical class="h-4 w-4" />
                            </DropdownMenuTrigger>
                            <DropdownMenuContent>
                              <RowActions
                                entry={entry()}
                                Item={DropdownMenuItem}
                                Separator={DropdownMenuSeparator}
                              />
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </ContextMenuTrigger>
                        <ContextMenuContent>
                          <RowActions
                            entry={entry()}
                            Item={ContextMenuItem}
                            Separator={ContextMenuSeparator}
                          />
                        </ContextMenuContent>
                      </ContextMenu>
                    </Show>
                  );
                }}
              </For>
            </div>
            <Show when={nextToken()}>
              <div class="flex justify-center p-3">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => load(false)}
                  disabled={loading()}
                >
                  {t("storage.loadMore")}
                </Button>
              </div>
            </Show>
          </Show>
        </Show>
      </div>

      {/* Dialogs */}
      <PromptDialog
        open={newFolderOpen()}
        onOpenChange={setNewFolderOpen}
        title={t("storage.newFolderTitle")}
        placeholder={t("storage.folderName")}
        onSubmit={createFolder}
      />
      <PromptDialog
        open={jumpOpen()}
        onOpenChange={setJumpOpen}
        title={t("storage.goToPath")}
        placeholder="path/to/folder/"
        initial={prefix()}
        confirmText={t("common.ok")}
        onSubmit={(v) => navigatePrefix(props.tab.id, normalizePrefix(v))}
      />
      <DownloadDialog
        open={!!downloadKeys()}
        onOpenChange={(o) => !o && setDownloadKeys(null)}
        count={downloadKeys()?.length ?? 0}
        onConfirm={doDownload}
      />
      <DestinationDialog
        open={!!copyMove()}
        onOpenChange={(o) => !o && setCopyMove(null)}
        move={copyMove()?.move ?? false}
        count={copyMove()?.keys.length ?? 0}
        buckets={buckets()}
        currentConnId={props.tab.connectionId}
        defaultBucket={bucket()}
        defaultPrefix={prefix()}
        onConfirm={doCopyMove}
      />
      <ConfirmDialog
        open={!!deleteKeys()}
        onOpenChange={(o) => !o && setDeleteKeys(null)}
        title={t("storage.batchConfirmTitle")}
        message={t("storage.batchDeleteMessage", { count: deleteKeys()?.length ?? 0 })}
        confirmText={t("common.delete")}
        destructive
        onConfirm={doDelete}
      />
      <Show when={previewKey()}>
        <PreviewDialog
          open
          onOpenChange={(o) => !o && setPreviewKey(null)}
          connId={props.tab.connectionId}
          bucket={bucket()}
          objKey={previewKey()!}
        />
      </Show>
      <Show when={propsKey()}>
        <PropertiesDialog
          open
          onOpenChange={(o) => !o && setPropsKey(null)}
          connId={props.tab.connectionId}
          bucket={bucket()}
          objKey={propsKey()!}
        />
      </Show>
      <Show when={presignKey()}>
        <PresignDialog
          open
          onOpenChange={(o) => !o && setPresignKey(null)}
          connId={props.tab.connectionId}
          bucket={bucket()}
          objKey={presignKey()!}
        />
      </Show>
      <Show when={tagsKey()}>
        <TagsDialog
          open
          onOpenChange={(o) => !o && setTagsKey(null)}
          connId={props.tab.connectionId}
          bucket={bucket()}
          objKey={tagsKey()!}
        />
      </Show>
      <Show when={objSettingsKey()}>
        <ObjectSettingsDialog
          open
          onOpenChange={(o) => !o && setObjSettingsKey(null)}
          connId={props.tab.connectionId}
          bucket={bucket()}
          objKey={objSettingsKey()!}
        />
      </Show>
      <Show when={versionsKey()}>
        <VersionsDialog
          open
          onOpenChange={(o) => !o && setVersionsKey(null)}
          connId={props.tab.connectionId}
          bucket={bucket()}
          objKey={versionsKey()!}
        />
      </Show>
      <CapabilitiesDialog
        open={capsOpen()}
        onOpenChange={setCapsOpen}
        connId={props.tab.connectionId}
        bucket={bucket()}
      />
      <Show when={renameEntry()}>
        {(entry) => (
          <PromptDialog
            open
            onOpenChange={(o) => !o && setRenameEntry(null)}
            title={t("storage.renameTitle")}
            placeholder={t("storage.newName")}
            initial={keyBasename(entry().key)}
            confirmText={t("common.rename")}
            onSubmit={doRename}
          />
        )}
      </Show>
      <Show when={batchTagKeys()}>
        {(keys) => (
          <TagsDialog
            open
            onOpenChange={(o) => !o && setBatchTagKeys(null)}
            connId={props.tab.connectionId}
            bucket={bucket()}
            objKeys={keys()}
          />
        )}
      </Show>
      <MultipartCleanupDialog
        open={multipartOpen()}
        onOpenChange={setMultipartOpen}
        connId={props.tab.connectionId}
        bucket={bucket()}
      />
      <StatsDialog
        open={statsOpen()}
        onOpenChange={setStatsOpen}
        connId={props.tab.connectionId}
        bucket={bucket()}
        prefix={prefix()}
      />
    </div>
  );
};

async function AppPickFiles(): Promise<string[]> {
  try {
    return (await AppService.PickFiles(wid)) ?? [];
  } catch {
    return [];
  }
}
async function AppPickFolders(): Promise<string[]> {
  try {
    return (await AppService.PickFolders(wid)) ?? [];
  } catch {
    return [];
  }
}
