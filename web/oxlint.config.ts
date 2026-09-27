import { defineConfig } from "oxlint";

export default defineConfig({
  $schema: "./node_modules/oxlint/configuration_schema.json",
  plugins: ["eslint", "typescript", "unicorn", "oxc", "import", "promise"],
  ignorePatterns: ["dist/**"],
  options: {
    typeAware: true,
  },
  categories: {
    correctness: "error",
    suspicious: "error",
    perf: "warn",
    pedantic: "off",
    style: "off",
    restriction: "off",
    nursery: "off",
  },
  env: {
    browser: true,
    node: true,
    es2024: true,
  },
  rules: {
    "import/no-unassigned-import": "off",
    "typescript/no-deprecated": "error",
  },
});
