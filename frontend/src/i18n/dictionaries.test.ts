import { describe, it, expect } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { en } from "./en";
import { zh } from "./zh";

type Dict = Record<string, unknown>;

/** Flatten a nested dictionary into dot-notation keys. */
function flatten(obj: Dict, prefix = ""): string[] {
  return Object.entries(obj).flatMap(([k, v]) => {
    const path = prefix ? `${prefix}.${k}` : k;
    return v && typeof v === "object" ? flatten(v as Dict, path) : [path];
  });
}

/** Every {placeholder} referenced by a dictionary value. */
function placeholders(obj: Dict, prefix = ""): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>();
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === "object") {
      for (const [kk, vv] of placeholders(v as Dict, path)) out.set(kk, vv);
    } else if (typeof v === "string") {
      const found = new Set([...v.matchAll(/\{\{?\s*(\w+)\s*\}?\}/g)].map((m) => m[1]));
      if (found.size) out.set(path, found);
    }
  }
  return out;
}

const enKeys = flatten(en as Dict);
const zhKeys = flatten(zh as Dict);

describe("translation dictionaries", () => {
  it("define exactly the same keys", () => {
    expect(zhKeys.filter((k) => !enKeys.includes(k))).toEqual([]);
    expect(enKeys.filter((k) => !zhKeys.includes(k))).toEqual([]);
  });

  it("never leave a value blank", () => {
    for (const [name, dict] of [
      ["en", en],
      ["zh", zh],
    ] as const) {
      const blank = flatten(dict as Dict).filter((key) => {
        const value = key.split(".").reduce<any>((acc, part) => acc?.[part], dict);
        return typeof value !== "string" || value.trim() === "";
      });
      expect(blank, `${name} has blank values`).toEqual([]);
    }
  });

  it("use the same interpolation placeholders in both languages", () => {
    const enPh = placeholders(en as Dict);
    const zhPh = placeholders(zh as Dict);
    for (const [key, expected] of enPh) {
      expect([...(zhPh.get(key) ?? [])].sort(), `placeholders differ for ${key}`).toEqual(
        [...expected].sort()
      );
    }
    for (const key of zhPh.keys()) {
      expect(enPh.has(key), `zh interpolates ${key} but en does not`).toBe(true);
    }
  });
});

/** Recursively collect the app's own source files. */
function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const full = join(dir, e.name);
    if (e.isDirectory()) return sourceFiles(full);
    return /\.tsx?$/.test(e.name) && !e.name.endsWith(".test.ts") ? [full] : [];
  });
}

describe("translation usage", () => {
  const used = new Set<string>();
  for (const file of sourceFiles(join(import.meta.dirname, ".."))) {
    const src = readFileSync(file, "utf8");
    // Only literal keys; template keys like t(`queue.status.${s}`) are covered
    // by the prefix check below.
    for (const m of src.matchAll(/\bt\(\s*["'`]([^"'`$]+)["'`]/g)) used.add(m[1]);
  }

  it("has a definition for every literal t() key", () => {
    expect([...used].filter((k) => !enKeys.includes(k)).sort()).toEqual([]);
  });

  it("keeps the dynamically indexed groups populated", () => {
    // t(`queue.status.${status}`) and t(`capabilities.ops.${op}`) are resolved at
    // runtime, so assert their groups exist rather than each individual key.
    for (const prefix of ["queue.status.", "queue.type.", "capabilities.ops."]) {
      expect(
        enKeys.some((k) => k.startsWith(prefix)),
        `${prefix} is empty`
      ).toBe(true);
    }
  });
});
