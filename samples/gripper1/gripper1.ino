
#include <RCore.h>

HardwareSerial* debugSerial;

class Gripper {
   public:
    Gripper(Motor* m)
        : _motor(m),
          _initialized(false),
          _initializeState(0),
          _fullTicks(1000),
          _power(255) {}

    bool initialzeReady() {
        if (_initialized) {
            return true;
        }

        if (_initializeState == 0) {  // haven't started yet
            Serial.println("Gripper::initialzeReady opening 1");
            open(_fullTicks);
            _initializeState = 1;
            return false;
        }

        auto howLong = millis() - _motor->lastTick();
        if (_motor->moving() && howLong < 300) {
            return false;
        }

        auto encoderTicksStop = _motor->encoderTicksStop();
        _motor->stop();

        auto numTicks =
            _motor->encoderTicks() - (encoderTicksStop - _fullTicks);

        if (_initializeState == 3) {
            _currentSpot = 0;
            _initialized = true;

            if (encoderTicksStop == 0) {
                // perfect
                Serial.println("perfect");
                _initialized = true;
                return true;
            }

            Serial.println("medium");
            Serial.println((double)numTicks);
            Serial.println(_fullTicks);

            return true;
        }

        if (encoderTicksStop == 0) {
            Serial.println("it stopped on it's own, this is unexpected");
            return false;
        }

        if (_initializeState == 1) {  // we opened, now we close
            _initializeState = 2;
            Serial.println("Gripper::initialzeReady closing 1");
            close(_fullTicks);
            return false;
        }

        if (_initializeState == 2) {  // we closed, how many ticks was there
            _fullTicks = numTicks;
            open(_fullTicks);
            _initializeState = 3;
            return false;
        }

        Serial.println("unknown _initializeState");
        Serial.println(_initializeState);

        return false;
    }

    bool moving() const { return _motor->moving(); }

    void checkEncoder() {
        _motor->checkEncoder();
        if (_initialized && moving()) {
            auto howLong = millis() - _motor->lastTick();
            if (howLong > 500) {
                _motor->stop();
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
};

Gripper* g;

void setup() {
    Serial.begin(9600);

    debugSerial = &Serial;
    g = new Gripper(new Motor(6, 7, 9, true));

    pinMode(2, INPUT);
    digitalWrite(2, HIGH);  // enable internal pullup resistor
    attachInterrupt(digitalPinToInterrupt(2), interruptName,
                    RISING);  // Interrupt initialization
}

uint64_t prev = 0;
int moving = 0;
int dir = 1;
unsigned long lastTime = 0;
uint64_t lastCount = 0;
const int maxDiff = 400;

int state = 0;
void loop() {
    g->checkEncoder();

    if (g->initialzeReady()) {
        if (!g->moving()) {
            delay(500);

            Serial.println("state");
            Serial.println(state);
            Serial.println(g->getPos());

            if (state == 0) {
                g->setPos(.5);
                state = 1;
            } else if (state == 1) {
                g->setPos(0);
                state = 2;
            } else if (state == 2) {
                g->setPos(1);
                state = 3;
            } else if (state == 3) {
                g->setPos(.5);
                state = 4;
            } else if (state == 4) {
                g->setPos(1);
                state = 5;
            } else {
                g->setPos(0);
                state = 0;
            }
        }
    }
}

void interruptName() { g->encoderTick(); }
