import { createApp } from 'vue';
import './rc/control_helpers';
import '@viamrobotics/prime';
import './index.css';
import App from './app.vue';
import './lib/resize';

createApp(App).mount('#app');
