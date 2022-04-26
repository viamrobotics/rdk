module.exports = {
  root: true,
  env: {
    es2020: true,
    node: true,
  },
  plugins: [
    "vue",
    "@typescript-eslint",
  ],
  extends: [
    "eslint:recommended",
    "plugin:vue/essential",
    "@vue/typescript/recommended",
  ],
  parserOptions: {
    ecmaVersion: 2020,
  },
  rules: {
    // Typescript catches these issues, and ESLint isn't smart enough to understand
    // Vue's macros like 'defineProps()', so we'll turn these off for now
    "no-undef": "off",
    "no-unused-vars": "off",
    "prefer-template": "error",
    "quote-props": ["error", "as-needed"],
    "no-console": process.env.NODE_ENV === "production" ? "warn" : "off",
    "no-debugger": process.env.NODE_ENV === "production" ? "warn" : "off",
    "eol-last": "error",
    "one-var": [
      "error",
      {
        let: "never",
        const: "never",
      },
    ],
    quotes: ["error", "double", { avoidEscape: true }],
    semi: ["error", "always"],
    "comma-dangle": [
      "error",
      {
        arrays: "always-multiline",
        objects: "always-multiline",
        imports: "always-multiline",
        exports: "never",
        functions: "never",
      },
    ],
    eqeqeq: ["error", "always", { null: "always" }],
    "no-unreachable-loop": "error",
    "no-unsafe-optional-chaining": "error",
    "require-atomic-updates": "error",
    "array-callback-return": "error",
    "no-caller": "error",
    "no-multi-spaces": "error",
    "no-param-reassign": "error",
    "no-return-await": "error",
    radix: "error",
    "require-await": "error",
    strict: "error",
    yoda: "error",
    "no-var": "error",
    "object-shorthand": ["error", "properties"],
    "prefer-arrow-callback": "error",
    "prefer-const": "error",

    // Vue
    "vue/script-setup-uses-vars": "error",
  
    /**
     * @TODO this rule cannot be currently enabled, because static analysis cannot be run on vue class components
     * We want to switch to exclusively using `<script setup>` components (there are many other static analysis benefits
     * of using only that component method), so once all components are converted we can re-enable this
     */ 
    "vue/no-undef-components": ["off", {
      ignorePatterns: [],
    }],

    /**
     * @TODO this should be turned on, but we have to rename a lot of components first
     */
    "vue/multi-word-component-names": "off",

    // Typescript
    "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
    "@typescript-eslint/no-explicit-any": "warn",
    "@typescript-eslint/no-non-null-assertion": "off",
    "@typescript-eslint/ban-ts-comment": "warn",
  },
  overrides: [
    {
      files: [
        "**/__tests__/*.{j,t}s?(x)",
        "**/tests/unit/**/*.spec.{j,t}s?(x)",
      ],
      env: {
        jest: true,
      },
    },
  ],
};
