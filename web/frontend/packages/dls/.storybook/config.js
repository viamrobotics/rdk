import { configure } from '@storybook/vue';
import Vue from "vue";
import "./index.css";

// Import your custom components.
import InputController from '../src/components/InputController';
import RadioButtons from '../src/components/RadioButtons';
import MotorDetail from '../src/components/MotorDetail';
import WebGamepad from '../src/components/WebGamepad';
import Camera from '../src/components/Camera';
import ViamButton from '../src/components/Button';
import ViamBadge from '../src/components/Badge';
import ViamInput from '../src/components/ViamInput';
import Container from '../src/components/Container';
import ToggleButton from '../src/components/ToggleButton';
import Grid from '../src/components/Grid';
import Collapse from '../src/components/Collapse';
import Accordion from '../src/components/Accordion';
import ViamSwitch from '../src/components/Switch';
import ViamTabs from '../src/components/ViamTabs';
import ViamTabsItem from '../src/components/ViamTabsItem';
import Breadcrumbs from '../src/components/Breadcrumbs';


// Register custom components.
Vue.component('InputController', InputController);
Vue.component('RadioButtons', RadioButtons);
Vue.component('MotorDetail', MotorDetail);
Vue.component('WebGamepad', WebGamepad);
Vue.component('Camera', Camera);
Vue.component('ViamButton', ViamButton);
Vue.component('Breadcrumbs', Breadcrumbs);
Vue.component('ViamBadge', ViamBadge);
Vue.component('ViamInput', ViamInput);
Vue.component('Container', Container);
Vue.component('ToggleButton', ToggleButton);
Vue.component('ViamSwitch', ViamSwitch);
Vue.component('Grid', Grid);
Vue.component('Collapse', Collapse);
Vue.component('ViamTabs', ViamTabs);
Vue.component('ViamTabsItem', ViamTabsItem);

const req = require.context('../src/stories', true, /.stories.js$/);
function loadStories() {
  req.keys().forEach(filename => req(filename));
}

function loadAddons() {
    require('storybook-addon-designs');
}

configure(loadStories, module);
configure(loadAddons, module);