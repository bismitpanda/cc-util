import { defineConfig } from "oxfmt";

export default defineConfig({
  $schema: "./node_modules/oxfmt/configuration_schema.json",
  printWidth: 100,
  semi: true,
  singleQuote: false,
  jsxSingleQuote: false,
  trailingComma: "all",
  arrowParens: "always",
  bracketSpacing: true,
  quoteProps: "as-needed",
  endOfLine: "lf",
  tabWidth: 2,
  useTabs: false,
  objectWrap: "preserve",
  sortImports: {
    groups: [
      "side_effect",
      ["builtin", "external"],
      ["internal", "subpath"],
      ["parent", "sibling", "index"],
      "style",
      "type",
      "unknown",
    ],
    newlinesBetween: true,
  },
  sortTailwindcss: {
    stylesheet: "./src/app.css",
    functions: ["cn", "cva"],
  },
  sortPackageJson: true,
  ignorePatterns: ["dist/**"],
});
