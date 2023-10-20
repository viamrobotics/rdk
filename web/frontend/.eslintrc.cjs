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

    // @TODO(mp) These were disabled when moving over to js-config. They should be gradually re-enabled.
    '@typescript-eslint/no-floating-promises': 'off',
    '@typescript-eslint/no-misused-promises': 'off',
    '@typescript-eslint/no-unsafe-enum-comparison': 'off',
    'svelte/valid-compile': 'off',
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
