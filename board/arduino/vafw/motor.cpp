
#include "motor.h"

extern HardwareSerial* debugSerial;

Motor::Motor(const char* name, int in1, int in2, int pwm)
    : _in1(in1), _in2(in2), _pwm(pwm), _moving(0), _regulated(false) {
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

void Motor::setPower(int power) {
    if (power < 0) {
        power = 0;
    } else if (power > 255) {
        power = 255;
    }
    _power = power;
    analogWrite(_pwm, power);
}

void Motor::go(bool forward, int power) {
    _regulated = false;
    setPower(power);

    if (forward) {
        _moving = 1;
        digitalWrite(_in1, HIGH);
        digitalWrite(_in2, LOW);
    } else {
        _moving = -1;
        digitalWrite(_in1, LOW);
        digitalWrite(_in2, HIGH);
    }
}

void Motor::goFor(long ticksPerSecond, long ticks) {
    auto currentPosition = _encoder.position();
    _lastRPMCheck = millis();
    _lastRPMEncoderCount = currentPosition;

    setPower(16);

    if (ticks > 0) {
        _moving = 1;
        digitalWrite(_in1, HIGH);
        digitalWrite(_in2, LOW);
    } else {
        _moving = -1;
        digitalWrite(_in1, LOW);
        digitalWrite(_in2, HIGH);
    }

    _ticksPerSecond = ticksPerSecond;
    _goal = currentPosition + ticks;
    _regulated = true;
}

void Motor::checkEncoder(long unsigned int now) {
    if (!_regulated) {
        return;
    }

    auto currentPosition = _encoder.position();

    if ((_moving > 0 && currentPosition >= _goal) ||
        (_moving < 0 && currentPosition <= _goal)) {
        stop();
        return;
    }

    long unsigned int millisBetween = 333;
    auto timeDiff = now - _lastRPMCheck;
    if (timeDiff > millisBetween) {
        // it's been more than time limit, so we do a check
        auto ticksPerSecond = long(abs(currentPosition - _lastRPMEncoderCount) *
                                   (1000 / millisBetween));

        if (ticksPerSecond == 0) {
            if (_power < 16) {
                setPower(16);
            } else {
                setPower(_power * 2);
            }
        } else if (ticksPerSecond > _ticksPerSecond) {
            setPower(_power / 1.1);
        } else if (ticksPerSecond < _ticksPerSecond) {
            setPower(_power * 1.1);
        }
        /*
        Serial.print(currentPosition);
        Serial.print(" ");
        Serial.print(_lastRPMEncoderCount);
        Serial.print(" ");
        */
        Serial.print(currentPosition - _lastRPMEncoderCount);
        Serial.print(" ");
        Serial.print(_ticksPerSecond);
        Serial.print(" ");
        Serial.print(ticksPerSecond);
        Serial.print(" ");
        Serial.print(_power);
        Serial.println(" ");

        _lastRPMCheck = now;
        _lastRPMEncoderCount = currentPosition;
    }
}
