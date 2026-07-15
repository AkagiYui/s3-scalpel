import { createSignal, onMount, onCleanup, Show, For, type Component, type JSX } from "solid-js";
import { Download, Upload, ExternalLink, Mail, Bug, Palette, Bell, ArrowDownUp, Database } from "lucide-solid";
import { PageHeader } from "~/components/PageHeader";
import { Card, CardHeader, CardTitle, CardDescription, CardContent, Input, Label, Separator } from "~/components/ui/primitives";
import { Button } from "~/components/ui/button";
import { Switch } from "~/components/ui/switch";
import { SimpleSelect } from "~/components/ui/select";
import { toast } from "~/components/ui/toast";
import { settings, updateSettings } from "~/stores/settings";
import { AppService, SettingsService, windowID, type AppInfo } from "~/lib/api";
import { t } from "~/i18n";
import * as bus from "~/lib/bus";

const MB = 1024 * 1024;

const Section: Component<{ icon: JSX.Element; title: string; desc?: string; children: JSX.Element }> = (
  props
) => (
  <Card>
    <CardHeader>
      <CardTitle class="flex items-center gap-2">
        {props.icon}
        {props.title}
      </CardTitle>
      <Show when={props.desc}>
        <CardDescription>{props.desc}</CardDescription>
      </Show>
    </CardHeader>
    <CardContent class="flex flex-col gap-4">{props.children}</CardContent>
  </Card>
);

const Row: Component<{ label: string; hint?: string; children: JSX.Element }> = (props) => (
  <div class="flex items-center justify-between gap-4">
    <div class="min-w-0">
      <div class="text-sm font-medium">{props.label}</div>
      <Show when={props.hint}>
        <div class="text-xs text-muted-foreground">{props.hint}</div>
      </Show>
    </div>
    <div class="shrink-0">{props.children}</div>
  </div>
);

