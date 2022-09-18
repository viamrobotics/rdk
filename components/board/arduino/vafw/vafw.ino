
#include "buffer.h"
#include "general.h"
#include "motor.h"
#include "pwm.h"

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

PWM *pwm;

Buffer* buf1 = 0;
#if defined(__AVR_ATmega1280__) || defined(__AVR_ATmega2560__)
Buffer* buf2 = 0;
#endif

void(* resetBoard) (void) = 0; //declare reset function @ address 0

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

void configureMotorDC(Buffer* b, const char* name, PinNumber pwm, PinNumber pinA,
                      PinNumber pinB, PinNumber pinDir, PinNumber pinEn, PinNumber encA, PinNumber encB) {
    int motor = findEmptyMotor();
    if (motor < 0) {
        b->println("#not enough motor slots");
        return;
    }

    motors[motor].motor = new Motor(name, pinA, pinB, pinDir, pinEn, pwm);
    motors[motor].encA = encA;
    motors[motor].encB = encB;

    motors[motor].motor->setIncrementalEncoder(new IncrementalEncoder(encA, encB));

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
	pwm = new PWM();
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
        // Reset board as we're about to be reconfigured.
        resetBoard();
    }

    if (isCommand(line, "echo")) {
        char res[255];
        sprintf(res, "@%s", line + 5);
        b->println(res);
        return;
    }

    if (isCommand(line, "config-motor-dc")) {
        char name[255];
        int pwm, pinA, pinB, pinDir, pinEn, encA, encB;
        int n = sscanf(line, "config-motor-dc %s %d %d %d %d %d e %d %d", name, &pwm,
                       &pinA, &pinB, &pinDir, &pinEn, &encA, &encB);
        if (n != 8) {
            b->println("");
            b->print(n);
            b->println("");
            b->println("#error parsing config-motor-dc");
            return;
        }

        configureMotorDC(b, name, pwm, pinA, pinB, pinDir, pinEn, encA, encB);

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

    if (const char* rest = isCommand(line, "motor-zero")) {
        char name[255];
        long offset;
        int n = sscanf(rest, "%s %ld", name, &offset);
        if (n != 2) {
            b->print(n);
            b->println("");
            b->println("#error parsing motor-zero");
            return;
        }

        Motor* m = findMotor(name);
        if (!m) {
            b->println(name);
            b->println("#couldn't find motor");
            return;
        }
        m->encoder()->zero(offset);
        b->println("@ok");
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

    if (const char* rest = isCommand(line, "motor-goto")) {
        char name[255];
        long numTicks, ticksPerSecond;
        int n = sscanf(rest, "%s %ld %ld", name, &numTicks, &ticksPerSecond);
        if (n != 3) {
            b->print(n);
            b->println("");
            b->println("#error parsing motor-goto");
            return;
        }

        Motor* m = findMotor(name);
        if (!m) {
            b->println(name);
            b->println("#couldn't find motor");
            return;
        }
        m->goTo(ticksPerSecond, numTicks);
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

    if (const char* name = isCommand(line, "analog-read")) {
        int val = myAnalogRead(name);
        if (val < 0) {
            b->println("#couldn't find analog reader");
            return;
        }
        b->print("@");
        b->print(val);
        b->println("");
        return;
    }
    if (const char* rest = isCommand(line, "set-pwm-freq")) {
        uint32_t pin,freq;
        int n = sscanf(rest,"%lu %lu",&pin,&freq);
        if (n != 2) {
          b->println("");
          b->print(n);
          b->println("");
          b->println("#error parsing set-pwm-freq");
          return;
        }
        if(!pwm->setPinFrequency(pin,freq)){
            b->println("#couldn't set pwm freq for pin");
            return;
        }
        b->println("@ok");
        return;
    }
    if (const char* rest = isCommand(line, "set-pwm-duty")) {
        uint16_t pin,duty;
        int n = sscanf(rest,"%u %u",&pin,&duty);
          if (n != 2) {
            b->print(n);
            b->println("");
            b->println("#error parsing set-pwm-duty");
            return;
          }
        pwm->analogWrite(pin,duty);
        b->println("@ok");
        return;
    }

    b->println(line);
    b->println("#unknown command");
}

void loop() {
    processBuffer(buf1);
#if defined(__AVR_ATmega1280__) || defined(__AVR_ATmega2560__)
    processBuffer(buf2);
#endif

    for (int i = 0; i < MAX_MOTORS; i++) {
        if (motors[i].motor) {
            motors[i].motor->checkEncoder(millis());
        }
    }
}

#if defined(FALLING)
void setupInterruptBasic(PinNumber pin, void (*ISR)(), int what) {
#else
// Needed for new arduino API where PinStatus is an enum, instead of a macro
void setupInterruptBasic(PinNumber pin, void (*ISR)(), PinStatus what) {
#endif
    pinMode(pin, INPUT_PULLUP);
    attachInterrupt(digitalPinToInterrupt(pin), ISR, what);
}

void motorEncoder(PinNumber pin) {
    for (int i = 0; i < MAX_MOTORS; i++) {
        if (motors[i].encA == pin || motors[i].encB == pin) {
            motors[i].motor->encoder()->encoderTick();
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
    pinMode(pin, INPUT_PULLUP);
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

int myAnalogRead(const char* name) {
    if (name[0] == 'A') {
        name++;
    }

    auto n = atoi(name);

    switch (n) {
        case 0:
            return analogRead(A0);
        case 1:
            return analogRead(A1);
        case 2:
            return analogRead(A2);
        case 3:
            return analogRead(A3);
        case 4:
            return analogRead(A4);
        case 5:
            return analogRead(A5);
        #if defined(A6)
        case 6:
            return analogRead(A6);
        #endif
        #if defined(A7)
        case 7:
            return analogRead(A7);
        #endif
        #if defined(A8)
        case 8:
            return analogRead(A8);
        case 9:
            return analogRead(A9);
        case 10:
            return analogRead(A10);
        case 11:
            return analogRead(A11);
        #endif
        #if defined(A12)
        case 12:
            return analogRead(A12);
        case 13:
            return analogRead(A13);
        case 14:
            return analogRead(A14);
        #endif
        #if defined(A15)
        case 15:
            return analogRead(A15);
        #endif
    }

    return -1;
}
