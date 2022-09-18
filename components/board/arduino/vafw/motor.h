// motor.h

#pragma once

#include <Arduino.h>

#include "encoder.h"

class Motor {
   public:
    Motor(const char* name, int in1, int in2, int indir, int inen, int pwm);

    void stop();

    // power 0 -> 255
    void go(bool forward, int power);
    void goFor(long ticksPerSecond, long ticks);
    void goTo(long ticksPerSecond, long ticks);
    // power 0 -> 255
    void setPower(int power);

    void checkEncoder(long unsigned int now);

    IncrementalEncoder* encoder() { return _encoder; }
    const IncrementalEncoder* encoder() const { return _encoder; }
    void setIncrementalEncoder(IncrementalEncoder* e) { _encoder = e; }

    bool moving() const { return _moving != 0; }

    const char* name() const { return _name; }

   private:
    char _name[255];
    int _in1;
    int _in2;
    int _inDir;
    int _inEn;
    int _pwm;

    int _moving;  // 0: no, -1: backwards, 1: forwards
    int _power;   // 0 -> 255

    IncrementalEncoder* _encoder;

    bool _regulated;
    EncoderCount _goal;
    long _ticksPerSecond;

    unsigned long int _lastRPMCheck;
    EncoderCount _lastRPMEncoderCount;
};
