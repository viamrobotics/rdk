//go:build !no_pigpio
#include <pigpio.h>

extern void pigpioInterruptCallback(int gpio, int level, uint32_t tick);

// interruptCallback calls through to the go linked interrupt callback.
void interruptCallback(int gpio, int level, uint32_t tick) {
    if (level == 2) {
        // watchdog
        return;
    }
    pigpioInterruptCallback(gpio, level, tick);
}

int setupInterrupt(int gpio) {
    int result = gpioSetMode(gpio, PI_INPUT);
    if (result != 0) {
        return result;
    }
    result = gpioSetPullUpDown(gpio, PI_PUD_UP); // should this be configurable?
    if (result != 0) {
        return result;
    }
    result = gpioSetAlertFunc(gpio, interruptCallback);
    return result;
}

int teardownInterrupt(int gpio) {
    int result = gpioSetAlertFunc(gpio, NULL);
    // Do we need to unset the pullup resistors?
    return result;
}
