// core.h

#pragma once

#include <Arduino.h>

class Motor {
   public:
    Motor(const char* name, int in1, int in2, int pwm, bool trackSpeed = false);

    void stop();
    void forward(int val, int ticks = 0);
    void backward(int val, int ticks = 0);

    void setTicksToGo(int ticks);

    bool checkEncoder();

    uint64_t encoderTick() {
        if (_trackSpeed) {
            _lastTick = millis();
        }
        return ++_encoderTicks;
    }

    uint64_t encoderTicks() const { return _encoderTicks; }
    uint64_t encoderTicksStop() const { return _encoderTicksStop; }

    bool moving() const { return _moving; }

    unsigned long lastTick() const {
        if (!_trackSpeed) {
            Serial.println("lastTick called but trackSpeed off");
            return 0;
        }
        return _lastTick;
    }

    void setSlowDown(bool b) { _slowDown = b; }

    const char* name() const { return _name; }
    
   private:
    char _name[255];
    int _in1;
    int _in2;
    int _pwm;
    uint64_t _encoderTicks;
    uint64_t _encoderTicksStop;
    bool _moving;
    bool _trackSpeed;
    unsigned long _lastTick;
    bool _slowDown;
    int _power;
};

void setupInterrupt(int pin, void (*ISR)(), int what);
