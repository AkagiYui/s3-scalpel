import { createSignal, createEffect, For, Show, type Component, type JSX } from "solid-js";
import { Plus, Trash2, Globe, Lock } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input, Label, Spinner, Badge } from "~/components/ui/primitives";
import { SimpleSelect } from "~/components/ui/select";
import { S3Service, type ObjectACL, type ObjectMetaUpdate, type Tag } from "~/lib/api";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";
import { cn } from "~/lib/utils";

type TabKey = "metadata" | "permissions" | "archive";

const STORAGE_CLASSES = ["STANDARD", "STANDARD_IA", "ONEZONE_IA", "INTELLIGENT_TIERING", "GLACIER", "DEEP_ARCHIVE", "GLACIER_IR", "REDUCED_REDUNDANCY"];
const CANNED_ACLS = ["private", "public-read", "public-read-write", "authenticated-read", "bucket-owner-read", "bucket-owner-full-control"];
const RESTORE_TIERS = ["Standard", "Expedited", "Bulk"];

export const ObjectSettingsDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
  objKey: string;
}> = (props) => {
  const [tab, setTab] = createSignal<TabKey>("metadata");
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);

  const [contentType, setContentType] = createSignal("");
  const [cacheControl, setCacheControl] = createSignal("");
  const [contentDisposition, setContentDisposition] = createSignal("");
  const [contentEncoding, setContentEncoding] = createSignal("");
  const [storageClass, setStorageClass] = createSignal("");
  const [meta, setMeta] = createSignal<Tag[]>([]);

  const [acl, setAcl] = createSignal<ObjectACL | null>(null);
  const [cannedAcl, setCannedAcl] = createSignal("private");

  const [restoreDays, setRestoreDays] = createSignal(7);
  const [restoreTier, setRestoreTier] = createSignal("Standard");

  createEffect(() => {
    if (props.open) {
      setTab("metadata");
      load();
    }
  });

  const load = async () => {
    setLoading(true);
    try {
      const [props2, aclData] = await Promise.all([
        S3Service.Properties(props.connId, props.bucket, props.objKey, "").catch(() => null),
        S3Service.GetACL(props.connId, props.bucket, props.objKey).catch(() => null),
      ]);
      if (props2) {
        setContentType(props2.contentType ?? "");
        setCacheControl(props2.cacheControl ?? "");
        setContentDisposition(props2.contentDisposition ?? "");
        setContentEncoding(props2.contentEncoding ?? "");
        setStorageClass(props2.storageClass ?? "");
        const m = props2.metadata ?? {};
        setMeta(Object.keys(m).map((k) => ({ key: k, value: m[k] }) as Tag));
      }
      if (aclData) setAcl(aclData as ObjectACL);
    } finally {
      setLoading(false);
    }
  };

  const run = async (fn: () => Promise<void>, okMsg?: string) => {
    setSaving(true);
    try {
      await fn();
      toast.success(okMsg ?? t("common.success"));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    } finally {
      setSaving(false);
    }
  };

  const saveMetadata = () => {
    const metadata: Record<string, string> = {};
    for (const m of meta()) if (m.key.trim()) metadata[m.key.trim()] = m.value;
    const update: ObjectMetaUpdate = {
      contentType: contentType(),
      cacheControl: cacheControl(),
      contentDisposition: contentDisposition(),
      contentEncoding: contentEncoding(),
      storageClass: storageClass(),
      metadata,
    } as ObjectMetaUpdate;
    run(() => S3Service.UpdateMetadata(props.connId, props.bucket, props.objKey, update as any) as Promise<void>, t("objectSettings.metadataSaved"));
  };

  const applyAcl = () =>
    run(async () => {
      await S3Service.SetACL(props.connId, props.bucket, props.objKey, cannedAcl());
      const fresh = await S3Service.GetACL(props.connId, props.bucket, props.objKey).catch(() => null);
      if (fresh) setAcl(fresh as ObjectACL);
    }, t("objectSettings.aclApplied"));

  const doRestore = () =>
    run(() => S3Service.Restore(props.connId, props.bucket, props.objKey, restoreDays(), restoreTier()) as Promise<void>, t("objectSettings.restoreStarted"));

  const TabButton: Component<{ id: TabKey; label: string }> = (p) => (
    <button
      class={cn("rounded-md px-3 py-1.5 text-sm transition-colors", tab() === p.id ? "bg-primary text-primary-foreground" : "hover:bg-accent")}
      onClick={() => setTab(p.id)}
    >
      {p.label}
    </button>
  );

  const Field: Component<{ label: string; children: JSX.Element }> = (p) => (
    <div class="grid grid-cols-[10rem_1fr] items-center gap-2">
      <Label>{p.label}</Label>
      {p.children}
    </div>
  );

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{t("objectSettings.title")}</DialogTitle>
        </DialogHeader>
        <div class="truncate font-mono text-xs text-muted-foreground">{props.objKey}</div>

        <div class="flex gap-1 border-b pb-2">
          <TabButton id="metadata" label={t("objectSettings.metadata")} />
          <TabButton id="permissions" label={t("objectSettings.permissions")} />
          <TabButton id="archive" label={t("objectSettings.archive")} />
        </div>

        <Show when={!loading()} fallback={<div class="flex justify-center py-10"><Spinner class="h-6 w-6" /></div>}>
          <div class="min-h-[16rem] py-1 text-sm">
            {/* Metadata --------------------------------------------------- */}
            <Show when={tab() === "metadata"}>
              <div class="flex flex-col gap-3">
                <Field label={t("properties.contentType")}>
                  <Input value={contentType()} onInput={(e) => setContentType(e.currentTarget.value)} />
                </Field>
                <Field label={t("properties.cacheControl")}>
                  <Input value={cacheControl()} onInput={(e) => setCacheControl(e.currentTarget.value)} />
                </Field>
                <Field label={t("objectSettings.contentDisposition")}>
                  <Input value={contentDisposition()} onInput={(e) => setContentDisposition(e.currentTarget.value)} />
                </Field>
                <Field label={t("properties.contentEncoding")}>
                  <Input value={contentEncoding()} onInput={(e) => setContentEncoding(e.currentTarget.value)} />
                </Field>
                <Field label={t("properties.storageClass")}>
                  <SimpleSelect value={storageClass()} placeholder={t("bucketSettings.storageClass")}
                    options={[{ value: "", label: t("common.none") }, ...STORAGE_CLASSES.map((c) => ({ value: c, label: c }))]}
                    onChange={setStorageClass} />
                </Field>
                <div class="mt-1">
                  <Label>{t("properties.metadata")}</Label>
                  <div class="mt-1 flex flex-col gap-2">
                    <For each={meta()} fallback={<p class="text-xs text-muted-foreground">{t("properties.noMetadata")}</p>}>
                      {(m, i) => (
                        <div class="grid grid-cols-[1fr_1fr_auto] gap-2">
                          <Input value={m.key} placeholder={t("tags.key")} onInput={(e) => setMeta((p) => p.map((x, idx) => (idx === i() ? { ...x, key: e.currentTarget.value } : x)))} />
                          <Input value={m.value} placeholder={t("tags.value")} onInput={(e) => setMeta((p) => p.map((x, idx) => (idx === i() ? { ...x, value: e.currentTarget.value } : x)))} />
                          <Button variant="ghost" size="icon" onClick={() => setMeta((p) => p.filter((_, idx) => idx !== i()))}>
                            <Trash2 class="h-4 w-4 text-destructive" />
                          </Button>
                        </div>
                      )}
                    </For>
                    <Button variant="outline" size="sm" class="self-start" onClick={() => setMeta((p) => [...p, { key: "", value: "" } as Tag])}>
                      <Plus class="h-4 w-4" />{t("tags.add")}
                    </Button>
                  </div>
                </div>
                <p class="text-xs text-muted-foreground">{t("objectSettings.metadataHint")}</p>
                <div class="flex justify-end">
                  <Button size="sm" onClick={saveMetadata} disabled={saving()}>{saving() ? t("common.saving") : t("common.save")}</Button>
                </div>
              </div>
            </Show>

            {/* Permissions ------------------------------------------------ */}
            <Show when={tab() === "permissions"}>
              <div class="flex flex-col gap-3">
                <div class="flex items-center gap-2">
                  <Show when={acl()?.isPublic} fallback={<Badge variant="secondary"><Lock class="mr-1 h-3 w-3" />{t("objectSettings.private")}</Badge>}>
                    <Badge variant="destructive"><Globe class="mr-1 h-3 w-3" />{t("objectSettings.public")}</Badge>
                  </Show>
                  <Show when={acl()?.owner}>
                    <span class="text-xs text-muted-foreground">{t("objectSettings.owner")}: {acl()?.owner}</span>
                  </Show>
                </div>
                <Field label={t("objectSettings.cannedAcl")}>
                  <SimpleSelect value={cannedAcl()} options={CANNED_ACLS.map((a) => ({ value: a, label: a }))} onChange={setCannedAcl} />
                </Field>
                <div>
                  <Label>{t("objectSettings.grants")}</Label>
                  <div class="mt-1 flex flex-col gap-1">
                    <For each={acl()?.grants ?? []} fallback={<p class="text-xs text-muted-foreground">{t("common.none")}</p>}>
                      {(g) => (
                        <div class="flex justify-between rounded border px-2 py-1 text-xs">
                          <span class="font-mono">{g.grantee || "—"}</span>
                          <span class="text-muted-foreground">{g.permission}</span>
                        </div>
                      )}
                    </For>
                  </div>
                </div>
                <div class="flex justify-end">
                  <Button size="sm" onClick={applyAcl} disabled={saving()}>{saving() ? t("common.saving") : t("objectSettings.applyAcl")}</Button>
                </div>
              </div>
            </Show>

            {/* Archive / restore ------------------------------------------ */}
            <Show when={tab() === "archive"}>
              <div class="flex flex-col gap-3">
                <p class="text-sm text-muted-foreground">{t("objectSettings.restoreHint")}</p>
                <Field label={t("objectSettings.restoreDays")}>
                  <Input type="number" min={1} value={restoreDays()} onInput={(e) => setRestoreDays(parseInt(e.currentTarget.value) || 1)} />
                </Field>
                <Field label={t("objectSettings.restoreTier")}>
                  <SimpleSelect value={restoreTier()} options={RESTORE_TIERS.map((r) => ({ value: r, label: r }))} onChange={setRestoreTier} />
                </Field>
                <div class="flex justify-end">
                  <Button size="sm" onClick={doRestore} disabled={saving()}>{saving() ? t("common.saving") : t("objectSettings.restore")}</Button>
                </div>
              </div>
            </Show>
          </div>
        </Show>

        <DialogFooter>
          <Button variant="outline" onClick={() => props.onOpenChange(false)}>{t("common.close")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