const Settings: Component = () => {
  const [info, setInfo] = createSignal<AppInfo | null>(null);
  const [includeSensitive, setIncludeSensitive] = createSignal(false);
  let aboutRef: HTMLDivElement | undefined;

  onMount(async () => {
    try {
      setInfo(await AppService.Info());
    } catch (e) {
      console.error(e);
    }
    const off = bus.on("scroll-about", () => aboutRef?.scrollIntoView({ behavior: "smooth" }));
    onCleanup(off);
  });

  const partMB = () => Math.round(settings().partSize / MB);
  const previewMB = () => Math.round(settings().previewMaxSize / MB);

  const exportSettings = async () => {
    try {
      const path = await SettingsService.Export(includeSensitive());
      if (path) toast.success(t("settings.exported", { path }));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const importSettings = async () => {
    try {
      const ok = await SettingsService.Import();
      if (ok) toast.success(t("settings.imported"));
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  const chooseDownloadDir = async () => {
    try {
      const dir = await AppService.PickDirectory(windowID(), t("settings.defaultDownloadDir"));
      if (dir) updateSettings({ defaultDownloadDir: dir });
    } catch (e: any) {
      toast.error(String(e?.message ?? e));
    }
  };

  return (
    <div class="flex h-full flex-col">
      <PageHeader title={t("settings.title")} />
      <div class="flex-1 overflow-y-auto">
        <div class="mx-auto flex max-w-3xl flex-col gap-6 p-6">
          {/* Appearance */}
          <Section
            icon={<Palette class="h-4 w-4" />}
            title={t("settings.appearance")}
            desc={t("settings.appearanceDesc")}
          >
            <Row label={t("settings.language")}>
              <SimpleSelect
                class="w-44"
                value={settings().language}
                onChange={(v) => updateSettings({ language: v })}
                options={[
                  { value: "system", label: t("settings.languageSystem") },
                  { value: "zh", label: "简体中文" },
                  { value: "en", label: "English" },
                ]}
              />
            </Row>
            <Separator />
            <Row label={t("settings.theme")}>
              <SimpleSelect
                class="w-44"
                value={settings().theme}
                onChange={(v) => updateSettings({ theme: v })}
                options={[
                  { value: "system", label: t("settings.themeSystem") },
                  { value: "light", label: t("settings.themeLight") },
                  { value: "dark", label: t("settings.themeDark") },
                ]}
              />
            </Row>
          </Section>

          {/* Notifications */}
          <Section
            icon={<Bell class="h-4 w-4" />}
            title={t("settings.notifications")}
            desc={t("settings.notificationsDesc")}
          >
            <Row label={t("settings.sendNotifications")}>
              <Switch
                checked={settings().notifyEnabled}
                onChange={(v) => updateSettings({ notifyEnabled: v })}
              />
            </Row>
            <Separator />
            <Row label={t("settings.notificationSound")}>
              <Switch
                checked={settings().notifySound}
                onChange={(v) => updateSettings({ notifySound: v })}
              />
            </Row>
          </Section>

          {/* Transfer & Queue */}
          <Section
            icon={<ArrowDownUp class="h-4 w-4" />}
            title={t("settings.transfer")}
            desc={t("settings.transferDesc")}
          >
            <Row label={t("settings.multipart")} hint={t("settings.multipartDesc")}>
              <Switch
                checked={settings().multipartEnabled}
                onChange={(v) => updateSettings({ multipartEnabled: v })}
              />
            </Row>
            <Separator />
            <Row label={t("settings.partSize")} hint={t("settings.partSizeHint")}>
              <Input
                type="number"
                min={5}
                class="w-24"
                value={partMB()}
                onChange={(e) => {
                  const mb = Math.max(5, Number(e.currentTarget.value) || 5);
                  updateSettings({ partSize: mb * MB });
                }}
              />
            </Row>
            <Separator />
            <Row label={t("settings.concurrency")}>
              <Input
                type="number"
                min={1}
                max={32}
                class="w-24"
                value={settings().concurrency}
                onChange={(e) =>
                  updateSettings({ concurrency: Math.max(1, Number(e.currentTarget.value) || 1) })
                }
              />
            </Row>
            <Separator />
            <Row label={t("settings.autoConsume")}>
              <Switch
                checked={settings().autoConsumeQueue}
                onChange={(v) => updateSettings({ autoConsumeQueue: v })}
              />
            </Row>
            <Separator />
            <Row label={t("settings.uploadStorageClass")} hint={t("settings.uploadStorageClassHint")}>
              <SimpleSelect
                class="w-52"
                value={settings().uploadStorageClass ?? ""}
                onChange={(v) => updateSettings({ uploadStorageClass: v })}
                options={[
                  { value: "", label: t("settings.providerDefault") },
                  ...["STANDARD", "STANDARD_IA", "ONEZONE_IA", "INTELLIGENT_TIERING", "GLACIER", "DEEP_ARCHIVE", "GLACIER_IR"].map((c) => ({ value: c, label: c })),
                ]}
              />
            </Row>
            <Separator />
            <Row label={t("settings.uploadEncryption")}>
              <SimpleSelect
                class="w-52"
                value={settings().uploadSSE ?? ""}
                onChange={(v) => updateSettings({ uploadSSE: v })}
                options={[
                  { value: "", label: t("settings.providerDefault") },
                  { value: "AES256", label: "AES256 (SSE-S3)" },
                  { value: "aws:kms", label: "aws:kms (SSE-KMS)" },
                ]}
              />
            </Row>
            <Show when={(settings().uploadSSE ?? "") === "aws:kms"}>
              <Separator />
              <Row label={t("settings.uploadKmsKey")}>
                <Input
                  class="w-64"
                  value={settings().uploadKMSKeyId ?? ""}
                  onInput={(e) => updateSettings({ uploadKMSKeyId: e.currentTarget.value })}
                />
              </Row>
            </Show>
            <Separator />
            <Row label={t("settings.previewMaxSize")}>
              <Input
                type="number"
                min={1}
                class="w-24"
                value={previewMB()}
                onChange={(e) =>
                  updateSettings({ previewMaxSize: Math.max(1, Number(e.currentTarget.value) || 1) * MB })
                }
              />
            </Row>
            <Separator />
            <Row label={t("settings.defaultDownloadDir")}>
              <div class="flex items-center gap-2">
                <span class="max-w-48 truncate text-xs text-muted-foreground">
                  {settings().defaultDownloadDir || t("settings.defaultDownloadDirNotSet")}
                </span>
                <Button size="sm" variant="outline" onClick={chooseDownloadDir}>
                  {t("settings.chooseFolder")}
                </Button>
                <Show when={settings().defaultDownloadDir}>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => updateSettings({ defaultDownloadDir: "" })}
                  >
                    {t("settings.clearFolder")}
                  </Button>
                </Show>
              </div>
            </Row>
          </Section>

          {/* Import / Export */}
          <Section
            icon={<Database class="h-4 w-4" />}
            title={t("settings.dataManagement")}
            desc={t("settings.dataManagementDesc")}
          >
            <Row label={t("settings.includeSensitive")} hint={t("settings.includeSensitiveHint")}>
              <Switch checked={includeSensitive()} onChange={setIncludeSensitive} />
            </Row>
            <Separator />
            <div class="flex gap-2">
              <Button variant="outline" onClick={exportSettings}>
                <Download class="h-4 w-4" />
                {t("settings.exportSettings")}
              </Button>
              <Button variant="outline" onClick={importSettings}>
                <Upload class="h-4 w-4" />
                {t("settings.importSettings")}
              </Button>
            </div>
          </Section>

          {/* About */}
          <div ref={aboutRef}>
            <Section icon={<Bug class="h-4 w-4" />} title={t("settings.about")}>
              <Row label={t("settings.version")}>
                <span class="font-mono text-sm">{info()?.version ?? "—"}</span>
              </Row>
              <Separator />
              <Row label={t("settings.buildVersion")}>
                <span class="font-mono text-sm">{info()?.buildVersion ?? "—"}</span>
              </Row>
              <Separator />
              <Row label={t("settings.homepage")}>
                <Button variant="link" class="h-auto p-0" onClick={() => AppService.OpenURL("https://aky.moe")}>
                  aky.moe
                  <ExternalLink class="h-3 w-3" />
                </Button>
              </Row>
              <Separator />
              <Row label={t("settings.contact")}>
                <Button
                  variant="link"
                  class="h-auto p-0"
                  onClick={() => AppService.OpenURL("mailto:akagiyui@yeah.net")}
                >
                  <Mail class="h-3 w-3" />
                  akagiyui@yeah.net
                </Button>
              </Row>

              <Show when={info()?.debug}>
                <Separator />
                <div class="rounded-md bg-muted/50 p-3">
                  <div class="mb-2 text-xs font-semibold text-muted-foreground">
                    {t("settings.debugInfo")}
                  </div>
                  <dl class="grid grid-cols-1 gap-1 text-xs">
                    <For
                      each={[
                        [t("settings.debugPlatform"), `${info()?.platform} / ${info()?.arch}`],
                        [t("settings.debugGoVersion"), info()?.goVersion],
                        [t("settings.debugWindows"), String(info()?.windowCount)],
                        [t("settings.debugDataDir"), info()?.dataDir],
                        [t("settings.debugCacheDir"), info()?.cacheDir],
                      ]}
                    >
                      {([k, v]) => (
                        <div class="flex gap-2">
                          <dt class="w-28 shrink-0 text-muted-foreground">{k}</dt>
                          <dd class="selectable break-all font-mono">{v}</dd>
                        </div>
                      )}
                    </For>
                  </dl>
                </div>
              </Show>
            </Section>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Settings;
