<template>
  <div class="bg-white p-4 flex items-center flex-wrap">
  <ul class="flex items-center border rounded-full px-2 leading-tight">
	<li 
    v-for="(crumb, ci) in crumbs"
    :key="ci"
    class="inline-flex items-center"
    >
	  <a href="#" class="text-gray-600 hover:text-blue-500">
		{{ crumb }}
	  </a>

	  <svg class="w-5 h-auto fill-current mx-2 text-gray-400" :class="{ disabled: isLast(ci) }" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0h24v24H0V0z" fill="none"/><path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6-6-6z"/></svg>
	</li>
  </ul>
</div>
</template>

<script>
export default {
  props: {
    crumbs: {
      type: Array,
      required: true,
    },
  },
  methods: {
    isLast(index) {
      return index === this.crumbs.length - 1;
    },
    selected(crumb) {
      this.$emit('selected', crumb);
    },
  },
};
</script>

<style scoped>
 .disabled {
   display: none;
 }
</style>