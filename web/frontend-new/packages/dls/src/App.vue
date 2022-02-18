<template>
  <div id="app">
    <div id="nav">
      <router-link to="/">Home</router-link> |
      <router-link to="/about">About</router-link>
    </div>
    <router-view />
  </div>
</template>

<script>
import Camera from "./components/Camera.vue";
export default {
  name: 'app',
  components: {
    Camera
  }
}
</script>

<style>
@import "./assets/css/styles.css";
#app {
  width: 1080px;
}
</style>
