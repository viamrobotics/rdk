'use strict';

const path = require('node:path');

module.exports = {
  root: true,
  extends: ['@viamrobotics/eslint-config/svelte'],
  parserOptions: {
    tsconfigRootDir: __dirname,
    project: './tsconfig.json',
  },
  settings: {
    tailwindcss: {
      config: path.join(__dirname, 'tailwind.config.ts'),
    },
  },
  env: {
    browser: true,
    node: true,
  },
  rules: {
    'no-void': ['error', { allowAsStatement: true }],
  },
  overrides: [
    {
      files: '**/*.svelte',
      rules: {
        'no-undef-init': 'off',
      },
    },
    {
      files: '**/*.d.ts',
      rules: {
        '@typescript-eslint/no-empty-interface': 'off',
      },
    },
  ],
  globals: {
    $$Generic: 'readonly',
  },
};
