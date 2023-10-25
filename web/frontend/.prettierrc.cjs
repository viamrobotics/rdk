'use strict';

const path = require('node:path');
const baseConfig = require('@viamrobotics/prettier-config/svelte');

module.exports = {
  ...baseConfig,
  tailwindConfig: path.join(__dirname, 'tailwind.config.ts'),
};
