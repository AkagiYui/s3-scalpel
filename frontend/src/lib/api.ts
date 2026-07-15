// Central access point to the generated Wails bindings and runtime events.
import { Events } from "@wailsio/runtime";

export {
  AppService,
  ConfigService,
  S3Service,
  BucketService,
  QueueService,
  SettingsService,
  PreviewService,
} from "../../bindings/s3scalpel";

export { AppInfo } from "../../bindings/s3scalpel/models";

import {
  Connection,
  AppSettings,
  BucketInfo,
  ObjectEntry,
  ListResult,
  ObjectProperties,
  Tag,
  ObjectVersion,
  Task,
  PreviewData,
  TestResult,
  Capability,
  BucketVersioning,
  CORSRule,
  LifecycleRule,
  BucketEncryption,
  PublicAccessBlock,
  ObjectACL,
  ACLGrant,
  ObjectMetaUpdate,
  PrefixStats,
  StorageClassStat,
  SearchResult,
} from "../../bindings/s3scalpel/internal/model/models";

export {
  Connection,
  AppSettings,
  BucketInfo,
  ObjectEntry,
  ListResult,
  ObjectProperties,
  Tag,
  ObjectVersion,
  Task,
  PreviewData,
  TestResult,
  Capability,
  BucketVersioning,
  CORSRule,
  LifecycleRule,
  BucketEncryption,
  PublicAccessBlock,
  ObjectACL,
  ACLGrant,
  ObjectMetaUpdate,
  PrefixStats,
  StorageClassStat,
  SearchResult,
};

/** The current window id, supplied by the backend via the URL query string. */
export function windowID(): string {
  const p = new URLSearchParams(location.search);
  return p.get("wid") || "win-default";
}

/**
 * Subscribe to a backend event. The callback receives the event's data payload.
 * Returns an unsubscribe function.
 */
export function onEvent<T = any>(name: string, cb: (data: T) => void): () => void {
  const off = Events.On(name, (e: any) => cb(e?.data as T));
  return typeof off === "function" ? off : () => {};
}
