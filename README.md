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
- **Buckets & objects** — create/delete buckets; browse objects as a virtualised list
  with breadcrumb navigation, path jump, sorting (name/size/date), search/filter, rename,
  full keyboard navigation (arrows, shift-range, ⌘A, Enter, Space) and a right-click
  context menu. Recursive search, prefix statistics and capability probing can be
  cancelled while they run.
- **Multi-window & tabs** — open new windows from the menu (⌘N for a new connection,
  ⌘T new tab, ⌘W close tab); each window manages connections with custom in-app tabs
  (not native), and several connections/tabs can be open at once. Window geometry and
  the tab strip are restored on the next launch.
- **Operation queue** — uploads, downloads, deletes, copies and moves run through a
  per-window queue with concurrency limits (default 5), priorities and live progress
  (including large multipart transfers). Transient failures (5xx, throttling, timeouts,
  dropped connections) retry automatically with an exponential backoff; permanent ones
  (403, 404, malformed request) fail straight away. Auto-consume can be toggled; with it
  off, operations wait for an explicit *Start*. The task panel sits at the bottom of the
  storage page and is collapsible. The queue is persisted to disk; tasks left running by
  a crash are recovered as *failed* for manual retry.
- **Conflict policy** — when the destination already exists, transfers can overwrite it,
  skip it, or keep both by writing to the next free `name (n).ext`. Applies to uploads,
  downloads, copies and moves alike.
- **Transfers** — drag-and-drop upload, recursive folder upload/download, create folders,
  multipart upload with a configurable part size (default 8 MiB, min 5 MiB) and part
  concurrency. A multipart upload interrupted by a transient failure **resumes** from
  the parts already stored rather than starting over.
- **Incomplete uploads** — multipart uploads that were never finished keep billable
  parts alive forever; the bucket toolbar lists them with their size and lets you abort
  them in bulk.
- **Bookmarks** — star a bucket or a folder and jump straight back to it from the
  toolbar; the list is shared across windows and survives restarts.
- **Object tools** — properties, presigned URLs (custom expiry), object tags (single or
  applied across a whole selection), copy key / `s3://` URI to the clipboard, object
  versions (when the bucket supports versioning), and previews. Images and PDFs stream
  from the app's own asset server rather than being inlined as base64; text is read with
  one ranged GET capped at the preview limit, so a multi-gigabyte log previews its first
  page instead of being refused; audio and video stream from a short-lived presigned URL.
- **Settings** — appearance (language: 简体中文 / English, defaulting to the system locale;
  light/dark/system theme), notifications (system notification + sound toggles), transfer
  defaults, an About section, and import/export of all settings (optionally including
  credentials).
- **Persistence** — everything is stored in the platform-standard application data
  directory.

## Security

- **Credentials at rest** — access keys, secrets and session tokens are stored
  AES-256-GCM encrypted. The encryption key lives in the OS credential store (macOS
  Keychain, Windows Credential Manager, Linux Secret Service); a host without one falls
  back to a 0600 file beside the data, and the About panel says which is in use. If the
  credential store is present but refuses to answer, the app reports an error rather
  than minting a fresh key that would orphan your saved connections.
- **Exports** — an export that includes credentials is sealed with AES-256-GCM under an
  Argon2id key derived from a passphrase you choose; keys never land in readable JSON.
  Exports without credentials stay plain JSON.
- **Transport warnings** — a connection with certificate verification disabled, or one
  pointing at a plain `http://` endpoint, is flagged on its card in the connection list,
  not just inside the edit form.
- **Content-Security-Policy** — the webview makes no outbound requests of its own (all
  S3 traffic goes through the Go backend), so the policy is `default-src 'self'` with a
  narrow set of exceptions; only `media-src` stays open, because audio and video
  previews stream directly from a presigned URL.

## Prerequisites

- [Go](https://go.dev/) 1.25+
- [Node](https://nodejs.org/) 22.12+ (Vite 8 requirement)
- [pnpm](https://pnpm.io/) (the frontend package manager)
- [Wails 3 CLI](https://v3.wails.io/) (`wails3`, v3.0.0-beta.10)

## Development

```sh
wails3 dev
```

Runs the app with hot-reload. The frontend dev server (Vite 8) runs on port 9245.

`frontend/bindings` holds the generated TypeScript bridge to the Go services. It is
**not** committed; `wails3 dev` and `wails3 task build` regenerate it automatically, and
a bare frontend checkout needs one explicit pass before type-checking:

```sh
wails3 generate bindings -clean=true
```

> Note: macOS system notifications require a real `.app` bundle, so they are disabled in
> the bare-binary `wails3 dev` workflow and enabled in packaged builds.

## Build

```sh
wails3 task build      # compile the binary into ./bin
wails3 task package    # produce a distributable bundle (.app on macOS)
```

The application version lives in the root `VERSION` file and is embedded at compile
time; `build/config.yml` must carry the same value (a unit test enforces it). `COMMIT`
identifies the individual build and is rewritten by CI with the short SHA (or the
release tag); it stays `dev` locally.

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
VERSION / COMMIT     embedded build identity (see Build)
```

## Code quality

```sh
gofmt -l .                        # formatting
go vet ./...                      # vet
golangci-lint run ./...           # lint (config in .golangci.yml)

cd frontend
pnpm run format:check             # prettier
pnpm run lint                     # eslint
pnpm run typecheck                # tsc --noEmit
```

CI runs all of the above on every push and pull request.

## Testing

The Go suite covers the queue (crash recovery, cancellation classification, control
ops, persistence), the encrypted store, the S3 client cache and — through an in-memory
fake S3 server — listing/pagination, recursive delete, single and multipart upload,
ranged parallel download and cross-endpoint streaming copy:

```sh
go test -race ./...
```

The frontend suite (Vitest) covers the object list helpers, formatting utilities, the
tab store and dictionary parity between the two languages:

```sh
cd frontend && pnpm run test:run
```

An additional end-to-end test against a live endpoint is included but **skipped unless
credentials are supplied via the environment** (so no secrets live in the repo):

```sh
S3SCALPEL_TEST_ENDPOINT="https://your-endpoint" \
S3SCALPEL_TEST_ACCESS="..." \
S3SCALPEL_TEST_SECRET="..." \
S3SCALPEL_TEST_PATHSTYLE=1 \
go test ./internal/s3x -run TestIntegration -v
```

## License

[MIT](LICENSE)
