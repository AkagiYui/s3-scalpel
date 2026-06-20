import { createMemo } from "solid-js";
import * as i18n from "@solid-primitives/i18n";
import { en } from "./en";
import { zh } from "./zh";
import { effectiveLocale } from "~/stores/settings";

const dictionaries = { en, zh };

// A flat dictionary is recomputed whenever the effective locale changes; every
// t() call reads it, so translations update reactively across the app.
const flatDict = createMemo(() => i18n.flatten(dictionaries[effectiveLocale()]));

const translate = i18n.translator(flatDict, i18n.resolveTemplate);

/** Translate a dot-notation key, with optional template params. */
export function t(key: string, params?: Record<string, unknown>): string {
  return (translate(key as any, params as any) as string) ?? key;
}

/** The current resolved locale ("en" | "zh"). */
export const locale = effectiveLocale;
