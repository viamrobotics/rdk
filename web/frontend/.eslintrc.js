// eslint-disable-next-line unicorn/prefer-module
module.exports = {
  root: true,
  env: {
    es2021: true,
    node: true,
  },
  parser: 'vue-eslint-parser',
  parserOptions: {
    parser: '@typescript-eslint/parser',
    ecmaVersion: 'latest',
    sourceType: 'module',
    ecmaFeatures: { jsx: true },
  },
  plugins: [
    'vue',
    '@typescript-eslint',
    'unicorn',
    'import',
    'tailwindcss',
  ],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:import/recommended',
    'plugin:import/typescript',
    'plugin:unicorn/recommended',
    'plugin:tailwindcss/recommended',
    'plugin:vue/vue3-essential',
    'plugin:vue/vue3-strongly-recommended',
    'plugin:vue/vue3-recommended',
  ],
  rules: {
    // Spacing and code style
    indent: ['error', 2],
    'arrow-spacing': 'error',
    'block-spacing': 'error',
    'comma-spacing': 'error',
    'computed-property-spacing': 'error',
    'func-call-spacing': 'error',
    'key-spacing': 'error',
    'keyword-spacing': 'error',
    'rest-spread-spacing': 'error',
    'semi-spacing': 'error',
    'array-bracket-spacing': 'error',
    'space-before-blocks': 'error',
    'space-in-parens': 'error',
    'space-infix-ops': 'error',
    'space-unary-ops': 'error',
    'spaced-comment': 'error',
    'template-curly-spacing': 'error',
    'object-curly-spacing': ['error', 'always'],
    'no-multiple-empty-lines': ['error', { max: 1 }],
    'no-multi-spaces': 'error',
    'eol-last': 'error',
    'brace-style': 'error',
    'semi-style': 'error',
    'dot-notation': 'error',
    'nonblock-statement-body-position': 'error',
    quotes: ['error', 'single', { avoidEscape: true }],
    semi: ['error', 'always'],
    'comma-dangle': [
      'error',
      {
        arrays: 'always-multiline',
        objects: 'always-multiline',
        imports: 'always-multiline',
        exports: 'never',
        functions: 'never',
      },
    ],

    // Typescript catches these issues, and ESLint isn't smart enough to understand
    // Vue's macros like "defineProps()", so we'll turn these off for now
    'no-undef': 'off',
    'no-unused-vars': 'off',
    'prefer-object-spread': 'error',
    'prefer-template': 'error',
    'quote-props': ['error', 'as-needed'],
    'no-console': process.env.NODE_ENV === 'production' ? 'error' : 'off',
    'no-debugger': process.env.NODE_ENV === 'production' ? 'error' : 'off',
    'one-var': [
      'error',
      {
        let: 'never',
        const: 'never',
      },
    ],
    curly: ['error', 'all'],
    eqeqeq: ['error', 'always', { null: 'always' }],
    'no-unreachable-loop': 'error',
    'no-unsafe-optional-chaining': 'error',
    'require-atomic-updates': 'error',
    'array-callback-return': 'error',
    'no-caller': 'error',
    'no-param-reassign': 'error',
    'no-return-await': 'error',
    radix: 'error',
    'require-await': 'error',
    strict: 'error',
    yoda: 'error',
    'no-implicit-coercion': 'error',
    'no-unneeded-ternary': 'error',
    'no-useless-return': 'error',
    'no-var': 'error',
    'object-shorthand': ['error', 'properties'],
    'prefer-arrow-callback': 'error',
    'prefer-const': 'error',

    // Vue
    'vue/multi-word-component-names': 'off',
    'vue/no-deprecated-slot-attribute': 'off',
    'vue/require-default-prop': 'off',
    'vue/no-undef-components': ['error', { ignorePatterns: ['v-'] }],

    // Import
    'import/no-unresolved': 'error',
    'import/named': 'error',
    'import/default': 'error',
    'import/namespace': 'error',
    'import/no-absolute-path': 'error',
    'import/no-webpack-loader-syntax': 'error',
    'import/no-self-import': 'error',
    'import/no-cycle': 'error',
    'import/no-useless-path-segments': 'error',
    'import/order': 'error',
    'import/export': 'error',
    'import/first': 'error',
    'import/extensions': [
      'error',
      'ignorePackages',
      {
        js: 'never',
        ts: 'never',
      },
    ],

    // Unicorn
    'unicorn/no-empty-file': 'off',
    'unicorn/no-unsafe-regex': 'error',
    'unicorn/no-unused-properties': 'error',
    'unicorn/custom-error-definition': 'error',
    'unicorn/import-index': 'error',
    'unicorn/import-style': 'error',
    // @todo : re-enable prefer-at when support exists https://caniuse.com/mdn-javascript_builtins_array_at
    'unicorn/prefer-at': 'off',
    'unicorn/prefer-object-has-own': 'error',
    'unicorn/prefer-string-replace-all': 'error',
    'unicorn/string-content': 'error',
    'unicorn/prevent-abbreviations': 'off',
    'unicorn/filename-case': 'off',
    'unicorn/no-null': 'off',
    'unicorn/consistent-destructuring': 'off',

    // Tailwind
    'tailwindcss/no-custom-classname': 'off',

    // Typescript
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    '@typescript-eslint/no-explicit-any': 'warn',
    '@typescript-eslint/no-non-null-assertion': 'off',
    '@typescript-eslint/ban-ts-comment': 'warn',
  },
  settings: {
    'import/resolver': {
      node: {
        extensions: ['.js', '.ts', '.vue'],
      },
    },
  },
  ignorePatterns: ['**/cypress/**', '**/node_modules/**', '*.json', '**/runtime-shared/**'],
};
