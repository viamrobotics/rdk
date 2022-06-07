import { configure } from '@storybook/vue';
import Vue from "vue";
import "./index.css";
import InputController from '../src/components/InputController';
import ViamBadge from '../src/components/Badge';
import ViamButton from '../src/components/Button';
import ViamIcon from '../src/components/ViamIcon';
import Range from '../src/components/Range';
import ViamSelect from '../src/components/ViamSelect';
import ViamInput from '../src/components/ViamInput';
import Breadcrumbs from '../src/components/Breadcrumbs';
import Collapse from '../src/components/Collapse';
import Container from '../src/components/Container';
import ViamSwitch from '../src/components/Switch';
import Grid from '../src/components/Grid';
import KeyboardInput from '../src/components/KeyboardInput';
import NumberInput from '../src/components/NumberInput';
import Tab from '../src/components/Tab';
import Tabs from '../src/components/Tabs';
import Base from '../src/components/Base';
import RadioButtons from '../src/components/RadioButtons';
import Camera from '../src/components/Camera';
import { library } from '@fortawesome/fontawesome-svg-core';
import { faCheckSquare } from '@fortawesome/free-regular-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome';
import ViamInfoButton from '../src/components/ViamInfoButton';
import ClickOutside from "../src/directives/clickOutside";
import MotorDetail from '../src/components/MotorDetail';

library.add(faCheckSquare)

Vue.component('font-awesome-icon', FontAwesomeIcon);
Vue.component('InputController', InputController);
Vue.component('Range', Range);
Vue.component('ViamSelect', ViamSelect);
Vue.component('ViamBadge', ViamBadge);
Vue.component('ViamButton', ViamButton);
Vue.component('ViamIcon', ViamIcon);
Vue.component('ViamInput', ViamInput);
Vue.component('Breadcrumbs', Breadcrumbs);
Vue.component('Collapse', Collapse);
Vue.component('Container', Container);
Vue.component('ViamSwitch', ViamSwitch);
Vue.component('Grid', Grid);
Vue.component('KeyboardInput', KeyboardInput);
Vue.component('NumberInput', NumberInput);
Vue.component('Tab', Tab);
Vue.component('Tabs', Tabs);
Vue.component('Base', Base);
Vue.component('RadioButtons', RadioButtons);
Vue.component('Camera', Camera);
Vue.component('ViamInfoButton', ViamInfoButton);
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
