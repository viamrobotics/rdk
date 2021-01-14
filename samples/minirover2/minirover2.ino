
#include <Servo.h>

HardwareSerial* debugSerial;

struct Command {
    Command() : direction('s'), speed(255), ticks(0) {}
    Command(char d, int s, int t) : direction(d), speed(s), ticks(t) {}

    char direction;  // f, b, s
    int speed;       // [0, 255]
    int ticks;       // 0 means ignored, >= 0 means stop after that many
};

Command parseCommand(const char* buf) {
    Command c;

    if (!buf[0]) {
        return c;
    }

    c.direction = buf[0];
    buf++;

    if (!buf[0]) {
        return c;
    }

    c.speed = atoi(buf);
    if (c.speed <= 0 || c.speed > 255) {
        // bad data, do nothing
        c.direction = 's';
        c.speed = 0;
        return c;
    }

    // move pase the number to see if we have more data

    while (isdigit(buf[0])) {
        buf++;
    }

    if (buf[0] != ',') {
        return c;
    }
    buf++;  // move past the comma

    c.ticks = atoi(buf);

    return c;
}

void _testParseCommand(const char* buf, Command correct) {
    Command c = parseCommand(buf);
    if (c.direction == correct.direction && c.speed == correct.speed &&
        c.ticks == correct.ticks) {
        return;
    }

    Serial.println(buf);
    Serial.println("BROKE");
    exit(-1);
}

void testParseCommand() {
    _testParseCommand("s", Command('s', 255, 0));

    _testParseCommand("f", Command('f', 255, 0));
    _testParseCommand("f9", Command('f', 9, 0));
    _testParseCommand("f91", Command('f', 91, 0));
    _testParseCommand("f191", Command('f', 191, 0));
    _testParseCommand("f1000", Command('s', 0, 0));

    _testParseCommand("b91", Command('b', 91, 0));

    _testParseCommand("f100,100", Command('f', 100, 100));
}

class Motor {
   public:
    Motor(int in1, int in2, int pwm)
        : _in1(in1),
          _in2(in2),
          _pwm(pwm),
          _encoderTicks(0),
          _encoderTicksStop(0) {
        pinMode(_in1, OUTPUT);
        pinMode(_in2, OUTPUT);
        pinMode(_pwm, OUTPUT);
    }

    void stop() {
        digitalWrite(_in1, LOW);
        digitalWrite(_in2, LOW);
    }

    void forward(int val) {
        analogWrite(_pwm, val);
        digitalWrite(_in1, HIGH);
        digitalWrite(_in2, LOW);
    }

    void backward(int val) {
        analogWrite(_pwm, val);
        digitalWrite(_in1, LOW);
        digitalWrite(_in2, HIGH);
    }

    void doCommand(const char* buf) {
        Command c = parseCommand(buf);
        if (c.ticks == 0) {
            _encoderTicksStop = 0;
        } else {
            _encoderTicksStop = c.ticks + _encoderTicks;
        }

        switch (c.direction) {
            case 'f':
                debugSerial->println("forward");
                forward(c.speed);
                break;
            case 'b':
                debugSerial->println("backward");
                backward(c.speed);
                break;
            case 's':
                debugSerial->println("stop");
                stop();
                break;
            default:
                debugSerial->println("unknown command");
                debugSerial->println(buf[0], DEC);
        }
    }

    void checkEncoder() {
        if (_encoderTicksStop > 0 && _encoderTicks > _encoderTicksStop) {
            stop();
        }
    }

    uint64_t encoderTick() { return ++_encoderTicks; }

    uint64_t encoderTicks() const { return _encoderTicks; }

   private:
    int _in1;
    int _in2;
    int _pwm;
    uint64_t _encoderTicks;
    uint64_t _encoderTicksStop;
};

class Buffer {
   public:
    Buffer(HardwareSerial* s) : _port(s) {
        _port->begin(9600);
        _pos = 0;
    }

    // return true if got a new line
    bool readTillNewLine() {
        while (_port->available()) {
            int x = _port->read();
            if (x == '\n') {
                continue;
            }

            if (x == '\r') {
                _buf[_pos] = 0;
                return true;
            }

            if (_pos > 200) {
                Serial.println("bad bad");
                return false;
            }

            _buf[_pos++] = x;
        }

        return false;
    }

    const char* getLineAndReset() {
        _buf[_pos] = 0;
        _pos = 0;
        return _buf;
    }

   private:
    HardwareSerial* _port;

    char _buf[256];
    int _pos;
};

void processBuffer(Buffer* b);

int numMotors = 4;
Motor** motors;
Buffer* buf1;
Buffer* buf2;

Servo pan;
Servo tilt;

void setupInterrupt(int pin, void (*ISR)()) {
    pinMode(pin, INPUT);

    // enable internal pullup resistor
    digitalWrite(pin, HIGH);

    // Interrupt initialization
    attachInterrupt(digitalPinToInterrupt(pin), ISR, RISING);
}
void setup() {
    Serial.begin(9600);
    debugSerial = &Serial;

    testParseCommand();

    motors = new Motor*[numMotors];
    //                   in1  in2 pwm
    motors[0] = new Motor(51, 50, 5);
    motors[1] = new Motor(52, 53, 4);
    motors[2] = new Motor(24, 25, 6);
    motors[3] = new Motor(23, 22, 7);

    pan.attach(10);
    tilt.attach(11);

    buf1 = new Buffer(&Serial);
    buf2 = new Buffer(&Serial1);

    setupInterrupt(3, interrupt0);
    setupInterrupt(2, interrupt1);
    setupInterrupt(20, interrupt2);
    setupInterrupt(21, interrupt3);
}

void processBuffer(Buffer* b) {
    if (b->readTillNewLine()) {
        const char* line = b->getLineAndReset();

        if (line[0] == 'p') {
            int deg = atoi(line + 1);
            pan.write(deg);
        } else if (line[0] == 't') {
            int deg = atoi(line + 1);
            tilt.write(deg);
        } else {
            int motorNumber = line[0] - '0';
            if (motorNumber < 0 || motorNumber >= numMotors) {
                debugSerial->println("motor number invalid");
                debugSerial->println(motorNumber, DEC);
            }

            motors[motorNumber]->doCommand(line + 1);
        }
    }
}

int temp = 0;

void loop() {
    for (int i = 0; i < numMotors; i++) {
        motors[i]->checkEncoder();
    }

    processBuffer(buf1);
    processBuffer(buf2);
}

void interruptCallback(int num) { motors[num]->encoderTick(); }

void interrupt0() { interruptCallback(0); }

void interrupt1() { interruptCallback(1); }

void interrupt2() { interruptCallback(2); }

void interrupt3() { interruptCallback(3); }
