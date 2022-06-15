<template>
  <div id="app">
    <div id="nav">
      <font-awesome-icon icon="fa-regular fa-address-book" />
      <router-link to="/">Home</router-link> |
      <router-link to="/about">About</router-link>
    </div>
    <router-view />
  </div>
</template>

<script>
export default {
  name: 'app',
  components: {},
};
</script>

<style>
@import './assets/css/styles.css';
#app {
  width: 1080px;
}
</style>
