

HardwareSerial* debugSerial;

class Motor {
   public:
    Motor(int in1, int in2, int pwm) : _in1(in1), _in2(in2), _pwm(pwm) {
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
        int power = 255;

        if (buf[1] >= '0' && buf[1] <= '9') {
            power = atoi(buf + 1);
        }

        Serial.println(power, DEC);

        switch (buf[0]) {
            case 'f':
                debugSerial->println("forward");
                forward(power);
                break;
            case 'b':
                debugSerial->println("backward");
                backward(power);
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

   private:
    int _in1;
    int _in2;
    int _pwm;
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

int numMotors = 4;
Motor** motors;
Buffer* buf1;
Buffer* buf2;

void setup() {
    Serial.begin(9600);
    debugSerial = &Serial;

    motors = new Motor*[numMotors];
    //                   in1  in2 pwm
    motors[0] = new Motor(52, 53, 2);
    motors[1] = new Motor(51, 50, 3);
    motors[2] = new Motor(24, 25, 6);
    motors[3] = new Motor(23, 22, 7);

    buf1 = new Buffer(&Serial);
    buf2 = new Buffer(&Serial1);
}

void process(Buffer* b) {
    if (b->readTillNewLine()) {
        const char* line = b->getLineAndReset();
        int motorNumber = line[0] - '0';
        if (motorNumber < 0 || motorNumber >= numMotors) {
            debugSerial->println("motor number invalid");
            debugSerial->println(motorNumber, DEC);
        }

        motors[motorNumber]->doCommand(line + 1);
    }
}

void loop() {
    process(buf1);
    process(buf2);
}
