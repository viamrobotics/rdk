import { enableAutoDestroy, mount } from '@vue/test-utils';
import NumberInput from '@/components/NumberInput.vue';

describe('NumberInput', () => {
  enableAutoDestroy(afterEach);

  it('has html structure', async () => {
    const wrapper = mount({
      data() {
        return { value: 12 };
      },
      template: '<div> <number-input v-model="value"></number-input> </div>',
      components: { NumberInput },
    });
    const input = wrapper.find('input').element as HTMLInputElement;
    expect(wrapper.element.tagName).toBe('DIV');
    expect(input.value).toBe('12');
  });

  it('set value check', async () => {
    const wrapper = mount({
      data() {
        return { value: 12 };
      },
      template: '<div> <number-input v-model="value"></number-input> </div>',
      components: { NumberInput },
    });
    const input = wrapper.find('input');
    input.setValue('154');
    expect(wrapper.vm.$data.value).toBe(154);
  });

  it('increase check', async () => {
    const wrapper = mount({
      data() {
        return { value: 12 };
      },
      template: '<div> <number-input v-model="value"></number-input> </div>',
      components: { NumberInput },
    });
    wrapper.findAll('.arrow-icon').at(0).trigger('click');

    expect(wrapper.vm.$data.value).toBe(13);
  });
  it('decrease check', async () => {
    const wrapper = mount({
      data() {
        return { value: 12 };
      },
      template: '<div> <number-input v-model="value"></number-input> </div>',
      components: { NumberInput },
    });
    wrapper.findAll('.arrow-icon').at(1).trigger('click');

    // input.setValue("154");
    expect(wrapper.vm.$data.value).toBe(11);
  });

  it('step check', async () => {
    const wrapper = mount({
      data() {
        return { value: 12 };
      },
      template:
        '<div> <number-input v-model="value" :step="2"></number-input> </div>',
      components: { NumberInput },
    });
    wrapper.findAll('.arrow-icon').at(0).trigger('click');

    expect(wrapper.vm.$data.value).toBe(14);
  });
});
