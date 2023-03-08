import './index.css?inline';
import { createApp } from 'vue';
import App from './app.vue';

export const createRcApp = () => {
  return createApp(App);
};
