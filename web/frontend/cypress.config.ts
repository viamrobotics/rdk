import { defineConfig } from 'cypress';

export default defineConfig({
  includeShadowDom: true,

  e2e: {
    baseUrl: 'http://127.0.0.1:8080',

    setupNodeEvents (_on, _config) {
      // implement node event listeners here
    },
  },
});
