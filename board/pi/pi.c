#include <pigpio.h>

extern void pigpioInterruptCallback(int gpio, int level, uint32_t tick);

// doAnalogRead reads an SPI device given by h on the given channel. 
int doAnalogRead(int h, int channel) {
    char buf[3];
    buf[0] = 1;
    buf[1] = (8+channel) << 4;
    buf[2] = 0;
    spiXfer(h, buf, buf, 3);
    return ((buf[1]&3)<<8) | buf[2];
}

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
