import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    vue({
      reactivityTransform: true,
      template: {
        compilerOptions: {
          // treat all tags with a dash as custom elements
          isCustomElement: (tag) => tag.startsWith('v-'),
        },
      },
    }),
  ],
  build: {
    minify: 'terser',
    rollupOptions: {
      input: {
        control: './src/main.js',
        api: './src/api.ts',
      },
      output: {
        entryFileNames: '[name].js',
        assetFileNames: '[name].[ext]',
      },
    },
    outDir: '../runtime-shared/static',
  },
  server: {
    port: 5174,
  },
});
