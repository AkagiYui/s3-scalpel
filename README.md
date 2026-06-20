# S3 Scalpel · S3 手术刀

A surgical, cross-platform desktop client for S3-compatible object storage (MinIO,
Cloudflare R2, and any generic S3 endpoint). Built with [Wails 3](https://v3.wails.io/),
a Go backend, and a [SolidJS](https://www.solidjs.com/) + [solid-ui](https://www.solid-ui.com/)
(Kobalte + Tailwind) frontend.

Bundle identifier: `com.akagiyui.s3_scalpel`

## Features

- **Connections** — add any number of S3-compatible accounts (display name, endpoint,
  region, path-/virtual-hosted style, access/secret key). Test connections before saving.
  Configs are stored locally and shared across all windows.
- **Buckets & objects** — create/delete buckets; browse objects as a tree with
  breadcrumb navigation, path jump, sorting (name/size/date), search/filter, and a
  right-click context menu.
- **Multi-window & tabs** — open new windows from the menu (⌘N for a new connection,
  ⌘T new tab, ⌘W close tab); each window manages connections with custom in-app tabs
  (not native), and several connections/tabs can be open at once.
- **Operation queue** — uploads, downloads, deletes, copies and moves run through a
  per-window queue with concurrency limits (default 5), priorities, retry, and live
  progress (including large multipart transfers). Auto-consume can be toggled; with it
  off, operations wait for an explicit *Start*. The task panel sits at the bottom of the
  storage page and is collapsible. The queue is persisted to disk; tasks left running by
  a crash are recovered as *failed* for manual retry.
- **Transfers** — drag-and-drop upload, recursive folder upload/download, create folders,
  multipart upload with a configurable part size (default 8 MiB, min 5 MiB).
- **Object tools** — properties, presigned URLs (custom expiry), object tags, object
  versions (when the bucket supports versioning), and previews for images, text and PDF
  (downloaded to a temp dir) plus streamed audio/video via a presigned URL. Preview size
  is capped (configurable).
- **Settings** — appearance (language: 简体中文 / English, defaulting to the system locale;
  light/dark/system theme), notifications (system notification + sound toggles), transfer
  defaults, an About section, and import/export of all settings (optionally including
  credentials).
- **Persistence** — everything is stored in the platform-standard application data
  directory.

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [Node](https://nodejs.org/) 20.19+ / 22.12+ (Vite 8 requirement)
- [pnpm](https://pnpm.io/) (the frontend package manager)
- [Wails 3 CLI](https://v3.wails.io/) (`wails3`)

## Development

```sh
wails3 dev
```

Runs the app with hot-reload. The frontend dev server (Vite 8) runs on port 9245.

> Note: macOS system notifications require a real `.app` bundle, so they are disabled in
> the bare-binary `wails3 dev` workflow and enabled in packaged builds.

## Build

```sh
wails3 task build      # compile the binary into ./bin
wails3 task package    # produce a distributable bundle (.app on macOS)
```

## Project layout

```
main.go              app setup, services, menu, window/lifecycle
core.go              shared backend state (settings, connections, queue, clients)
*_service.go         Wails-bound services (Settings, Config, S3, Queue, Preview, App)
internal/model       data types shared with the generated TS bindings
internal/store       atomic JSON persistence
internal/s3x         AWS SDK v2 wrapper (client cache, operations, transfers)
internal/queue       per-window operation queue (scheduling, retry, persistence)
frontend/src         SolidJS app (pages, features/storage, components/ui, i18n, stores)
```

## Testing

Unit tests cover the queue (crash recovery, control ops, persistence) and path helpers:

```sh
go test ./...
```

An end-to-end integration test against a live endpoint is included but **skipped unless
credentials are supplied via the environment** (so no secrets live in the repo):

```sh
S3SCALPEL_TEST_ENDPOINT="https://your-endpoint" \
S3SCALPEL_TEST_ACCESS="..." \
S3SCALPEL_TEST_SECRET="..." \
S3SCALPEL_TEST_PATHSTYLE=1 \
go test ./internal/s3x -run TestIntegration -v
```

## License

MIT
