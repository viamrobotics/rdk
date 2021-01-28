
#include <RCore.h>

HardwareSerial* debugSerial;

void logMsg(const char* msg) {
    char buf[128];
    sprintf(buf, "@%s", msg);
    Serial.println(buf);
}

void debug(const char* name, int x) {
    char buf[16];
    sprintf(buf, "@%s: %d", name, x);
    Serial.println(buf);
}

class Gripper {
   public:
    Gripper(Motor* m)
        : _motor(m),
          _initialized(false),
          _initializeState(0),
          _fullTicks(500),
          _power(128) {
        _motor->setSlowDown(true);
    }

    bool initialzeReady() {
        if (_initialized) {
            return true;
        }

        if (_initializeState == INIT_STATE_START) {  // haven't started yet
            logMsg("Gripper::initialzeReady opening 1");
            _currentSpot = 0;
            open(_fullTicks);
            _initializeState = INIT_STATE_OPEN1;
            return false;
        }

        auto howLong = millis() - _motor->lastTick();
        int howLongThreshold = 300;
        if (_currentSpot == 0) {
            howLongThreshold *= 3;
        }
        if (_motor->moving() && howLong < howLongThreshold) {
            return false;
        }

        auto encoderTicksStop = _motor->encoderTicksStop();
        _motor->stop();

        auto numTicks =
            _motor->encoderTicks() - (encoderTicksStop - _fullTicks);

        if (_initializeState == INIT_STATE_OPEN2) {
            if (_fullTicks == 0) {
                logMsg("_fullTicks is 0, starting over");
                _initializeState = INIT_STATE_START;
                _fullTicks = 500;
                return false;
            }
            _currentSpot = 0;
            _initialized = true;

            if (encoderTicksStop == 0) {
                debug("perfect", _fullTicks);
                _initialized = true;
                return true;
            }

            debug("medium", _fullTicks);

            return true;
        }

        if (encoderTicksStop == INIT_STATE_START) {
            logMsg("it stopped on it's own, this is unexpected");
            return false;
        }

        if (_initializeState == INIT_STATE_OPEN1) {  // we opened, now we close
            _initializeState = INIT_STATE_CLOSE;
            logMsg("Gripper::initialzeReady closing 1");
            _currentSpot = 0;
            close(_fullTicks);
            return false;
        }

        if (_initializeState ==
            INIT_STATE_CLOSE) {  // we closed, how many ticks was there
            _fullTicks = numTicks;
            open(_fullTicks);
            _initializeState = INIT_STATE_OPEN2;
            return false;
        }

        debug("unknown _initializeState", _initializeState);

        return false;
    }

    bool moving() const { return _motor->moving(); }

    void checkEncoder() {
        _motor->checkEncoder();
        if (_initialized && moving()) {
            auto howLong = millis() - _motor->lastTick();
            if (howLong > 500) {
                //_motor->stop();
            }
        }
    }

    void encoderTick() {
        _motor->encoderTick();
        if (_opening) {
            _currentSpot--;
            if (_currentSpot < 0) {
                _currentSpot = 0;
            }
        } else {
            _currentSpot++;
            if (_currentSpot > _fullTicks) {
                _currentSpot = _fullTicks;
            }
        }
    }

    void setPos(double pos) { setRawPos(int(pos * _fullTicks)); }

    double getPos() const { return double(_currentSpot) / double(_fullTicks); }
    int getRawPos() const { return _currentSpot; }

    // g0.3 -- go to position .0
    // p  - return current position
    void processCommand(const char* command, Buffer* b) {
        if (command[0] == 'g') {
            auto pos = atof(command + 1);
            debug("got g", pos);
            setPos(pos);
        } else if (command[0] == 'p') {
            auto pos = int(100 * getPos());
            if (pos >= 100) {
                b->println("gp:1.0");
            } else {
                char buf[16];
                auto x = sprintf(buf, "gp:.%.2d %d", pos, getRawPos());
                b->println(buf);
            }
        } else {
            b->println("bad command");
        }
    }

   private:
    void setRawPos(int pos) {
        if (pos < 0) {
            pos = 0;
        } else if (pos > _fullTicks) {
            pos = _fullTicks;
        }

        if (pos == _currentSpot) {
            return;
        } else if (pos < _currentSpot) {
            open(_currentSpot - pos);
        } else {
            close(pos - _currentSpot);
        }
    }

    void open(int ticks) {
        _opening = true;
        _motor->forward(_power, ticks);
    }

    void close(int ticks) {
        _opening = false;
        _motor->backward(_power, ticks);
    }

    Motor* _motor;
    int _fullTicks;
    int _power;

    bool _opening;     // true is opening, false is closing
    int _currentSpot;  // 0 is open

    bool _initialized;
    int _initializeState;

    static const int INIT_STATE_START = 0;
    static const int INIT_STATE_OPEN1 = 1;
    static const int INIT_STATE_CLOSE = 2;
    static const int INIT_STATE_OPEN2 = 3;
};

Gripper* g;
Buffer* b;

void setup() {
    b = new Buffer(&Serial);
    debugSerial = &Serial;

    g = new Gripper(new Motor(6, 7, 9, true));

    pinMode(2, INPUT);
    digitalWrite(2, HIGH);  // enable internal pullup resistor
    attachInterrupt(digitalPinToInterrupt(2), interruptName,
                    RISING);  // Interrupt initialization
}

void loop() {
    g->checkEncoder();

    if (g->initialzeReady()) {
        if (b->readTillNewLine()) {
            auto line = b->getLineAndReset();
            if (line[0] == 'g') {
                g->processCommand(line + 1, b);
            } else {
                char buf[128];
                sprintf(buf, "bad command: %s", line);
                Serial.println(buf);
            }
        }
    }
}

void interruptName() { g->encoderTick(); }
