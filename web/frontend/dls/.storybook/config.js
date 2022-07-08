import { configure } from '@storybook/vue';
import Vue from "vue";
import "./index.css";
import InputController from '../src/components/InputController';
import ViamIcon from '../src/components/ViamIcon';
import Range from '../src/components/Range';
import Collapse from '../src/components/Collapse';
import KeyboardInput from '../src/components/KeyboardInput';
import NumberInput from '../src/components/NumberInput';
import Tab from '../src/components/Tab';
import Tabs from '../src/components/Tabs';
import Base from '../src/components/Base';
import Camera from '../src/components/Camera';
import { library } from '@fortawesome/fontawesome-svg-core';
import { faCheckSquare } from '@fortawesome/free-regular-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome';
import ClickOutside from "../src/directives/clickOutside";
import MotorDetail from '../src/components/MotorDetail';

library.add(faCheckSquare)

Vue.component('font-awesome-icon', FontAwesomeIcon);
Vue.component('InputController', InputController);
Vue.component('Range', Range);
Vue.component('ViamIcon', ViamIcon);
Vue.component('Collapse', Collapse);
Vue.component('KeyboardInput', KeyboardInput);
Vue.component('NumberInput', NumberInput);
Vue.component('Tab', Tab);
Vue.component('Tabs', Tabs);
Vue.component('Base', Base);
Vue.component('Camera', Camera);
Vue.directive("click-outside", ClickOutside);
Vue.component('MotorDetail', MotorDetail);

const req = require.context('../src/stories', true, /.stories.js$/);
function loadStories() {
  req.keys().forEach(filename => req(filename));
}

function loadAddons() {
    require('storybook-addon-designs');
}

configure(loadStories, module);
configure(loadAddons, module);
