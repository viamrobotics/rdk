/* eslint-disable @typescript-eslint/no-explicit-any */
import { defineComponent, h } from 'vue';

export const svelteAdapter = (Component: any, style: any = {}, tag = 'span') => {
  return defineComponent({
    data () {
      return {
        comp: null as any,
      };
    },
    mounted () {
      this.comp = new Component({
        target: this.$refs.container,
        props: this.$attrs,
      });

      const watchers: any[] = [];

      for (const [key, value] of Object.entries(this.$attrs)) {
        if (key.startsWith('on') === false) {
          continue;
        }

        this.comp.$on(key.replace('on', '').toLowerCase(), value);

        // eslint-disable-next-line prefer-named-capture-group
        const watchMatch = key.match(/watch:([^]+)/u);

        if (watchMatch && typeof value === 'function') {
          watchers.push([`${watchMatch[1]![0]!.toLowerCase()}${watchMatch[1]!.slice(1)}`, value]);
        }
      }

      if (watchers.length > 0) {
        const { comp } = this;
        const { update } = this.comp.$$;
        this.comp.$$.update = (...args: any[]) => {
          for (const [name, callback] of watchers) {
            const index = comp.$$.props[name];
            callback(comp.$$.ctx[index]);
          }
          Reflect.apply(update, null, args);
        };
      }
    },
    updated () {
      this.comp.$set(this.$attrs);
    },
    unmounted () {
      this.comp.$destroy();
    },
    render () {
      return h(tag, {
        ref: 'container',
        style,
      });
    },
  });
};
