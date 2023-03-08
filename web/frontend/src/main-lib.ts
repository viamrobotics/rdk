import './index.css';
import { createApp } from 'vue';
import App from './app.vue';

export const createRcApp = () => {
  return createApp(App);
};
