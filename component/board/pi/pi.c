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

void setupInterrupt(int gpio) {
    gpioSetMode(gpio, PI_INPUT);
    gpioSetPullUpDown(gpio, PI_PUD_UP); // should this be configurable?
    gpioSetAlertFunc(gpio, interruptCallback);
}
