// encoder.cpp

#include "encoder.h"

HallEncoder::HallEncoder() : _a(0), _b(0), _position(0) {}

void HallEncoder::encoderTick(bool a) {
    if (a) {
        _a = !_a;
    } else {
        _b = !_b;
    }

    if (!_a && !_b) {  // state 1
        if (a) {
            _position++;
        } else {
            _position--;
        }
    } else if (!_a && _b) {  // state 2
        if (a) {
            _position--;
        } else {
            _position++;
        }
    } else if (_a && _b) {  // state 3
        if (a) {
            _position++;
        } else {
            _position--;
        }
    } else if (_a && !_b) {
        if (a) {
            _position--;
        } else {
            _position++;
        }
    }
}
