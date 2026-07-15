import { createSignal, createEffect, For, Show, type Component, type JSX } from "solid-js";
import { Plus, Trash2 } from "lucide-solid";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "~/components/ui/dialog";
import { Button } from "~/components/ui/button";
import { Input, Textarea, Label, Spinner } from "~/components/ui/primitives";
import { Switch } from "~/components/ui/switch";
import { SimpleSelect } from "~/components/ui/select";
import {
  BucketService,
  type CORSRule,
  type LifecycleRule,
  type BucketEncryption,
  type PublicAccessBlock,
  type Tag,
} from "~/lib/api";
import { toast } from "~/components/ui/toast";
import { t } from "~/i18n";
import { cn } from "~/lib/utils";

type TabKey = "general" | "policy" | "cors" | "lifecycle" | "encryption" | "access" | "tags";

const STORAGE_CLASSES = ["STANDARD", "STANDARD_IA", "ONEZONE_IA", "INTELLIGENT_TIERING", "GLACIER", "DEEP_ARCHIVE", "GLACIER_IR"];

export const BucketSettingsDialog: Component<{
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connId: string;
  bucket: string;
}> = (props) => {
  const [tab, setTab] = createSignal<TabKey>("general");
  const [loading, setLoading] = createSignal(false);
  const [saving, setSaving] = createSignal(false);

  const [region, setRegion] = createSignal("");
  const [versioning, setVersioning] = createSignal(false);
  const [policy, setPolicy] = createSignal("");
  const [cors, setCors] = createSignal<CORSRule[]>([]);
  const [lifecycle, setLifecycle] = createSignal<LifecycleRule[]>([]);
  const [enc, setEnc] = createSignal<BucketEncryption>({ enabled: false, sseAlgorithm: "AES256", kmsKeyId: "", bucketKeyEnabled: false } as BucketEncryption);
  const [pab, setPab] = createSignal<PublicAccessBlock>({ configured: false, blockPublicAcls: false, ignorePublicAcls: false, blockPublicPolicy: false, restrictPublicBuckets: false } as PublicAccessBlock);
  const [tags, setTags] = createSignal<Tag[]>([]);

  createEffect(() => {
    if (props.open) {
      setTab("general");
      loadAll();
    }
  });

  const loadAll = async () => {
    setLoading(true);
    try {
      const [reg, ver, pol, cr, lc, en, pb, tg] = await Promise.all([
        BucketService.Location(props.connId, props.bucket).catch(() => ""),
        BucketService.GetVersioning(props.connId, props.bucket).catch(() => null),
        BucketService.GetPolicy(props.connId, props.bucket).catch(() => ""),
        BucketService.GetCORS(props.connId, props.bucket).catch(() => []),
        BucketService.GetLifecycle(props.connId, props.bucket).catch(() => []),
        BucketService.GetEncryption(props.connId, props.bucket).catch(() => null),
        BucketService.GetPublicAccessBlock(props.connId, props.bucket).catch(() => null),
        BucketService.GetTags(props.connId, props.bucket).catch(() => []),
      ]);
      setRegion(reg ?? "");
      setVersioning((ver?.status ?? "") === "Enabled");
      setPolicy(pol ?? "");
      setCors((cr ?? []) as CORSRule[]);
      setLifecycle((lc ?? []) as LifecycleRule[]);
      if (en) setEnc(en as BucketEncryption);
      if (pb) setPab(pb as PublicAccessBlock);
      setTags((tg ?? []) as Tag[]);
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

  // ---- per-section save handlers -------------------------------------------
  const saveVersioning = (on: boolean) => {
    setVersioning(on);
    run(() => BucketService.SetVersioning(props.connId, props.bucket, on) as Promise<void>);
  };
  const savePolicy = () => run(() => BucketService.PutPolicy(props.connId, props.bucket, policy()) as Promise<void>);
  const saveCors = () =>
    run(() => BucketService.PutCORS(props.connId, props.bucket, cors() as any) as Promise<void>);
  const saveLifecycle = () =>
    run(() => BucketService.PutLifecycle(props.connId, props.bucket, lifecycle() as any) as Promise<void>);
  const saveEncryption = () => run(() => BucketService.PutEncryption(props.connId, props.bucket, enc() as any) as Promise<void>);
  const savePab = () => run(() => BucketService.PutPublicAccessBlock(props.connId, props.bucket, { ...pab(), configured: true } as any) as Promise<void>);
  const saveTags = () =>
    run(() => BucketService.PutTags(props.connId, props.bucket, tags().filter((x) => x.key.trim()) as any) as Promise<void>);

  const TabButton: Component<{ id: TabKey; label: string }> = (p) => (
    <button
      class={cn(
        "whitespace-nowrap rounded-md px-3 py-1.5 text-sm transition-colors",
        tab() === p.id ? "bg-primary text-primary-foreground" : "hover:bg-accent"
      )}
      onClick={() => setTab(p.id)}
    >
      {p.label}
    </button>
  );

  const Section: Component<{ children: JSX.Element; onSave?: () => void }> = (p) => (
    <div class="flex flex-col gap-3">
      {p.children}
      <Show when={p.onSave}>
        <div class="flex justify-end">
          <Button size="sm" onClick={p.onSave} disabled={saving()}>
            {saving() ? t("common.saving") : t("common.save")}
          </Button>
        </div>
      </Show>
    </div>
  );

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent size="xl">
        <DialogHeader>
          <DialogTitle>{t("bucketSettings.title", { name: props.bucket })}</DialogTitle>
        </DialogHeader>

        <div class="flex flex-wrap gap-1 border-b pb-2">
          <TabButton id="general" label={t("bucketSettings.general")} />
          <TabButton id="policy" label={t("bucketSettings.policy")} />
          <TabButton id="cors" label={t("bucketSettings.cors")} />
          <TabButton id="lifecycle" label={t("bucketSettings.lifecycle")} />
          <TabButton id="encryption" label={t("bucketSettings.encryption")} />
          <TabButton id="access" label={t("bucketSettings.publicAccess")} />
          <TabButton id="tags" label={t("bucketSettings.tags")} />
        </div>

        <Show when={!loading()} fallback={<div class="flex justify-center py-10"><Spinner class="h-6 w-6" /></div>}>
          <div class="min-h-[18rem] py-1 text-sm">
            {/* General ---------------------------------------------------- */}
            <Show when={tab() === "general"}>
              <Section>
                <div class="flex items-center justify-between rounded-md border p-3">
                  <span class="text-muted-foreground">{t("bucketSettings.region")}</span>
                  <span class="font-mono">{region() || "us-east-1"}</span>
                </div>
                <div class="flex items-center justify-between rounded-md border p-3">
                  <div>
                    <div class="font-medium">{t("bucketSettings.versioning")}</div>
                    <div class="text-xs text-muted-foreground">{t("bucketSettings.versioningHint")}</div>
                  </div>
                  <Switch checked={versioning()} onChange={saveVersioning} disabled={saving()} />
                </div>
              </Section>
            </Show>

            {/* Policy ----------------------------------------------------- */}
            <Show when={tab() === "policy"}>
              <Section onSave={savePolicy}>
                <Label>{t("bucketSettings.policyJson")}</Label>
                <Textarea
                  class="min-h-[16rem] font-mono text-xs"
                  value={policy()}
                  placeholder='{ "Version": "2012-10-17", "Statement": [] }'
                  onInput={(e) => setPolicy(e.currentTarget.value)}
                />
                <p class="text-xs text-muted-foreground">{t("bucketSettings.policyHint")}</p>
              </Section>
            </Show>

            {/* CORS ------------------------------------------------------- */}
            <Show when={tab() === "cors"}>
              <Section onSave={saveCors}>
                <For each={cors()} fallback={<p class="text-muted-foreground">{t("bucketSettings.corsEmpty")}</p>}>
                  {(rule, i) => (
                    <div class="flex flex-col gap-2 rounded-md border p-3">
                      <div class="flex items-center justify-between">
                        <span class="text-xs text-muted-foreground">#{i() + 1}</span>
                        <Button variant="ghost" size="icon-sm" onClick={() => setCors((p) => p.filter((_, idx) => idx !== i()))}>
                          <Trash2 class="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                      <CsvField label={t("bucketSettings.allowedOrigins")} value={rule.allowedOrigins}
                        onChange={(v) => setCors((p) => p.map((r, idx) => (idx === i() ? { ...r, allowedOrigins: v } : r)))} />
                      <CsvField label={t("bucketSettings.allowedMethods")} value={rule.allowedMethods}
                        onChange={(v) => setCors((p) => p.map((r, idx) => (idx === i() ? { ...r, allowedMethods: v } : r)))} />
                      <CsvField label={t("bucketSettings.allowedHeaders")} value={rule.allowedHeaders}
                        onChange={(v) => setCors((p) => p.map((r, idx) => (idx === i() ? { ...r, allowedHeaders: v } : r)))} />
                      <CsvField label={t("bucketSettings.exposeHeaders")} value={rule.exposeHeaders}
                        onChange={(v) => setCors((p) => p.map((r, idx) => (idx === i() ? { ...r, exposeHeaders: v } : r)))} />
                      <div class="grid grid-cols-[10rem_1fr] items-center gap-2">
                        <Label>{t("bucketSettings.maxAge")}</Label>
                        <Input type="number" value={rule.maxAgeSeconds}
                          onInput={(e) => setCors((p) => p.map((r, idx) => (idx === i() ? { ...r, maxAgeSeconds: parseInt(e.currentTarget.value) || 0 } : r)))} />
                      </div>
                    </div>
                  )}
                </For>
                <Button variant="outline" size="sm" class="self-start"
                  onClick={() => setCors((p) => [...p, { id: "", allowedOrigins: ["*"], allowedMethods: ["GET"], allowedHeaders: ["*"], exposeHeaders: [], maxAgeSeconds: 3000 } as CORSRule])}>
                  <Plus class="h-4 w-4" />{t("bucketSettings.addRule")}
                </Button>
              </Section>
            </Show>

            {/* Lifecycle -------------------------------------------------- */}
            <Show when={tab() === "lifecycle"}>
              <Section onSave={saveLifecycle}>
                <For each={lifecycle()} fallback={<p class="text-muted-foreground">{t("bucketSettings.lifecycleEmpty")}</p>}>
                  {(rule, i) => {
                    const upd = (patch: Partial<LifecycleRule>) =>
                      setLifecycle((p) => p.map((r, idx) => (idx === i() ? { ...r, ...patch } : r)));
                    return (
                      <div class="flex flex-col gap-2 rounded-md border p-3">
                        <div class="flex items-center justify-between gap-2">
                          <Input class="max-w-[14rem]" placeholder={t("bucketSettings.ruleId")} value={rule.id} onInput={(e) => upd({ id: e.currentTarget.value })} />
                          <div class="flex items-center gap-2">
                            <span class="text-xs text-muted-foreground">{rule.enabled ? t("common.enabled") : t("common.disabled")}</span>
                            <Switch checked={rule.enabled} onChange={(v) => upd({ enabled: v })} />
                            <Button variant="ghost" size="icon-sm" onClick={() => setLifecycle((p) => p.filter((_, idx) => idx !== i()))}>
                              <Trash2 class="h-4 w-4 text-destructive" />
                            </Button>
                          </div>
                        </div>
                        <NumField label={t("bucketSettings.prefix")} text value={rule.prefix} onText={(v) => upd({ prefix: v })} />
                        <NumField label={t("bucketSettings.expireDays")} value={rule.expirationDays} onNum={(v) => upd({ expirationDays: v })} />
                        <NumField label={t("bucketSettings.noncurrentDays")} value={rule.noncurrentVersionExpirationDays} onNum={(v) => upd({ noncurrentVersionExpirationDays: v })} />
                        <NumField label={t("bucketSettings.abortMultipartDays")} value={rule.abortIncompleteMultipartDays} onNum={(v) => upd({ abortIncompleteMultipartDays: v })} />
                        <div class="grid grid-cols-[10rem_1fr_1fr] items-center gap-2">
                          <Label>{t("bucketSettings.transition")}</Label>
                          <Input type="number" placeholder={t("bucketSettings.days")} value={rule.transitionDays} onInput={(e) => upd({ transitionDays: parseInt(e.currentTarget.value) || 0 })} />
                          <SimpleSelect value={rule.transitionStorageClass} placeholder={t("bucketSettings.storageClass")}
                            options={[{ value: "", label: t("common.none") }, ...STORAGE_CLASSES.map((c) => ({ value: c, label: c }))]}
                            onChange={(v) => upd({ transitionStorageClass: v })} />
                        </div>
                      </div>
                    );
                  }}
                </For>
                <Button variant="outline" size="sm" class="self-start"
                  onClick={() => setLifecycle((p) => [...p, { id: `rule-${p.length + 1}`, prefix: "", enabled: true, expirationDays: 0, noncurrentVersionExpirationDays: 0, abortIncompleteMultipartDays: 0, transitionDays: 0, transitionStorageClass: "" } as LifecycleRule])}>
                  <Plus class="h-4 w-4" />{t("bucketSettings.addRule")}
                </Button>
              </Section>
            </Show>

            {/* Encryption ------------------------------------------------- */}
            <Show when={tab() === "encryption"}>
              <Section onSave={saveEncryption}>
                <div class="flex items-center justify-between rounded-md border p-3">
                  <span>{t("bucketSettings.encEnabled")}</span>
                  <Switch checked={enc().enabled} onChange={(v) => setEnc((p) => ({ ...p, enabled: v } as BucketEncryption))} />
                </div>
                <Show when={enc().enabled}>
                  <div class="grid grid-cols-[10rem_1fr] items-center gap-2">
                    <Label>{t("bucketSettings.algorithm")}</Label>
                    <SimpleSelect value={enc().sseAlgorithm}
                      options={[{ value: "AES256", label: "AES256 (SSE-S3)" }, { value: "aws:kms", label: "aws:kms (SSE-KMS)" }]}
                      onChange={(v) => setEnc((p) => ({ ...p, sseAlgorithm: v } as BucketEncryption))} />
                  </div>
                  <Show when={enc().sseAlgorithm === "aws:kms"}>
                    <div class="grid grid-cols-[10rem_1fr] items-center gap-2">
                      <Label>{t("bucketSettings.kmsKey")}</Label>
                      <Input value={enc().kmsKeyId} onInput={(e) => setEnc((p) => ({ ...p, kmsKeyId: e.currentTarget.value } as BucketEncryption))} />
                    </div>
                  </Show>
                  <div class="flex items-center justify-between rounded-md border p-3">
                    <span>{t("bucketSettings.bucketKey")}</span>
                    <Switch checked={enc().bucketKeyEnabled} onChange={(v) => setEnc((p) => ({ ...p, bucketKeyEnabled: v } as BucketEncryption))} />
                  </div>
                </Show>
              </Section>
            </Show>

            {/* Public access block --------------------------------------- */}
            <Show when={tab() === "access"}>
              <Section onSave={savePab}>
                <For each={[
                  ["blockPublicAcls", t("bucketSettings.blockPublicAcls")],
                  ["ignorePublicAcls", t("bucketSettings.ignorePublicAcls")],
                  ["blockPublicPolicy", t("bucketSettings.blockPublicPolicy")],
                  ["restrictPublicBuckets", t("bucketSettings.restrictPublicBuckets")],
                ] as [keyof PublicAccessBlock, string][]}>
                  {([field, label]) => (
                    <div class="flex items-center justify-between rounded-md border p-3">
                      <span>{label}</span>
                      <Switch checked={!!pab()[field]} onChange={(v) => setPab((p) => ({ ...p, [field]: v } as PublicAccessBlock))} />
                    </div>
                  )}
                </For>
              </Section>
            </Show>

            {/* Bucket tags ------------------------------------------------ */}
            <Show when={tab() === "tags"}>
              <Section onSave={saveTags}>
                <For each={tags()} fallback={<p class="text-muted-foreground">{t("tags.empty")}</p>}>
                  {(tg, i) => (
                    <div class="grid grid-cols-[1fr_1fr_auto] gap-2">
                      <Input value={tg.key} placeholder={t("tags.key")} onInput={(e) => setTags((p) => p.map((x, idx) => (idx === i() ? { ...x, key: e.currentTarget.value } : x)))} />
                      <Input value={tg.value} placeholder={t("tags.value")} onInput={(e) => setTags((p) => p.map((x, idx) => (idx === i() ? { ...x, value: e.currentTarget.value } : x)))} />
                      <Button variant="ghost" size="icon" onClick={() => setTags((p) => p.filter((_, idx) => idx !== i()))}>
                        <Trash2 class="h-4 w-4 text-destructive" />
                      </Button>
                    </div>
                  )}
                </For>
                <Button variant="outline" size="sm" class="self-start" onClick={() => setTags((p) => [...p, { key: "", value: "" } as Tag])}>
                  <Plus class="h-4 w-4" />{t("tags.add")}
                </Button>
              </Section>
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

const CsvField: Component<{ label: string; value: string[]; onChange: (v: string[]) => void }> = (p) => (
  <div class="grid grid-cols-[10rem_1fr] items-center gap-2">
    <Label>{p.label}</Label>
    <Input
      value={(p.value ?? []).join(", ")}
      placeholder="*, GET, https://example.com"
      onInput={(e) => p.onChange(e.currentTarget.value.split(",").map((s) => s.trim()).filter(Boolean))}
    />
  </div>
);

const NumField: Component<{ label: string; value?: number | string; text?: boolean; onNum?: (v: number) => void; onText?: (v: string) => void }> = (p) => (
  <div class="grid grid-cols-[10rem_1fr] items-center gap-2">
    <Label>{p.label}</Label>
    <Input
      type={p.text ? "text" : "number"}
      value={p.value ?? (p.text ? "" : 0)}
      onInput={(e) => (p.text ? p.onText?.(e.currentTarget.value) : p.onNum?.(parseInt(e.currentTarget.value) || 0))}
    />
  </div>
);
