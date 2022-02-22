<template>
  <div class="faq-accordion-combo">
    <button
      class="faq-accordion-combo__trigger"
      type="button"
      @click.prevent="toggle"
    >
      <div class="faq-accordion-combo__trigger-inner">
        <div class="faq-accordion-combo__trigger-label">{{ label }}</div>
      </div>
    </button>
    <VueAccordion :expanded="active" :duration="duration">
      <div class="faq-accordion-combo__content">
        <slot></slot>
        <ToggleButton />
      </div>
    </VueAccordion>
  </div>
</template>

<script>
import Vue from 'vue'
import VueAccordion from './VueAccordion.vue'
import ToggleButton from './ToggleButton.vue';
export default Vue.extend({
  components: {
    VueAccordion,
    ToggleButton
  },
  props: {
    label: {
      type: String,
      required: false,
      default:'camera'
    },
    duration: {
      type: Number,
      default: undefined,
    },
  },
  data: () => ({
    active: false,
  }),
  methods: {
    toggle() {
      this.active = !this.active
    },
  },
})
</script>

 <style  scoped>
@font-face {
  font-family: "Source Sans Pro";
  src: local("Source Sans Pro"),
   url(../assets/SpaceMono-Regular.ttf) format("truetype");
}
.faq-accordion-combo {
  font-size: inherit;
  border: 1px solid black;
  font-family: Source Sans Pro,sans-serif !important;
    font-size: 14px;
    font-weight: 400;
    background-color: var(--white);
    padding: 12px;
    border: 1px solid var(--black-30);
    filter: drop-shadow(0 2px 4px var(--black-30));
    margin-bottom: 20px;
}
.faq-accordion-combo + .faq-accordion-combo {
  margin-top: 10px;
}
.faq-accordion-combo__trigger {
  width: 100%;
  background: none;
  border: none;
  appearance: none;
  padding: 0;
}
.faq-accordion-combo__trigger:focus {
  outline: none;
}
.faq-accordion-combo__trigger-inner {
  padding: 12px 16px;
  display: flex;
  justify-content: flex-start;
  align-items: center;
}
.faq-accordion-combo__trigger-label {
  text-align: left;
  margin-right: 16px;
}
.faq-accordion-combo__content {
  padding: 12px 16px;
  border-top: 1px solid #000000;
}
 </style>