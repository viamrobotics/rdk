#pragma once

// doAnalogRead reads an SPI device given by h on the given channel. 
int doAnalogRead(int h, int channel);

// interruptCallback calls through to the go linked interrupt callback.
void setupInterrupt(int gpio);
