import { createApp } from 'vue';
import './rc/control_helpers';
import '@viamrobotics/prime';
import '@fontsource/space-mono/400.css';
import '@fontsource/space-mono/400-italic.css';
import '@fontsource/space-mono/700.css';
import '@fontsource/space-mono/700-italic.css';
import './index.css';
import App from './app.vue';

createApp(App).mount('#app');
