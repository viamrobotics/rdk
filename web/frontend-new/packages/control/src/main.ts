import Vue from "vue";
import App from "./App.vue";
import router from "./router";
import store from "./store/store";
import CameraClientWrapper from "./services/cameraClientWrapper";

import "./index.css";
// import i18n from "./i18n";

const client: CameraClientWrapper = new CameraClientWrapper();

Vue.config.productionTip = false;

router.beforeEach((to, from, next) => {
  if (to.path === "/about") {
    next();
    return;
  }
  
  // if store is initialized go straight to next route, if not load the app
  if (store.state.isInitialized) {
    next();
    return;
  }
  store.commit("setIsInitialized");
  next();
  return;

  // const cameraFramePromise = client.renderFrame("test", "image/jpeg");

  // Promise.all([cameraFramePromise]).then(
  //   (values) => {
  //     store.commit("updateCameraFrame", values[0]);
  //     store.commit("setIsInitialized");
  //     next();
  //     return;
  //   }
  // );
});

new Vue({
  router,
  store,
  // i18n,
  render: (h) => h(App),
}).$mount("#control");

// const app = new Vue({
//   el: "#control",
//   store,
//   router,
//   components: {},
//   mounted() {
//     setInterval(() => {
//       client.renderFrame("test", "image/jpeg").then((frame) => {
//         store.commit("updateCameraFrame", frame);
//       });
//     }, 180000); //3mins or in future based on control
//   },
// });
