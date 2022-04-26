import { enableAutoDestroy, mount } from '@vue/test-utils';
import KeyboardInput from '@/components/KeyboardInput.vue';

describe('ViamButton', () => {
  enableAutoDestroy(afterEach);

  const testEventFire = (keys: string[], eventName: string) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const wrapper: any = mount(KeyboardInput);
    keys.forEach((keyName) => wrapper.vm.setKeyPressed(keyName, true));
    wrapper.vm.handleKeysStateInstantly();
    expect(wrapper.emitted()[eventName]).toBeTruthy();
  };

  it('has html structure', () => {
    const wrapper = mount(KeyboardInput);

    expect(wrapper.element.tagName).toBe('DIV');
  });

  it('check forward key fires event', () => {
    testEventFire(['forward'], 'forward');
  });
  it('check backward key fires event', () => {
    testEventFire(['backward'], 'backward');
  });
  it('check left key fires event', () => {
    testEventFire(['left'], 'spin-counter-clockwise');
  });
  it('check right key fires event', () => {
    testEventFire(['right'], 'spin-clockwise');
  });

  it('check right key fires event', () => {
    testEventFire(['right'], 'spin-clockwise');
  });

  //here are several buttons checkers
  it('check forward&right keys fires event', () => {
    testEventFire(['forward', 'right'], 'arc-right');
  });
  it('check forward&left keys fires event', () => {
    testEventFire(['forward', 'left'], 'arc-left');
  });
  it('check backward&left keys fires event', () => {
    testEventFire(['backward', 'left'], 'back-arc-left');
  });
  it('check backward&right keys fires event', () => {
    testEventFire(['backward', 'right'], 'back-arc-right');
  });
});
