import { createApp } from 'vue';
import '@viamrobotics/prime';
import './index.css';
import App from './app.vue';

const delay = (ms: number) => new Promise((resolve) => {
  setTimeout(resolve, ms);
});

setInterval(async () => {
  const app = createApp(App);
  app.mount('#app');

  await delay(1000);

  app.unmount();
}, 1000);
