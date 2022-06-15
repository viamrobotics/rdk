import { enableAutoDestroy, shallowMount } from '@vue/test-utils';
import ViamInput from '@/components/ViamInput.vue';

describe('ViamInput', () => {
  enableAutoDestroy(afterEach);

  it('has html structure', async () => {
    const wrapper = shallowMount(ViamInput);
    const $input = wrapper.find('input');

    expect(wrapper.element.tagName).toBe('LABEL');
    expect($input.attributes('type')).toBeDefined();
    expect($input.attributes('type')).toEqual('text');
    expect($input.attributes('disabled')).not.toBeDefined();
    expect($input.attributes('name')).not.toBeDefined();
    expect($input.attributes('required')).not.toBeDefined();
  });

  it('render content inside slot', async () => {
    const wrapper = shallowMount(ViamInput, {
      slots: {
        default: '<span>content</span>',
      },
    });

    expect(wrapper.find('span').exists()).toBe(true);
    expect(wrapper.text()).toBe('content');
  });

  it('do not renders custom root element', async () => {
    const wrapper = shallowMount(ViamInput, {
      propsData: {
        tag: 'div',
      },
    });

    expect(wrapper.element.tagName).toBe('LABEL');
  });

  it('accepts status properties', () => {
    const wrapper = shallowMount(ViamInput, {
      propsData: {
        status: 'error',
      },
    });

    const $input = wrapper.find('input');
    expect($input.classes('border-danger-500')).toBe(true);
  });

  it('has attribute disabled when input is disabled', () => {
    const wrapper = shallowMount(ViamInput, {
      propsData: {
        disabled: true,
      },
    });

    const $input = wrapper.find('input');
    expect($input.attributes('disabled')).toBeDefined();
  });

  it('should emit events', async () => {
    let called = 0;
    let event = null;
    const wrapper = shallowMount(ViamInput, {
      listeners: {
        blur: (e: Event) => {
          event = e;
          called += 1;
        },
        click: (e: Event) => {
          event = e;
          called += 1;
        },
        focus: (e: Event) => {
          event = e;
          called += 1;
        },
        input: (e: Event) => {
          event = e;
          called += 1;
        },
      },
    });
    const $input = wrapper.find('input');

    expect(called).toBe(0);
    expect(event).toEqual(null);

    await wrapper.trigger('click');
    expect(called).toBe(1);
    expect(event).toBeInstanceOf(MouseEvent);

    await $input.trigger('blur');
    expect(called).toBe(2);
    expect(event).toBeInstanceOf(FocusEvent);

    await $input.trigger('focus');
    expect(called).toBe(3);
    expect(event).toBeInstanceOf(FocusEvent);

    await $input.setValue('foobar');
    expect(called).toBe(4);
    expect(event).toBe('foobar');
  });

  it("shouldn't emit click event when clicked and disabled", async () => {
    let called = 0;
    const wrapper = shallowMount(ViamInput, {
      propsData: {
        disabled: true,
      },
      listeners: {
        click: () => {
          called += 1;
        },
      },
    });

    expect(called).toBe(0);
    await wrapper.trigger('click');
    expect(called).toBe(0);
  });
});
