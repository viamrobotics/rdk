
#include <RCore.h>

HardwareSerial* debugSerial;

Motor* m;
double count = 0;

void setup() {
    Serial.begin(9600);

    debugSerial = &Serial;
    m = new Motor(6, 7, 9);

    count = 0;
    pinMode(2, INPUT);
    digitalWrite(2, HIGH);  // enable internal pullup resistor
    attachInterrupt(digitalPinToInterrupt(2), interruptName,
                    RISING);  // Interrupt initialization
}

double prev = 0;
int moving = 0;
int dir = 1;
unsigned long lastTime = 0;
unsigned long lastCount = 0;
const double maxDiff = 500;
void loop() {
    auto diff = count - prev;

    if (diff < maxDiff) {
        if (millis() - 500 > lastTime) {
            if (count == lastCount) {
                Serial.println("stopped moving");
                Serial.println(diff);
                diff = maxDiff + 1;
            } else {
                lastTime = millis();
                lastCount = count;
            }
        }
    }

    if (diff > maxDiff) {
        m->stop();
        prev = count;
        moving = 0;
        Serial.println("stop");
        delay(1000);
    }

    if (!moving) {
        if (dir == 0) {
            m->forward(255);
            Serial.println("forward");
        } else {
            m->backward(255);
            Serial.println("backward");
        }
        dir = !dir;
        moving = 1;
    }
}

void interruptName() { count = count + 1; }
