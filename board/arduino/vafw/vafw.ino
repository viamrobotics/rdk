
#include "buffer.h"
#include "general.h"
#include "motor.h"

#define MAX_MOTORS 12

struct motorInfo {
    motorInfo() {
        motor = 0;
        encA = 0;
        encB = 0;
    }
    Motor* motor;
    PinNumber encA, encB;
};

motorInfo motors[MAX_MOTORS];

Buffer* buf1 = 0;
Buffer* buf2 = 0;

int findEmptyMotor() {
    for (int i = 0; i < MAX_MOTORS; i++) {
        if (!motors[i].motor) {
            return i;
        }
    }
    return -1;
}

Motor* findMotor(const char* name) {
    for (int i = 0; i < MAX_MOTORS; i++) {
        if (motors[i].motor && strcmp(name, motors[i].motor->name()) == 0) {
            return motors[i].motor;
        }
    }
    return 0;
}

void configureMotorDC(Buffer* b, const char* name, int pwm, PinNumber pinA,
                      PinNumber pinB, PinNumber encA, PinNumber encB) {
    int motor = findEmptyMotor();
    if (motor < 0) {
        b->println("#not enough motor slots");
        return;
    }

    motors[motor].motor = new Motor(name, pinA, pinB, pwm);
    motors[motor].encA = encA;
    motors[motor].encB = encB;

    motors[motor].motor->encoder()->setA(digitalRead(encA));
    motors[motor].motor->encoder()->setB(digitalRead(encB));

    if (!setupInterruptForMotor(encA) || !setupInterruptForMotor(encB)) {
        b->println("#encoder setup fail");
        return;
    }
}

void setup() {
    buf1 = new Buffer(&Serial);
    buf1->println("!");

#if defined(__AVR_ATmega1280__) || defined(__AVR_ATmega2560__)
    buf2 = new Buffer(&Serial3);
    buf2->println("!");
#endif
}

const char* isCommand(const char* line, const char* cmd) {
    int len = strlen(cmd);
    if (strncmp(line, cmd, len) == 0) {
        return line + len + 1;
    }
    return 0;
}

void processBuffer(Buffer* b) {
    if (!b->readTillNewLine()) {
        return;
    }

    const char* line = b->getLineAndReset();
    if (line[0] == '!') {
        b->println("!");
        return;
    }

    if (isCommand(line, "echo")) {
        char res[255];
        sprintf(res, "@%s", line + 5);
        b->println(res);
        return;
    }

    if (isCommand(line, "config-motor-dc")) {
        char name[255];
        int pwm, pinA, pinB, encA, encB;
        int n = sscanf(line, "config-motor-dc %s %d %d %d e %d %d", name, &pwm,
                       &pinA, &pinB, &encA, &encB);
        if (n != 6) {
            b->println("");
            b->print(n);
            b->println("");
            b->println("#error parsing config-motor-dc");
            return;
        }

        configureMotorDC(b, name, pwm, pinA, pinB, encA, encB);

        b->println("@ok");
        return;
    }

    if (const char* name = isCommand(line, "motor-position")) {
        Motor* m = findMotor(name);
        if (!m) {
            b->println(name);
            b->println("#couldn't find motor");
            return;
        }
        b->print("@");
        b->print(m->encoder()->position());
        b->println("");
        return;
    }

    if (const char* name = isCommand(line, "motor-ison")) {
        Motor* m = findMotor(name);
        if (!m) {
            b->println(name);
            b->println("#couldn't find motor");
            return;
        }
        b->println(m->moving() ? "@t" : "@f");
        return;
    }

    if (const char* name = isCommand(line, "motor-off")) {
        Motor* m = findMotor(name);
        if (!m) {
            b->println(name);
            b->println("#couldn't find motor");
            return;
        }
        m->stop();
        b->println("@ok");
        return;
    }

    if (const char* rest = isCommand(line, "motor-gofor")) {
        char name[255];
        long numTicks, ticksPerSecond;
        int n = sscanf(rest, "%s %ld %ld", name, &numTicks, &ticksPerSecond);
        if (n != 3) {
            b->print(n);
            b->println("");
            b->println("#error parsing motor-gofor");
            return;
        }

        Motor* m = findMotor(name);
        if (!m) {
            b->println(name);
            b->println("#couldn't find motor");
            return;
        }
        m->goFor(ticksPerSecond, numTicks);
        b->println("@ok");
        return;
    }

    if (const char* rest = isCommand(line, "motor-go")) {
        char name[255];
        char dir;
        int power;
        int n = sscanf(rest, "%s %c %d", name, &dir, &power);
        if (n != 3) {
            b->print(n);
            b->println("");
            b->println("#error parsing motor-gofor");
            return;
        }

        Motor* m = findMotor(name);
        if (!m) {
            b->println(name);
            b->println("#couldn't find motor");
            return;
        }
        m->go(dir == 'f', power);
        b->println("@ok");
        return;
    }

    if (const char* rest = isCommand(line, "motor-power")) {
        char name[255];
        int power;
        int n = sscanf(rest, "%s %d", name, &power);
        if (n != 2) {
            b->print(n);
            b->println("");
            b->println("#error parsing motor-gofor");
            return;
        }

        Motor* m = findMotor(name);
        if (!m) {
            b->println(name);
            b->println("#couldn't find motor");
            return;
        }
        m->setPower(power);
        b->println("@ok");
        return;
    }

    b->println(line);
    b->println("#unknown command");
}

void loop() {
    processBuffer(buf1);
    if (buf2) {
        processBuffer(buf2);
    }

    for (int i = 0; i < MAX_MOTORS; i++) {
        if (motors[i].motor) {
            motors[i].motor->checkEncoder(millis());
        }
    }
}

void setupInterruptBasic(PinNumber pin, void (*ISR)(), int what) {
    pinMode(pin, INPUT_PULLUP);
    attachInterrupt(digitalPinToInterrupt(pin), ISR, what);
}

void motorEncoder(PinNumber pin) {
    for (int i = 0; i < MAX_MOTORS; i++) {
        if (motors[i].encA == pin) {
            motors[i].motor->encoder()->encoderTick(true);
            return;
        }
        if (motors[i].encB == pin) {
            motors[i].motor->encoder()->encoderTick(false);
            return;
        }
    }
    Serial.println("found no encoder");
}

void motorInt2() { motorEncoder(2); }
void motorInt3() { motorEncoder(3); }
void motorInt18() { motorEncoder(18); }
void motorInt19() { motorEncoder(19); }
void motorInt20() { motorEncoder(20); }
void motorInt21() { motorEncoder(21); }

bool setupInterruptForMotor(PinNumber pin) {
    switch (pin) {
        case 2:
            setupInterruptBasic(pin, motorInt2, CHANGE);
            return true;
        case 3:
            setupInterruptBasic(pin, motorInt3, CHANGE);
            return true;
        case 18:
            setupInterruptBasic(pin, motorInt18, CHANGE);
            return true;
        case 19:
            setupInterruptBasic(pin, motorInt19, CHANGE);
            return true;
        case 20:
            setupInterruptBasic(pin, motorInt20, CHANGE);
            return true;
        case 21:
            setupInterruptBasic(pin, motorInt21, CHANGE);
            return true;
    }
    return false;
}
