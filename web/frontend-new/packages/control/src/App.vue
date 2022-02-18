<template>
  <div id="control">
    <div id="nav">
      <router-link to="/">Home</router-link> |
      <router-link to="/about">About</router-link>
    </div>
    <router-view />
  </div>
</template>

<script>
export default {
  name: 'control',
  components: {
  }
}
</script>

<style>
@import "./assets/css/styles.css";
#control {
  width: 1080px;
}
</style>
