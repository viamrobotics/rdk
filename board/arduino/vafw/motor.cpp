
#include "motor.h"

extern HardwareSerial* debugSerial;

Motor::Motor(const char* name, int in1, int in2, int inDir, int inEn, int pwm)
    : _in1(in1), _in2(in2), _inDir(inDir), _inEn(inEn), _pwm(pwm), _moving(0), _regulated(false) {
    strcpy(_name, name);

    if (_in1 >= 0) pinMode(_in1, OUTPUT);
    if (_in2 >= 0) pinMode(_in2, OUTPUT);
    if (_inDir >= 0) pinMode(_inDir, OUTPUT);
    if (_inEn >= 0) pinMode(_inEn, OUTPUT);
    if (_pwm >= 0) pinMode(_pwm, OUTPUT);

    _power = 0;
}

void Motor::stop() {
    _regulated = false;
    _moving = 0;
    setPower(0);
}

void Motor::setPower(int power) {
    if (power < 0) {
        power = 0;
    } else if (power > 255) {
        power = 255;
    }
    _power = power;

    if (power == 0) {
        if (_inEn >= 0) digitalWrite(_inEn, HIGH);
        if (_pwm >= 0) digitalWrite(_pwm, LOW);
        if (_in1 >= 0 && _in2 >= 0) {
            digitalWrite(_in1, LOW);
            digitalWrite(_in2, LOW);
        }
        return;
    }

    int PWMPin = -1;
    if (_pwm >= 0) {
        PWMPin = _pwm;
    }else if (_moving == 1) {
        PWMPin = _in2;
        power = 255 - power; // Other pin is always high, so only when PWM is LOW are we driving. Thus, we invert here.
    } else if (_moving == -1){
        PWMPin = _in1;
        power = 255 - power; // Other pin is always high, so only when PWM is LOW are we driving. Thus, we invert here.
    }
    if (_inEn >= 0) digitalWrite(_inEn, LOW);
    if (PWMPin >= 0) analogWrite(PWMPin, power);
}

void Motor::go(bool forward, int power) {
    _regulated = false;

    if (forward) {
        _moving = 1;
        if (_inDir >= 0) {
            digitalWrite(_inDir, HIGH);
        }else{
            digitalWrite(_in1, HIGH);
            digitalWrite(_in2, LOW);
        }
    } else {
        _moving = -1;
        if (_inDir >= 0) {
            digitalWrite(_inDir, LOW);
        }else{
            digitalWrite(_in1, LOW);
            digitalWrite(_in2, HIGH);
        }
    }
    setPower(power); // Must be last for A/B only motors (where PWM will take over one of A or B)
}

void Motor::goFor(long ticksPerSecond, long ticks) {
    auto currentPosition = _encoder.position();
    _lastRPMCheck = millis();
    _lastRPMEncoderCount = currentPosition;

    goTo(ticksPerSecond, ticks + currentPosition);
}

void Motor::goTo(long ticksPerSecond, long ticks) {
    go(ticks > 0, 16);
    _ticksPerSecond = ticksPerSecond;
    _goal = ticks;
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
