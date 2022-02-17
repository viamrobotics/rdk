import { configure } from '@storybook/vue';
import Vue from "vue";
import VueI18n from "vue-i18n";
import "./index.css";

Vue.use(VueI18n);

// Import your custom components.
import InputController from '../src/components/InputController';
import RadioButtons from '../src/components/RadioButtons';
import MotorDetail from '../src/components/MotorDetail';
import WebGamepad from '../src/components/WebGamepad';
import Camera from '../src/components/Camera';

// Register custom components.
Vue.component('InputController', InputController);
Vue.component('RadioButtons', RadioButtons);
Vue.component('MotorDetail', MotorDetail);
Vue.component('WebGamepad', WebGamepad);
Vue.component('Camera', Camera);

const req = require.context('../src/stories', true, /.stories.js$/);
function loadStories() {
  req.keys().forEach(filename => req(filename));
}

function loadAddons() {
    require('storybook-addon-designs');
}

configure(loadStories, module);
configure(loadAddons, module);