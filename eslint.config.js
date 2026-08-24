import eslint from "@eslint/js";
import eslintConfigPrettier from "eslint-config-prettier";
import vue from "eslint-plugin-vue";
import vueParser from "vue-eslint-parser";
import globals from "globals";
import tseslint from "typescript-eslint";

const sourceFiles = ["modules/console/web/src/**/*.{ts,vue}", "packages/**/*.{ts,vue}"];

export default [
  {
    ignores: [
      "**/dist/**",
      "**/node_modules/**",
      "**/.mimosa/**",
      "**/*.d.ts",
      "modules/stellar/web/assets/vendor/**",
      "resource/**",
    ],
  },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  ...vue.configs["flat/recommended"],
  {
    files: ["modules/console/web/src/**/*.vue", "packages/**/*.vue"],
    languageOptions: {
      parser: vueParser,
      globals: {
        ...globals.browser,
        ...globals.es2021,
      },
      parserOptions: {
        parser: tseslint.parser,
        extraFileExtensions: [".vue"],
        ecmaVersion: "latest",
        sourceType: "module",
      },
    },
  },
  {
    files: sourceFiles,
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unused-vars": "off",
      "@typescript-eslint/no-unused-expressions": "off",
      "no-empty": "off",
      "no-useless-escape": "off",
      "no-unused-vars": "off",
      "vue/attribute-hyphenation": "off",
      "vue/attributes-order": "off",
      "vue/first-attribute-linebreak": "off",
      "vue/multi-word-component-names": "off",
      "vue/no-mutating-props": "off",
      "vue/no-v-html": "off",
      "vue/no-unused-vars": "off",
      "vue/require-default-prop": "off",
    },
  },
  eslintConfigPrettier,
];
