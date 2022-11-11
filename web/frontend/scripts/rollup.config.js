/**
 * This file exists because protobuf cannot generate es modules, only commonjs...
 * but the rest of the world has moved on to es modules.
 * If in the future the first statement here becomes false, then please instead generate
 * es modules and get rid of this.
 */

const commonjs = require('@rollup/plugin-commonjs');
const { nodeResolve } = require('@rollup/plugin-node-resolve');
const copy = require('rollup-plugin-copy');
const alias = require('@rollup/plugin-alias');

const format = 'es';

const plugins = [
  nodeResolve(),
  commonjs({ sourceMap: false }),
  alias({
    entries: [
      {
        find: '@improbable-eng/grpc-web',
        replacement: './node_modules/@improbable-eng/grpc-web/dist/grpc-web-client.js',
      },
    ],
  }),
];

if (process.argv.length < 4) {
  throw "expected file to rollup"
}
const file = process.argv[4];

module.exports = {
  output: {
    file: `${file}.esm.js`,
    sourcemap: false,
    format,
  },
  plugins: [
    ...plugins,
    copy({
      targets: [
        {
          src: `${file}.d.ts`,
          dest: './',
          rename: () => `${file}.esm.d.ts`,
        },
      ],
    }),
  ],
  onwarn: (warning, warn) => {
    if (warning.code === 'EVAL') {
      return;
    }
    warn(warning);
  }
};
