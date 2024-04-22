// vite.config.ts
import { svelte } from "file:///host/web/frontend/node_modules/@sveltejs/vite-plugin-svelte/src/index.js";
import Hashes from "file:///host/web/frontend/node_modules/jshashes/hashes.js";
import { defineConfig } from "file:///host/web/frontend/node_modules/vite/dist/node/index.js";
import path from "node:path";
import url from "node:url";
var __vite_injected_original_import_meta_url = "file:///host/web/frontend/vite.config.ts";
var MD5 = new Hashes.MD5();
var plugins = [svelte()];
var vite_config_default = defineConfig({
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
        control: "./src/main.ts"
      },
      output: {
        entryFileNames: "[name].js",
        chunkFileNames: (chunk) => {
          const { modules } = chunk;
          const code = Object.keys(modules).reduce(
            (prev, key) => `${prev}${modules[key].code}`,
            ""
          );
          const hash = MD5.hex(code);
          return `assets/chunks.${hash}.js`;
        },
        assetFileNames: "[name].[ext]"
      },
      onwarn: (warning, warn) => {
        if (warning.code === "EVAL") {
          return;
        }
        warn(warning);
      }
    },
    outDir: "../runtime-shared/static",
    emptyOutDir: true
  },
  optimizeDeps: {
    exclude: ["@viamrobotics/rpc"]
  },
  server: {
    port: 5174
  },
  resolve: {
    alias: {
      "@": path.resolve(
        path.dirname(url.fileURLToPath(__vite_injected_original_import_meta_url)),
        "./src"
      )
    }
  }
});
export {
  vite_config_default as default,
  plugins
};
//# sourceMappingURL=data:application/json;base64,ewogICJ2ZXJzaW9uIjogMywKICAic291cmNlcyI6IFsidml0ZS5jb25maWcudHMiXSwKICAic291cmNlc0NvbnRlbnQiOiBbImNvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9kaXJuYW1lID0gXCIvaG9zdC93ZWIvZnJvbnRlbmRcIjtjb25zdCBfX3ZpdGVfaW5qZWN0ZWRfb3JpZ2luYWxfZmlsZW5hbWUgPSBcIi9ob3N0L3dlYi9mcm9udGVuZC92aXRlLmNvbmZpZy50c1wiO2NvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9pbXBvcnRfbWV0YV91cmwgPSBcImZpbGU6Ly8vaG9zdC93ZWIvZnJvbnRlbmQvdml0ZS5jb25maWcudHNcIjtpbXBvcnQgeyBzdmVsdGUgfSBmcm9tICdAc3ZlbHRlanMvdml0ZS1wbHVnaW4tc3ZlbHRlJztcbmltcG9ydCBIYXNoZXMgZnJvbSAnanNoYXNoZXMnO1xuaW1wb3J0IHsgZGVmaW5lQ29uZmlnIH0gZnJvbSAndml0ZSc7XG5pbXBvcnQgcGF0aCBmcm9tICdub2RlOnBhdGgnO1xuaW1wb3J0IHVybCBmcm9tICdub2RlOnVybCc7XG5cbi8vIGVzbGludC1kaXNhYmxlLW5leHQtbGluZSBAdHlwZXNjcmlwdC1lc2xpbnQvbm8tdW5zYWZlLWFzc2lnbm1lbnQsIEB0eXBlc2NyaXB0LWVzbGludC9uby11bnNhZmUtY2FsbCwgQHR5cGVzY3JpcHQtZXNsaW50L25vLXVuc2FmZS1tZW1iZXItYWNjZXNzXG5jb25zdCBNRDUgPSBuZXcgSGFzaGVzLk1ENSgpO1xuXG5leHBvcnQgY29uc3QgcGx1Z2lucyA9IFtzdmVsdGUoKV07XG5cbi8vIGh0dHBzOi8vdml0ZWpzLmRldi9jb25maWcvXG5leHBvcnQgZGVmYXVsdCBkZWZpbmVDb25maWcoe1xuICBwbHVnaW5zLFxuICBidWlsZDoge1xuICAgIHNvdXJjZW1hcDogdHJ1ZSxcblxuICAgIC8qKlxuICAgICAqIFRoaXMgaXMgY3VycmVudGx5IHNldCB0byBpbmZpbml0eSBkdWUgdG8gdGhlIGxhY2sgb2YgYW4gYXNzZXQgcGlwZWxpbmVcbiAgICAgKiB3aGVuIFJDIGlzIGVtYmVkZGVkIGluIGFwcC52aWFtLmNvbS5cbiAgICAgKi9cbiAgICBhc3NldHNJbmxpbmVMaW1pdDogTnVtYmVyLlBPU0lUSVZFX0lORklOSVRZLFxuICAgIHJvbGx1cE9wdGlvbnM6IHtcbiAgICAgIGlucHV0OiB7XG4gICAgICAgIGNvbnRyb2w6ICcuL3NyYy9tYWluLnRzJyxcbiAgICAgIH0sXG4gICAgICBvdXRwdXQ6IHtcbiAgICAgICAgZW50cnlGaWxlTmFtZXM6ICdbbmFtZV0uanMnLFxuICAgICAgICBjaHVua0ZpbGVOYW1lczogKGNodW5rKSA9PiB7XG4gICAgICAgICAgLy8gQHRzLWV4cGVjdC1lcnJvciBGaXggdGhpc1xuICAgICAgICAgIGNvbnN0IHsgbW9kdWxlcyB9ID0gY2h1bmsgYXMge1xuICAgICAgICAgICAgbW9kdWxlczogUmVjb3JkPHN0cmluZywgeyBjb2RlOiBzdHJpbmcgfT47XG4gICAgICAgICAgfTtcbiAgICAgICAgICAvLyBlc2xpbnQtZGlzYWJsZS1uZXh0LWxpbmUgdW5pY29ybi9uby1hcnJheS1yZWR1Y2VcbiAgICAgICAgICBjb25zdCBjb2RlID0gT2JqZWN0LmtleXMobW9kdWxlcykucmVkdWNlKFxuICAgICAgICAgICAgKHByZXYsIGtleSkgPT4gYCR7cHJldn0ke21vZHVsZXNba2V5XSEuY29kZX1gLFxuICAgICAgICAgICAgJydcbiAgICAgICAgICApO1xuICAgICAgICAgIC8vIGVzbGludC1kaXNhYmxlLW5leHQtbGluZSBAdHlwZXNjcmlwdC1lc2xpbnQvbm8tdW5zYWZlLWFzc2lnbm1lbnQsIEB0eXBlc2NyaXB0LWVzbGludC9uby11bnNhZmUtbWVtYmVyLWFjY2VzcywgQHR5cGVzY3JpcHQtZXNsaW50L25vLXVuc2FmZS1jYWxsXG4gICAgICAgICAgY29uc3QgaGFzaCA9IE1ENS5oZXgoY29kZSk7XG5cbiAgICAgICAgICByZXR1cm4gYGFzc2V0cy9jaHVua3MuJHtoYXNofS5qc2A7XG4gICAgICAgIH0sXG4gICAgICAgIGFzc2V0RmlsZU5hbWVzOiAnW25hbWVdLltleHRdJyxcbiAgICAgIH0sXG4gICAgICBvbndhcm46ICh3YXJuaW5nLCB3YXJuKSA9PiB7XG4gICAgICAgIGlmICh3YXJuaW5nLmNvZGUgPT09ICdFVkFMJykge1xuICAgICAgICAgIHJldHVybjtcbiAgICAgICAgfVxuICAgICAgICB3YXJuKHdhcm5pbmcpO1xuICAgICAgfSxcbiAgICB9LFxuICAgIG91dERpcjogJy4uL3J1bnRpbWUtc2hhcmVkL3N0YXRpYycsXG4gICAgZW1wdHlPdXREaXI6IHRydWUsXG4gIH0sXG4gIG9wdGltaXplRGVwczoge1xuICAgIGV4Y2x1ZGU6IFsnQHZpYW1yb2JvdGljcy9ycGMnXSxcbiAgfSxcbiAgc2VydmVyOiB7XG4gICAgcG9ydDogNTE3NCxcbiAgfSxcbiAgcmVzb2x2ZToge1xuICAgIGFsaWFzOiB7XG4gICAgICAnQCc6IHBhdGgucmVzb2x2ZShcbiAgICAgICAgcGF0aC5kaXJuYW1lKHVybC5maWxlVVJMVG9QYXRoKGltcG9ydC5tZXRhLnVybCkpLFxuICAgICAgICAnLi9zcmMnXG4gICAgICApLFxuICAgIH0sXG4gIH0sXG59KTtcbiJdLAogICJtYXBwaW5ncyI6ICI7QUFBd08sU0FBUyxjQUFjO0FBQy9QLE9BQU8sWUFBWTtBQUNuQixTQUFTLG9CQUFvQjtBQUM3QixPQUFPLFVBQVU7QUFDakIsT0FBTyxTQUFTO0FBSjRILElBQU0sMkNBQTJDO0FBTzdMLElBQU0sTUFBTSxJQUFJLE9BQU8sSUFBSTtBQUVwQixJQUFNLFVBQVUsQ0FBQyxPQUFPLENBQUM7QUFHaEMsSUFBTyxzQkFBUSxhQUFhO0FBQUEsRUFDMUI7QUFBQSxFQUNBLE9BQU87QUFBQSxJQUNMLFdBQVc7QUFBQTtBQUFBO0FBQUE7QUFBQTtBQUFBLElBTVgsbUJBQW1CLE9BQU87QUFBQSxJQUMxQixlQUFlO0FBQUEsTUFDYixPQUFPO0FBQUEsUUFDTCxTQUFTO0FBQUEsTUFDWDtBQUFBLE1BQ0EsUUFBUTtBQUFBLFFBQ04sZ0JBQWdCO0FBQUEsUUFDaEIsZ0JBQWdCLENBQUMsVUFBVTtBQUV6QixnQkFBTSxFQUFFLFFBQVEsSUFBSTtBQUlwQixnQkFBTSxPQUFPLE9BQU8sS0FBSyxPQUFPLEVBQUU7QUFBQSxZQUNoQyxDQUFDLE1BQU0sUUFBUSxHQUFHLElBQUksR0FBRyxRQUFRLEdBQUcsRUFBRyxJQUFJO0FBQUEsWUFDM0M7QUFBQSxVQUNGO0FBRUEsZ0JBQU0sT0FBTyxJQUFJLElBQUksSUFBSTtBQUV6QixpQkFBTyxpQkFBaUIsSUFBSTtBQUFBLFFBQzlCO0FBQUEsUUFDQSxnQkFBZ0I7QUFBQSxNQUNsQjtBQUFBLE1BQ0EsUUFBUSxDQUFDLFNBQVMsU0FBUztBQUN6QixZQUFJLFFBQVEsU0FBUyxRQUFRO0FBQzNCO0FBQUEsUUFDRjtBQUNBLGFBQUssT0FBTztBQUFBLE1BQ2Q7QUFBQSxJQUNGO0FBQUEsSUFDQSxRQUFRO0FBQUEsSUFDUixhQUFhO0FBQUEsRUFDZjtBQUFBLEVBQ0EsY0FBYztBQUFBLElBQ1osU0FBUyxDQUFDLG1CQUFtQjtBQUFBLEVBQy9CO0FBQUEsRUFDQSxRQUFRO0FBQUEsSUFDTixNQUFNO0FBQUEsRUFDUjtBQUFBLEVBQ0EsU0FBUztBQUFBLElBQ1AsT0FBTztBQUFBLE1BQ0wsS0FBSyxLQUFLO0FBQUEsUUFDUixLQUFLLFFBQVEsSUFBSSxjQUFjLHdDQUFlLENBQUM7QUFBQSxRQUMvQztBQUFBLE1BQ0Y7QUFBQSxJQUNGO0FBQUEsRUFDRjtBQUNGLENBQUM7IiwKICAibmFtZXMiOiBbXQp9Cg==
