#include "buffer.h"

// return true if got a new line
bool Buffer::readTillNewLine() {
    while (_port->available()) {
        int x = _port->read();

        if (x == '\r') {
            continue;
        }

        if (x == '\n') {
            _buf[_pos] = 0;
            return true;
        }

        if (_pos > 200) {
            // probably garbage data, just get rid of it
            _pos = 0;
            return false;
        }

        _buf[_pos++] = x;
    }

    return false;
}

const char* Buffer::getLineAndReset() {
    _buf[_pos] = 0;
    _pos = 0;
    return _buf;
}
