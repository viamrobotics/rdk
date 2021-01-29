
#include <RCore.h>

HardwareSerial* debugSerial;

class ServoInput {
   public:
    // TODO(erh): auto configure these?
    ServoInput(unsigned long min = 1100, unsigned long long max = 1900)
        : _min(min), _max(max) {
        _lastInterrupt = 0;
        _span = double(_max - _min);
    }

    void changed() {
        auto now = micros();
        auto diff = now - _lastInterrupt;
        _lastInterrupt = now;

        if (diff > 10000) {
            // this is the gap between signals, ignore
            return;
        }

        _lastDiff = diff;
    }

    double val() const {
        auto a = double(_lastDiff - _min);
        auto d = a / _span;
        if (d < 0) {
            d = 0;
        } else if (d > 1.0) {
            d = 1;
        }
        return d;
    }
    unsigned long lastDiff() const { return _lastDiff; }

   private:
    unsigned long long _min;
    unsigned long long _max;
    double _span;

    unsigned long _lastInterrupt;
    unsigned long _lastDiff;
};

ServoInput inputs[6];

void setup() {
    Serial.begin(9600);
    debugSerial = &Serial;

    Serial.println("setup starting");

    setupInterrupt(22, i1, CHANGE);
    setupInterrupt(24, i2, CHANGE);
    setupInterrupt(26, i3, CHANGE);
    setupInterrupt(28, i4, CHANGE);
    setupInterrupt(30, i5, CHANGE);
    setupInterrupt(32, i6, CHANGE);

    Serial.println("setup done");
}

void loop() {
    delay(1000);
    char buf[128];
    /*
    sprintf(buf, "l - %d %d %d %d %d %d",
            inputs[0].lastDiff(),
            inputs[1].lastDiff(),
            inputs[2].lastDiff(),
            inputs[3].lastDiff(),
            inputs[4].lastDiff(),
            inputs[5].lastDiff());
    Serial.println(buf);
    */
    sprintf(buf, "l - %0.2f %0.2f %0.2f %0.2f %0.2f %0.2f", inputs[0].val(),
            inputs[1].val(), inputs[2].val(), inputs[3].val(), inputs[4].val(),
            inputs[5].val());
    Serial.println(buf);
}

void gotInterrupt(int n) { inputs[n - 1].changed(); }

void i1() { gotInterrupt(1); }
void i2() { gotInterrupt(2); }
void i3() { gotInterrupt(3); }
void i4() { gotInterrupt(4); }
void i5() { gotInterrupt(5); }
void i6() { gotInterrupt(6); }
