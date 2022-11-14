import vue from '@vitejs/plugin-vue';
import Hashes from 'jshashes';
import { defineConfig } from 'vite';

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
    rollupOptions: {
      input: {
        control: './src/main.ts',
        api: './src/api.ts',
      },
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: (chunk) => {
          // eslint-disable-next-line unicorn/no-array-reduce
          const code = Object.keys(chunk.modules).reduce((prev, key) => `${prev}${chunk.modules[key].code}`, '');
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
    exclude: ['@viamrobotics/rpc']
  },
  server: {
    port: 5174,
  },
});
