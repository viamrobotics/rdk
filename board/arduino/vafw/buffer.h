// buffer.h

#pragma once

#include <Arduino.h>

class Buffer {
   public:
    Buffer(HardwareSerial* s) : _port(s) {
        _port->begin(9600);
        _pos = 0;
    }

    // return true if got a new line
    bool readTillNewLine();

    const char* getLineAndReset();

    void println(const char* buf) { _port->println(buf); }

    void print(int n) { _port->print(n); }

   private:
    HardwareSerial* _port;

    char _buf[256];
    int _pos;
};


