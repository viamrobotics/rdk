// This vite configuration builds RC in library mode for NPM distribution.
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import pkg from './package.json';
import path from 'node:path';
import url from 'node:url';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue({
      reactivityTransform: true,
      template: {
        compilerOptions: {
          // treat all tags with a dash as custom elements
          isCustomElement: (tag) => tag.includes('-'),
        },
      },
    }),
  ],
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
      // make sure to externalize deps that shouldn't be bundled into your library
      external: Object.keys(pkg.peerDependencies),
      output: {
        inlineDynamicImports: true,
        manualChunks: undefined,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(path.dirname(url.fileURLToPath(import.meta.url)), './src'),
    },
  },
});
