import js from "@eslint/js";
import tseslint from "typescript-eslint";
import solid from "eslint-plugin-solid/configs/typescript";
import globals from "globals";

export default tseslint.config(
  {
    // Generated Wails bindings and build output are not ours to lint.
    ignores: ["dist/**", "bindings/**", "node_modules/**"],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    ...solid,
    languageOptions: {
      globals: globals.browser,
      parser: tseslint.parser,
      parserOptions: { project: "tsconfig.json" },
    },
    rules: {
      // The Wails event bridge hands back untyped payloads; those sites are
      // explicitly annotated `any` and validated at the boundary instead.
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "no-empty": ["error", { allowEmptyCatch: true }],
      // Solid populates `let el: HTMLElement | undefined` bindings through the
      // ref={} attribute, which the rule cannot see as an assignment.
      "no-unassigned-vars": "off",
    },
  },
  {
    files: ["**/*.cjs"],
    languageOptions: {
      globals: globals.node,
      sourceType: "commonjs",
    },
    rules: {
      "@typescript-eslint/no-require-imports": "off",
    },
  },
  {
    files: ["*.config.{js,ts}", "vite.config.ts"],
    languageOptions: { globals: { ...globals.node, ...globals.browser } },
  }
);
