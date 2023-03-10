/* eslint-disable @typescript-eslint/no-var-requires */
import fs from 'node:fs';

const source = './node_modules/@viamrobotics/prime/dist';
const dest = '../runtime-shared/static';

fs.copyFileSync(`${source}/prime.css`, `${dest}/prime.css`);
fs.copyFileSync(`${source}/icons.woff2`, `${dest}/icons.woff2`);
