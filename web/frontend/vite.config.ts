import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      input: {
        control: './src/main.js',
      },
      output: {
        entryFileNames: '[name].js',
      },
    },
    outDir: '../runtime-shared/static',
  },
});
