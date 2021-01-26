
#pragma once

#include <Arduino.h>

class Motor {
   public:
    Motor(int in1, int in2, int pwm);

    void stop();
    void forward(int val);
    void backward(int val);

    void doCommand(const char* buf);

    bool checkEncoder();

    uint64_t encoderTick() { return ++_encoderTicks; }

    uint64_t encoderTicks() const { return _encoderTicks; }

    bool moving() const { return _moving; }

   private:
    int _in1;
    int _in2;
    int _pwm;
    uint64_t _encoderTicks;
    uint64_t _encoderTicksStop;
    bool _moving;
};

struct Command {
    Command() : direction('s'), speed(255), ticks(0) {}
    Command(char d, int s, int t) : direction(d), speed(s), ticks(t) {}

    static Command parse(const char* buf);

    char direction;  // f, b, s
    int speed;       // [0, 255]
    int ticks;       // 0 means ignored, >= 0 means stop after that many
};

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

   private:
    HardwareSerial* _port;

    char _buf[256];
    int _pos;
};

void testParseCommand();
