import { enableAutoDestroy, mount } from "@vue/test-utils";
import KeyboardInput from "@/components/KeyboardInput.vue";

describe("ViamButton", () => {
  enableAutoDestroy(afterEach);

  const testEventFire = (keys: string[], eventName: string) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const wrapper: any = mount(KeyboardInput);
    keys.forEach((keyName) => wrapper.vm.setKeyPressed(keyName, true));
    wrapper.vm.handleKeysStateInstantly();
    expect(wrapper.emitted()[eventName]).toBeTruthy();
  };

  it("has html structure", async () => {
    const wrapper = mount(KeyboardInput);

    expect(wrapper.element.tagName).toBe("DIV");
  });

  it("check forward key fires event", async () => {
    testEventFire(["forward"], "keyboard-ctl");
  });

  // multiple buttons
  it("check forward&right keys fires event", async () => {
    testEventFire(["forward", "right"], "keyboard-ctl");
  });
});
