
#include <RCore.h>

HardwareSerial* debugSerial;

void logMsg(const char* msg) {
    char buf[128];
    sprintf(buf, "@%s", msg);
    Serial.println(buf);
}

void debug(const char* name, int x) {
    char buf[128];
    sprintf(buf, "@%s: %d", name, x);
    Serial.println(buf);
}

class Gripper {
   public:
    Gripper(Motor* m, int locationSensor, int forceSensor)
        : _motor(m),
          _locationSensor(locationSensor),
          _forceSensor(forceSensor),
          _power(8),
          _moving(false),
          _initialized(false),
          _initializeState(0) {
        _motor->setSlowDown(true);
    }

    bool initialzeReady() {
        if (_initialized) {
            return true;
        }

        if (_initializeState == INIT_STATE_START) {  // haven't started yet
            logMsg("Gripper::initialzeReady opening 1");
            _lastSpot = getRawPos();
            _myts = millis();
            open();
            _initializeState = INIT_STATE_OPEN1;
            return false;
        }

        auto pos = getRawPos();
        // debug("pos", pos);
        if (!same(pos, _lastSpot)) {
            // we're still moving
            _lastSpot = pos;
            _myts = millis();
            return false;
        }

        auto now = millis();
        if (now - _myts < 500) {
            // wait longer
            return false;
        }

        // we've waited long enough

        if (_initializeState == INIT_STATE_OPEN2) {
            logMsg("Gripper::initialzeReady opening done");
            auto diff = abs(pos - _fullyOpen);
            debug("Gripper::_initializeState diff", diff);
            _initialized = true;
            _rawGoal = pos;
            _forceSensorDefault = rawForceSensor();
            return true;
        }

        if (_initializeState == INIT_STATE_OPEN1) {
            logMsg("Gripper::initialzeReady closing");
            _fullyOpen = pos;
            _initializeState = INIT_STATE_CLOSE;
            close();
            _myts = now;
            delay(20);
            return false;
        }

        if (_initializeState == INIT_STATE_CLOSE) {
            logMsg("Gripper::initialzeReady opening 2");
            _fullyClosed = pos;
            debug("_fullyOpen", _fullyOpen);
            debug("_fullyClosed", _fullyClosed);
            if (same(_fullyClosed, _fullyOpen)) {
                logMsg("Gripper::initialzeReady NOT MOVING");
                _initializeState = INIT_STATE_START;
                return false;
            }
            _initializeState = INIT_STATE_OPEN2;
            _myts = now;
            open();
            return false;
        }

        debug("unknown _initializeState", _initializeState);

        return false;
    }

    void setPos(double pos) {
        double delta = abs(_fullyClosed - _fullyOpen);
        double diff = pos * delta;

        if (_fullyOpen < _fullyClosed) {
            setRawPos(_fullyOpen + diff);
        } else {
            setRawPos(_fullyOpen - diff);
        }
    }

    double getPos() const {
        double delta = abs(_fullyClosed - _fullyOpen);
        auto pos = getRawPos();
        if (_fullyOpen < _fullyClosed) {
            return (pos - _fullyOpen) / delta;
        }
        return 1 - ((pos - _fullyClosed) / delta);
    }

    // g0.3 -- go to position .0
    // p  - return current position
    void processCommand(const char* command, Buffer* b) {
        if (command[0] == 'g') {
            double pos = atof(command + 1);
            debug("got g", 100 * pos);
            setPos(pos);
        } else if (command[0] == 'p') {
            auto pos = int(100 * getPos());
            if (pos >= 100) {
                b->println("gp:1.0");
            } else {
                char buf[32];
                sprintf(buf, "gp:.%.2d %d", pos, getRawPos());
                b->println(buf);
            }
        } else {
            b->println("bad command");
        }
    }

    void check() {
        if (!_initialized) {
            return;
        }

        if (_holding) {
            auto force = rawForceSensor();
            if (!same(force, _lastForce, 20)) {
                debug("F1", force);
                debug("F2", _lastForce);
                close(2);
                return;
            }
        }

        if (!_moving) {
            return;
        }

        auto now = getRawPos();
        if (same(now, _rawGoal)) {
            _motor->stop();
            _moving = false;
            debug("s1", now);
            debug("s2", _rawGoal);
            return;
        }

        auto force = rawForceSensor();
        if (_forceSensorDefault - force > 100) {
            debug("force", force);
            _motor->stop();
            _moving = false;
            _holding = true;
            _lastForce = force;
            return;
        }

        setRawPos(_rawGoal);
    }

    // private:
    bool same(int raw1, int raw2, int delta = 10) {
        return abs(raw1 - raw2) < delta;
    }

    void setRawPos(int pos) {
        auto current = getRawPos();
        if (current == pos) {
            return;
        }

        _rawGoal = pos;

        if (_fullyOpen < _fullyClosed) {
            if (pos < current) {
                open();
            } else {
                close();
            }
        } else {
            if (pos < current) {
                close();
            } else {
                open();
            }
        }
    }

    void open() {
        _holding = false;
        _moving = true;
        _opening = true;
        _motor->forward(_power);
    }

    void close(int powerMul = 1) {
        _holding = false;
        _moving = true;
        _opening = false;
        _motor->backward(_power * powerMul);
    }

    int getRawPos() const { return analogRead(_locationSensor); }

    int rawForceSensor() const { return analogRead(_forceSensor); }

    Motor* _motor;
    int _locationSensor;
    int _forceSensor;

    int _power;

    bool _moving;
    bool _opening;  // true is opening, false is closing
    int _rawGoal;

    bool _initialized;
    int _initializeState;
    unsigned long _myts;
    int _lastSpot;  // just used for initialization

    int _fullyOpen;
    int _fullyClosed;

    int _forceSensorDefault;

    bool _holding;
    int _lastForce;

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

    g = new Gripper(new Motor(2, 4, 3, true), 3, 5);
}

void loop() {
    g->check();

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
