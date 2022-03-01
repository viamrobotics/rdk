import Vue from "vue";
import App from "./App.vue";
import router from "./router";
import store from "./store";
import "./index.css";
// import i18n from "./i18n";

/* import the fontawesome core */
import { library } from '@fortawesome/fontawesome-svg-core'
import { faAddressBook } from '@fortawesome/free-regular-svg-icons'
/* import font awesome icon component */
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'

library.add(faAddressBook)
Vue.component('font-awesome-icon', FontAwesomeIcon)
Vue.config.productionTip = false;

new Vue({
  router,
  store,
  // i18n,
  render: (h) => h(App),
}).$mount("#app");
