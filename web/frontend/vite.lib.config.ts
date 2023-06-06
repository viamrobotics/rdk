// This vite configuration builds RC in library mode for NPM distribution.
import { defineConfig } from 'vite';
import pkg from './package.json';
import path from 'node:path';
import url from 'node:url';
import cssInjectedByJsPlugin from 'vite-plugin-css-injected-by-js';
import { plugins } from './vite.config';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [...plugins, cssInjectedByJsPlugin()],
  build: {
    minify: true,
    target: 'esnext',
    lib: {
      formats: ['es'],
      // Could also be a dictionary or array of multiple entry points
      entry: 'src/main-lib.ts',
      name: 'RC',
      // the proper extensions will be added
      fileName: 'rc',
    },
    rollupOptions: {
      // make sure to externalize deps that shouldn't be bundled
      external: Object.keys(pkg.peerDependencies),
      output: {
        inlineDynamicImports: true,
        manualChunks: undefined,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(
        path.dirname(url.fileURLToPath(import.meta.url)),
        './src'
      ),
    },
  },
});
