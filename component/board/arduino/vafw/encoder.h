// encoder.h
#include <Arduino.h>
#pragma once

typedef long EncoderCount;

class HallEncoder {
   public:
    HallEncoder(int pinA, int pinB);

    void encoderTick();
    void zero(long offset);

    EncoderCount position() const { return _position; }

   private:
    int _pinA;
    int _pinB;
    EncoderCount _position;
    EncoderCount _praw;
    uint16_t _pState;
};
