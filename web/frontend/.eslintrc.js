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
  },
  plugins: [
    'vue',
    '@typescript-eslint',
    'unicorn',
    'tailwindcss',
    'promise',
  ],
  extends: [
    'plugin:@typescript-eslint/recommended',
    'eslint:all',
    'plugin:unicorn/recommended',
    'plugin:tailwindcss/recommended',
    'plugin:vue/vue3-essential',
    'plugin:vue/vue3-strongly-recommended',
    'plugin:vue/vue3-recommended',
    'plugin:promise/recommended',
  ],
  ignorePatterns: ['**/node_modules/**', '*.json', '**/protos/**'],
  rules: {
    // https://github.com/eslint/eslint/issues/13956
    indent: 'off',
    'array-element-newline': ['error', 'consistent'],
    'arrow-body-style': 'off',
    camelcase: ['error', { properties: 'never' }],
    'capitalized-comments': 'off',
    'consistent-return': 'off',
    'default-case': 'off',
    'dot-location': ['error', 'property'],
    'function-call-argument-newline': ['error', 'consistent'],
    'function-paren-newline': ['error', 'consistent'],
    'id-length': ['error', { exceptions: ['_', 'x', 'y', 'z', 'w', 'i', 'j', 'k'] }],
    'init-declarations': 'off',
    'implicit-arrow-linebreak': 'off',
    'lines-around-commen': 'off',
    'max-len': ['error', { code: 120 }],
    'max-lines': 'off',
    'max-lines-per-function': 'off',
    'max-params': 'off',
    'max-statements': 'off',
    'multiline-ternary': ['error', 'always-multiline'],
    'prefer-destructuring': [
      'error', {
        AssignmentExpression: { array: false, object: false },
        VariableDeclarator: { array: true, object: true },
      },
    ],
    'sort-keys': 'off',
    'sort-imports': 'off',
    'object-curly-spacing': ['error', 'always'],
    'object-property-newline': ['error', { allowAllPropertiesOnSameLine: true }],
    'no-continue': 'off',
    "no-duplicate-imports": "off",
    'no-extra-parens': 'off',
    'no-magic-numbers': 'off',
    'no-multiple-empty-lines': ['error', { max: 1 }],
    'no-ternary': 'off',
    'no-undefined': 'off',
    // Eventually we want to re-enable, so that people comment jira tickets instead of TODO.
    'no-warning-comments': 'off',
    'padded-blocks': 'off',
    'vue/no-deprecated-slot-attribute': 'off',
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

    /**
     * Typescript catches these issues, and ESLint isn't smart enough to understand
     * Vue's macros like 'defineProps()', so we'll turn these off for now
     */
    'no-undef': 'off',
    'no-unused-vars': 'off',
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
    'no-var': 'error',
    'object-shorthand': ['error', 'properties'],
    'prefer-arrow-callback': 'error',
    'prefer-const': 'error',

    // Vue
    'vue/multi-word-component-names': 'off',
    'vue/no-undef-components': ['error', { ignorePatterns: ['-'] }],
    'vue/require-default-prop': 'off',

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
    '@typescript-eslint/indent': ['error', 2],
    "@typescript-eslint/no-duplicate-imports": ["error"],
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    '@typescript-eslint/no-explicit-any': 'error',
    '@typescript-eslint/no-non-null-assertion': 'off',
    '@typescript-eslint/ban-ts-comment': 'warn',

    // Promise
    'promise/prefer-await-to-then': 'error',
  },
  settings: {
    'import/extensions': ['.vue'],
    'import/parsers': {
      '@typescript-eslint/parser': ['.ts', '.js'],
    },
    'import/resolver': {
      typescript: {
        alwaysTryTypes: true,
        project: './tsconfig.json',
      },
      'eslint-import-resolver-custom-alias': {
        alias: {
          '@': './frontend/src',
        },
        extensions: ['.ts', '.js', '.vue'],
      },
    },
  },
  // User ts parser for typescript files
  overrides: [
    {
      files: ['*.ts'],
      parser: '@typescript-eslint/parser',
    },
  ],
};
