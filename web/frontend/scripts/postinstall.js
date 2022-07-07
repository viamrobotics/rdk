/* eslint-disable @typescript-eslint/no-var-requires */
/* eslint-disable unicorn/prefer-module */
const fs = require('node:fs');

fs.copyFileSync('./node_modules/@viamrobotics/prime/dist/prime.css', '../runtime-shared/static/prime.css');
