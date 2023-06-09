import '@viamrobotics/prime';
import './tailwind.css';
import './index.css';
import App from './app.svelte';

export default new App({
  target: document.querySelector('#app')!,
});
