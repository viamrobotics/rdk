import { configure } from '@storybook/vue';
import Vue from "vue";
import "./index.css";

// Import your custom components.
import InputController from '../src/components/InputController';

/* import the fontawesome core */
import { library } from '@fortawesome/fontawesome-svg-core'
import { faCheckSquare } from '@fortawesome/free-regular-svg-icons'
/* import font awesome icon component */
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'

library.add(faCheckSquare)

Vue.component('font-awesome-icon', FontAwesomeIcon);

// Register custom components.
Vue.component('InputController', InputController);

const req = require.context('../src/stories', true, /.stories.js$/);
function loadStories() {
  req.keys().forEach(filename => req(filename));
}

function loadAddons() {
    require('storybook-addon-designs');
}

configure(loadStories, module);
configure(loadAddons, module);