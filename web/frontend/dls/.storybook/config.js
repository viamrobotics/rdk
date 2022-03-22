import { configure } from '@storybook/vue';
import Vue from "vue";
import "./index.css";

import InputController from '../src/components/InputController';
import ViamBadge from '../src/components/Badge';

import { library } from '@fortawesome/fontawesome-svg-core';
import { faCheckSquare } from '@fortawesome/free-regular-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome';

library.add(faCheckSquare)

Vue.component('font-awesome-icon', FontAwesomeIcon);
Vue.component('InputController', InputController);
Vue.component('ViamBadge', ViamBadge);

const req = require.context('../src/stories', true, /.stories.js$/);
function loadStories() {
  req.keys().forEach(filename => req(filename));
}

function loadAddons() {
    require('storybook-addon-designs');
}

configure(loadStories, module);
configure(loadAddons, module);
