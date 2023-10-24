
import { svelte } from '@sveltejs/vite-plugin-svelte';
import Hashes from 'jshashes';
import { defineConfig } from 'vite';
import path from 'node:path';
import url from 'node:url';

// eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access
const MD5 = new Hashes.MD5();

export const plugins = [
  svelte(),
];

// https://vitejs.dev/config/
export default defineConfig({
  plugins,
  build: {
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
          // @ts-expect-error Fix this
          const { modules } = (chunk as { modules: Record<string, { code: string }> })
          // eslint-disable-next-line unicorn/no-array-reduce
          const code = Object.keys(modules).reduce(
            (prev, key) => `${prev}${modules[key]!.code}`,
            ''
          );
          // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-member-access, @typescript-eslint/no-unsafe-call
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
