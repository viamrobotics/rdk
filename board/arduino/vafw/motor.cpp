
#include "motor.h"

extern HardwareSerial* debugSerial;

Motor::Motor(const char* name, int in1, int in2, int pwm)
    : _in1(in1),
      _in2(in2),
      _pwm(pwm),
      _moving(0),
      _regulated(false){
    strcpy(_name, name);
    pinMode(_in1, OUTPUT);
    pinMode(_in2, OUTPUT);
    pinMode(_pwm, OUTPUT);

    _power = 0;
}

void Motor::stop() {
    _regulated = false;
    _moving = 0;
    digitalWrite(_in1, LOW);
    digitalWrite(_in2, LOW);
}

void Motor::forward(int val, int ticks) {
    _power = val;
    this->setTicksToGo(ticks);
    _moving = 1;
    analogWrite(_pwm, val);
    digitalWrite(_in1, HIGH);
    digitalWrite(_in2, LOW);
}

void Motor::backward(int val, int ticks) {
    _power = val;
    this->setTicksToGo(-1 * ticks);
    _moving = -1;
    analogWrite(_pwm, val);
    digitalWrite(_in1, LOW);
    digitalWrite(_in2, HIGH);
}

void Motor::setTicksToGo(int ticks) {
    if (ticks == 0) {
        _regulated = false;
        return;
    }

    _regulated = true;
    _goal = _encoder.position() + ticks;
}

void Motor::checkEncoder() {
    if ( !_regulated) {
        return;
    }

    Serial.print(_encoder.position());
    Serial.print(" - ");
    Serial.print(_goal);
    Serial.println("");

    if ( (_moving > 0 && _encoder.position() >= _goal) ||
         (_moving < 0 && _encoder.position() <= _goal) ) {
        stop();
        return;
    }

}


