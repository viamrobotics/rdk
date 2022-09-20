// encoder.cpp

#include "encoder.h"

// State Transition Table
//     +---------------+----+----+----+----+
//     | pState/nState | 00 | 01 | 10 | 11 |
//     +---------------+----+----+----+----+
//     |       00      | 0  | -1 | +1 | x  |
//     +---------------+----+----+----+----+
//     |       01      | +1 | 0  | x  | -1 |
//     +---------------+----+----+----+----+
//     |       10      | -1 | x  | 0  | +1 |
//     +---------------+----+----+----+----+
//     |       11      | x  | +1 | -1 | 0  |
//     +---------------+----+----+----+----+
// 0 -> same state
// x -> impossible state

IncrementalEncoder::IncrementalEncoder(int pinA, int pinB)
    : _pinA(pinA), _pinB(pinB), _position(0), _praw(0) {
    pinMode(_pinA, INPUT_PULLUP);
    pinMode(_pinB, INPUT_PULLUP);
    _pState = digitalRead(_pinA) | (digitalRead(_pinB) << 1);
}

void IncrementalEncoder::zero(long offset) {
    _position = offset;
    _praw = (offset << 1) | (_praw & 0x1);
}

void IncrementalEncoder::encoderTick() {
    uint16_t nState = digitalRead(_pinA) | (digitalRead(_pinB) << 1);
    if (nState == _pState) {
        return;
    }
    switch ((_pState << 2) | nState) {
        case 0b0001:
        case 0b0111:
        case 0b1000:
        case 0b1110:
            _praw--;
            _position = _praw >> 1;
            _pState = nState;
            break;
        case 0b0010:
        case 0b0100:
        case 0b1011:
        case 0b1101:
            _praw++;
            _position = _praw >> 1;
            _pState = nState;
            break;
    }
}
