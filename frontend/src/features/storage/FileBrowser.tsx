import { createSignal, createEffect, createMemo, on, For, Show, onMount, onCleanup, type Component } from "solid-js";
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
} from "lucide-solid";
import { Button, buttonVariants } from "~/components/ui/button";
import { Input, Checkbox, Spinner } from "~/components/ui/primitives";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator } from "~/components/ui/dropdown-menu";
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem, ContextMenuSeparator } from "~/components/ui/context-menu";
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
import { S3Service, QueueService, AppService, windowID, onEvent, type ObjectEntry, type BucketInfo } from "~/lib/api";
import { navigatePrefix, openBucket, type Tab } from "~/stores/tabs";
import { filterEntries, sortEntries, breadcrumbSegments, normalizePrefix, type SortKey, type SortDir } from "./objects";
import { formatBytes, formatDate } from "~/lib/utils";
import { effectiveLocale, updateSettings } from "~/stores/settings";
import { toast } from "~/components/ui/toast";
import { cn } from "~/lib/utils";
import * as bus from "~/lib/bus";
import { t } from "~/i18n";

const wid = windowID();

export const FileBrowser: Component<{ tab: Tab }> = (props) => {
  const [entries, setEntries] = createSignal<ObjectEntry[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [nextToken, setNextToken] = createSignal("");
  const [filter, setFilter] = createSignal("");
  const [searchResults, setSearchResults] = createSignal<ObjectEntry[] | null>(null);
  const [searchTruncated, setSearchTruncated] = createSignal(false);
  const [searching, setSearching] = createSignal(false);
  const [statsOpen, setStatsOpen] = createSignal(false);
  const [sortKey, setSortKey] = createSignal<SortKey>("name");
  const [sortDir, setSortDir] = createSignal<SortDir>("asc");
  const [selected, setSelected] = createSignal<Set<string>>(new Set<string>());
  const [buckets, setBuckets] = createSignal<BucketInfo[]>([]);
  let filterInput: HTMLInputElement | undefined;

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
        setSelected(new Set<string>());
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

  const runSearch = async () => {
    const q = filter().trim();
    if (!q) {
      setSearchResults(null);
      return;
    }
    setSearching(true);
    try {
      const res = await S3Service.Search(props.tab.connectionId, bucket(), prefix(), q, 1000);
      setSearchResults(res.entries ?? []);
      setSearchTruncated(res.truncated ?? false);
      setSelected(new Set<string>());
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setSearching(false);
    }
  };

  const clearSearch = () => {
    setSearchResults(null);
    setSearchTruncated(false);
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
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });
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
      const n = await QueueService.EnqueueUpload(wid, props.tab.connectionId, bucket(), prefix(), paths, 0);
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
      const n = await QueueService.EnqueueDownload(wid, props.tab.connectionId, bucket(), keys, destDir, 0);
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
      setSelected(new Set<string>());
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
      setSelected(new Set<string>());
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setDeleteKeys(null);
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
      <p.Separator />
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
    <button class={cn("flex items-center gap-1 hover:text-foreground", p.class)} onClick={() => setSort(p.k)}>
      {p.label}
      <Show when={sortKey() === p.k}>
        <ArrowUpDown class="h-3 w-3" />
      </Show>
    </button>
  );

  return (
    <div class="flex h-full flex-col" data-file-drop-target>
      {/* Toolbar */}
      <div class="flex flex-wrap items-center gap-2 border-b px-3 py-2">
        <Button size="icon-sm" variant="ghost" onClick={() => openBucket(props.tab.id, null)} title={t("storage.buckets")}>
          <ChevronLeft class="h-4 w-4" />
        </Button>

        {/* Breadcrumb */}
        <div class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto no-scrollbar text-sm">
          <button class="flex items-center gap-1 font-medium hover:text-primary" onClick={() => navigatePrefix(props.tab.id, "")}>
            <Home class="h-3.5 w-3.5" />
            {bucket()}
          </button>
          <For each={breadcrumbSegments(prefix())}>
            {(seg) => (
              <>
                <ChevronRight class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <button class="shrink-0 hover:text-primary" onClick={() => navigatePrefix(props.tab.id, seg.prefix)}>
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
          <Button size="icon-sm" variant="outline" onClick={runSearch} title={t("storage.deepSearch")} disabled={searching()}>
            <Search class={cn("h-3.5 w-3.5", searching() && "animate-pulse")} />
          </Button>
          <Button size="icon-sm" variant="outline" onClick={() => setStatsOpen(true)} title={t("storage.stats")}>
            <BarChart3 class="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" variant="outline" onClick={() => setJumpOpen(true)} title={t("storage.goToPath")}>
            {t("storage.goToPath")}
          </Button>
          <Button size="icon-sm" variant="outline" onClick={() => load(true)} title={t("storage.refreshList")}>
            <RefreshCw class={cn("h-3.5 w-3.5", loading() && "animate-spin")} />
          </Button>
          <Button size="icon-sm" variant="outline" onClick={() => setCapsOpen(true)} title={t("capabilities.title")}>
            <ShieldCheck class="h-3.5 w-3.5" />
          </Button>
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
          <Button size="icon-sm" variant="ghost" class="ml-auto" onClick={clearSearch} title={t("common.close")}>
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
            <Button size="sm" variant="outline" onClick={() => setCopyMove({ keys: selectedKeys(), move: false })}>
              <Copy class="h-3.5 w-3.5" />
              {t("storage.copyTo")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => setCopyMove({ keys: selectedKeys(), move: true })}>
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
        <Checkbox aria-label={t("storage.selectAll")} checked={allSelected()} onChange={toggleAll} />
        <SortHeader label={t("storage.colName")} k="name" />
        <SortHeader label={t("storage.colSize")} k="size" class="justify-end" />
        <SortHeader label={t("storage.colModified")} k="modified" />
        <span>{t("storage.colStorage")}</span>
        <span />
      </div>

      {/* Rows */}
      <div class="flex-1 overflow-y-auto">
        <Show when={!loading() || entries().length > 0} fallback={<div class="flex justify-center py-12"><Spinner class="h-6 w-6" /></div>}>
          <Show
            when={visible().length}
            fallback={<div class="py-16 text-center text-sm text-muted-foreground">{t("storage.empty")}</div>}
          >
            <For each={visible()}>
              {(e) => (
                <ContextMenu>
                  <ContextMenuTrigger
                    as="div"
                    class={cn(
                      "grid grid-cols-[2rem_1fr_7rem_11rem_6rem_2.5rem] items-center gap-2 border-b px-3 py-1.5 text-sm hover:bg-accent/40",
                      selected().has(e.key) && "bg-accent/60"
                    )}
                  >
                    <Checkbox checked={selected().has(e.key)} onChange={() => toggle(e.key)} />
                    <button class="flex min-w-0 items-center gap-2 text-left" onDblClick={() => openEntry(e)} onClick={() => openEntry(e)}>
                      <Show when={e.isFolder} fallback={<FileIcon class="h-4 w-4 shrink-0 text-muted-foreground" />}>
                        <Folder class="h-4 w-4 shrink-0 text-primary" />
                      </Show>
                      <span class="truncate">{e.name}</span>
                    </button>
                    <span class="text-right text-muted-foreground">{e.isFolder ? "—" : formatBytes(e.size)}</span>
                    <span class="text-muted-foreground">{e.isFolder ? "" : formatDate(e.lastModified, effectiveLocale())}</span>
                    <span class="truncate text-xs text-muted-foreground">{e.storageClass}</span>
                    <DropdownMenu>
                      <DropdownMenuTrigger class="flex h-7 w-7 items-center justify-center rounded hover:bg-accent" onClick={(ev: MouseEvent) => ev.stopPropagation()}>
                        <MoreVertical class="h-4 w-4" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent>
                        <RowActions entry={e} Item={DropdownMenuItem} Separator={DropdownMenuSeparator} />
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </ContextMenuTrigger>
                  <ContextMenuContent>
                    <RowActions entry={e} Item={ContextMenuItem} Separator={ContextMenuSeparator} />
                  </ContextMenuContent>
                </ContextMenu>
              )}
            </For>
            <Show when={nextToken()}>
              <div class="flex justify-center p-3">
                <Button variant="outline" size="sm" onClick={() => load(false)} disabled={loading()}>
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
        <PreviewDialog open onOpenChange={(o) => !o && setPreviewKey(null)} connId={props.tab.connectionId} bucket={bucket()} objKey={previewKey()!} />
      </Show>
      <Show when={propsKey()}>
        <PropertiesDialog open onOpenChange={(o) => !o && setPropsKey(null)} connId={props.tab.connectionId} bucket={bucket()} objKey={propsKey()!} />
      </Show>
      <Show when={presignKey()}>
        <PresignDialog open onOpenChange={(o) => !o && setPresignKey(null)} connId={props.tab.connectionId} bucket={bucket()} objKey={presignKey()!} />
      </Show>
      <Show when={tagsKey()}>
        <TagsDialog open onOpenChange={(o) => !o && setTagsKey(null)} connId={props.tab.connectionId} bucket={bucket()} objKey={tagsKey()!} />
      </Show>
      <Show when={objSettingsKey()}>
        <ObjectSettingsDialog open onOpenChange={(o) => !o && setObjSettingsKey(null)} connId={props.tab.connectionId} bucket={bucket()} objKey={objSettingsKey()!} />
      </Show>
      <Show when={versionsKey()}>
        <VersionsDialog open onOpenChange={(o) => !o && setVersionsKey(null)} connId={props.tab.connectionId} bucket={bucket()} objKey={versionsKey()!} />
      </Show>
      <CapabilitiesDialog open={capsOpen()} onOpenChange={setCapsOpen} connId={props.tab.connectionId} bucket={bucket()} />
      <StatsDialog open={statsOpen()} onOpenChange={setStatsOpen} connId={props.tab.connectionId} bucket={bucket()} prefix={prefix()} />
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
