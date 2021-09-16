import Vue, { VueConstructor } from "vue";
import HelloWorld from "./components/HelloWorld.vue";

const Components: { [key: string]: VueConstructor<Vue> } = {
  HelloWorld,
};

Object.keys(Components).forEach((name) => {
  Vue.component(name, Components[name]);
});

export default Components;
