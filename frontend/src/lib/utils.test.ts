import { describe, it, expect } from "vitest";
import { formatBytes, formatDate, keyBasename, cn } from "./utils";

describe("formatBytes", () => {
  it.each([
    [0, "0 B"],
    [-1, "0 B"],
    [512, "512 B"],
    [1024, "1 KB"],
    [1536, "1.5 KB"],
    [1024 ** 2, "1 MB"],
    [1024 ** 3, "1 GB"],
    [1024 ** 5, "1 PB"],
  ])("formats %d as %s", (input, expected) => {
    expect(formatBytes(input)).toBe(expected);
  });

  it("honours the decimals argument", () => {
    expect(formatBytes(1536, 0)).toBe("2 KB");
    expect(formatBytes(1590, 2)).toBe("1.55 KB");
  });

  it("clamps beyond the largest unit rather than producing undefined", () => {
    expect(formatBytes(1024 ** 7)).toMatch(/PB$/);
  });
});

describe("formatDate", () => {
  it("returns an empty string for a zero timestamp", () => {
    expect(formatDate(0)).toBe("");
  });

  it("renders a fixed instant for a known locale", () => {
    // 2024-03-05T08:09:00Z rendered in UTC-agnostic terms: just assert the
    // pieces are present rather than pinning the runner's time zone.
    const out = formatDate(Date.UTC(2024, 2, 5, 8, 9), "en-US");
    expect(out).toMatch(/\d{2}\/\d{2}\/\d{4}/);
  });
});

describe("keyBasename", () => {
  it.each([
    ["file.txt", "file.txt"],
    ["a/b/file.txt", "file.txt"],
    ["a/b/folder/", "folder"],
    ["folder/", "folder"],
    ["", ""],
  ])("reduces %j to %j", (input, expected) => {
    expect(keyBasename(input)).toBe(expected);
  });
});

describe("cn", () => {
  it("merges conflicting tailwind utilities, last one winning", () => {
    expect(cn("p-2", "p-4")).toBe("p-4");
  });

  it("drops falsy values", () => {
    const off: string | false = "b".length > 5 && "b";
    expect(cn("a", off, undefined, null, "c")).toBe("a c");
  });
});
