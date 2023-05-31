import vue from '@vitejs/plugin-vue';
import Hashes from 'jshashes';
import { defineConfig } from 'vite';
import path from 'node:path';
import url from 'node:url';

const MD5 = new Hashes.MD5();

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
    minify: 'terser',
    sourcemap: true,

    /**
     * This is currently set to infinity due to the lack of an asset pipeline
     * when RC is embedded in app.viam.com.
     */
    assetsInlineLimit: Number.POSITIVE_INFINITY,
    rollupOptions: {
      input: {
        control: './src/main.ts',
      },
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: (chunk) => {
          // eslint-disable-next-line unicorn/no-array-reduce
          const code = Object.keys(chunk.modules).reduce(
            (prev, key) => `${prev}${chunk.modules[key].code}`,
            ''
          );
          const hash = MD5.hex(code);

          return `assets/chunks.${hash}.js`;
        },
        assetFileNames: '[name].[ext]',
      },
      onwarn: (warning, warn) => {
        if (warning.code === 'EVAL') {
          return;
        }
        warn(warning);
      },
    },
    outDir: '../runtime-shared/static',
    emptyOutDir: true,
  },
  optimizeDeps: {
    exclude: ['@viamrobotics/rpc'],
  },
  server: {
    port: 5174,
  },
  resolve: {
    alias: {
      '@': path.resolve(path.dirname(url.fileURLToPath(import.meta.url)), './src'),
    },
  },
});
