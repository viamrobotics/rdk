import { configure } from '@storybook/vue';
import Vue from "vue";
import "./index.css";

import InputController from '../src/components/InputController';
import ViamBadge from '../src/components/Badge';
import ViamButton from '../src/components/Button';
import ViamIcon from '../src/components/ViamIcon';
import Range from '../src/components/Range';
import ViamInput from '../src/components/ViamInput';
import Breadcrumbs from '../src/components/Breadcrumbs';
import Collapse from '../src/components/Collapse';
import Container from '../src/components/Container';
import ViamSwitch from '../src/components/Switch';
import Grid from '../src/components/Grid';
import { library } from '@fortawesome/fontawesome-svg-core';
import { faCheckSquare } from '@fortawesome/free-regular-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome';

library.add(faCheckSquare)

Vue.component('font-awesome-icon', FontAwesomeIcon);
Vue.component('InputController', InputController);
Vue.component('Range', Range);
Vue.component('ViamBadge', ViamBadge);
Vue.component('ViamButton', ViamButton);
Vue.component('ViamIcon', ViamIcon);
Vue.component('ViamInput', ViamInput);
Vue.component('Breadcrumbs', Breadcrumbs);
Vue.component('Collapse', Collapse);
Vue.component('Container', Container);
Vue.component('ViamSwitch', ViamSwitch);
Vue.component('Grid', Grid);

const req = require.context('../src/stories', true, /.stories.js$/);
function loadStories() {
  req.keys().forEach(filename => req(filename));
}

function loadAddons() {
    require('storybook-addon-designs');
}

configure(loadStories, module);
configure(loadAddons, module);
