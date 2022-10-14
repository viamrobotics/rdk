import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import Hashes from 'jshashes';

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
          const module = chunk.modules[Object.keys(chunk.modules).find((key) => key.includes(chunk.name))];
          const hash = MD5.hex(module.code);

          return `assets/${chunk.name}.${hash}.js`;
        },
        assetFileNames: '[name].[ext]',
      },
    },
    outDir: '../runtime-shared/static',
    emptyOutDir: true,
  },
  server: {
    port: 5174,
  },
});
