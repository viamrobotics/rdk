
#include "buffer.h"
#include "motor.h"

#define MAX_MOTORS 12

struct motorInfo {
    motorInfo() {
        motor = 0;
        encA = 0;
        encB = 0;
    }
    Motor* motor;
    int encA, encB;
};

motorInfo motors[MAX_MOTORS];

Buffer* buf1;
Buffer* buf2;

int findEmptyMotor() {
    for (int i=0; i<MAX_MOTORS; i++) {
        if (!motors[i].motor) {
            return i;
        }
    }
    return -1;
}

Motor* findMotor(const char* name) {
    for (int i=0; i<MAX_MOTORS; i++) {
        if (motors[i].motor && strcmp(name, motors[i].motor->name()) == 0) {
            return motors[i].motor;
        }
    }
    return 0;
}

void configureMotorDC(Buffer* b, const char* name, int pwm, int pinA, int pinB, int encA, int encB) {
    int motor = findEmptyMotor();
    if (motor < 0) {
        b->println("#not enough motor slots");
        return;
    }

    if (!setupInterruptForMotor(encA) || !setupInterruptForMotor(encB)) {
        b->println("#encoder setup fail");
        return;
    }
    
    motors[motor].motor = new Motor(name, pinA, pinB, pwm, true);
    motors[motor].encA = encA;
    motors[motor].encB = encB;
}

void setup() {
    Serial.begin(9600);
    
    buf1 = new Buffer(&Serial);
    buf2 = new Buffer(&Serial1);

    buf1->println("!");
    buf2->println("!");
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
        sprintf(res, "@%s", line+5);
        b->println(res);
        return;
    }

    if (isCommand(line, "config-motor-dc")) {
        char name[255];
        int pwm, pinA, pinB, encA, encB;
        int n = sscanf(line, "config-motor-dc %s %d %d %d e %d %d", name, &pwm, &pinA, &pinB, &encA, &encB);
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
        b->print(m->encoderTicks());
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
        int numTicks, ticksPerSecond;
        int n = sscanf(rest, "%s %d %d", name, &numTicks, &ticksPerSecond);
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

        m->forward(128, numTicks);
        b->println("@ok");
        return;
    }

    b->println(line);
    b->println("#unknown command");
}

void loop() {
    processBuffer(buf1);
    processBuffer(buf2);
    for (int i=0; i<MAX_MOTORS; i++) {
        if (motors[i].motor) {
            motors[i].motor->checkEncoder();
        }
    }
}

void setupInterruptBasic(int pin, void (*ISR)(), int what) {
    pinMode(pin, INPUT);
    digitalWrite(pin, HIGH); // enable internal pullup resistor
    attachInterrupt(digitalPinToInterrupt(pin), ISR, what);
}

void motorEncoder(int pin, bool rising) {
    for (int i=0; i<MAX_MOTORS; i++) {
        if (motors[i].encA == pin) {
            motors[i].motor->encoderTick(true, rising);
            return;
        }
        if (motors[i].encB == pin) {
            motors[i].motor->encoderTick(false, rising);
            return;
        }
    }
}

void motorInt2Fall() {motorEncoder(2, false);}
void motorInt2Rising() {motorEncoder(2, true);}
void motorInt3Fall() {motorEncoder(3, false);}
void motorInt3Rising() {motorEncoder(3, true);}
void motorInt18Fall() {motorEncoder(18, false);}
void motorInt18Rising() {motorEncoder(18, true);}
void motorInt19Fall() {motorEncoder(19, false);}
void motorInt19Rising() {motorEncoder(19, true);}
void motorInt20Fall() {motorEncoder(20, false);}
void motorInt20Rising() {motorEncoder(20, true);}
void motorInt21Fall() {motorEncoder(21, false);}
void motorInt21Rising() {motorEncoder(21, true);}

bool setupInterruptForMotor(int pin){
    switch(pin) {
    case 2:
        setupInterruptBasic(pin, motorInt2Fall, FALLING);
        setupInterruptBasic(pin, motorInt2Rising, RISING);
        return true;
    case 3:
        setupInterruptBasic(pin, motorInt3Fall, FALLING);
        setupInterruptBasic(pin, motorInt3Rising, RISING);
        return true;
    case 18:
        setupInterruptBasic(pin, motorInt18Fall, FALLING);
        setupInterruptBasic(pin, motorInt18Rising, RISING);
        return true;
    case 19:
        setupInterruptBasic(pin, motorInt19Fall, FALLING);
        setupInterruptBasic(pin, motorInt19Rising, RISING);
        return true;
    case 20:
        setupInterruptBasic(pin, motorInt20Fall, FALLING);
        setupInterruptBasic(pin, motorInt20Rising, RISING);
        return true;
    case 21:
        setupInterruptBasic(pin, motorInt21Fall, FALLING);
        setupInterruptBasic(pin, motorInt21Rising, RISING);
        return true;

    }
    return false;
}
