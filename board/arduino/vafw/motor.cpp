
#include "motor.h"

extern HardwareSerial* debugSerial;

Motor::Motor(const char* name, int in1, int in2, int pwm, bool trackSpeed)
    : _in1(in1),
      _in2(in2),
      _pwm(pwm),
      _encoderTicks(0),
      _encoderTicksStop(0),
      _trackSpeed(trackSpeed) {
    strcpy(_name, name);
    pinMode(_in1, OUTPUT);
    pinMode(_in2, OUTPUT);
    pinMode(_pwm, OUTPUT);
    _moving = false;
    _slowDown = false;
    _power = 0;
}

void Motor::stop() {
    _moving = false;
    _encoderTicksStop = 0;
    digitalWrite(_in1, LOW);
    digitalWrite(_in2, LOW);
}

void Motor::forward(int val, int ticks) {
    _power = val;
    this->setTicksToGo(ticks);
    _moving = true;
    analogWrite(_pwm, val);
    digitalWrite(_in1, HIGH);
    digitalWrite(_in2, LOW);
}

void Motor::backward(int val, int ticks) {
    _power = val;
    this->setTicksToGo(ticks);
    _moving = true;
    analogWrite(_pwm, val);
    digitalWrite(_in1, LOW);
    digitalWrite(_in2, HIGH);
}

void Motor::setTicksToGo(int ticks) {
    if (ticks <= 0) {
        _encoderTicksStop = 0;
    } else {
        _encoderTicksStop = ticks + _encoderTicks;
    }
    if (_trackSpeed) {
        _lastTick = millis();
    }
}

bool Motor::checkEncoder() {
    if (_encoderTicksStop <= 0) {
        return false;
    }

    if (_encoderTicks > _encoderTicksStop) {
        stop();
        return true;
    }

    if (_slowDown) {
        int newVal = 0;
        if (_encoderTicks + 50 > _encoderTicksStop) {
            newVal = int((double)_power * .5);

        } else if (_encoderTicks + 100 > _encoderTicksStop) {
            newVal = int((double)_power * .8);
        }

        if (newVal > 0) {
            analogWrite(_pwm, newVal);
            _pwm = newVal;
        }
    }

    return false;
}


