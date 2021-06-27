
#include "buffer.h"
#include "core.h"

#define MAX_MOTORS 12
Motor* motors[MAX_MOTORS];

Buffer* buf1;
Buffer* buf2;

int findEmptyMotor() {
    for (int i=0; i<MAX_MOTORS; i++) {
        if (!motors[i]) {
            return i;
        }
    }
    return -1;
}

void configureMotorDC(Buffer* b, const char* name, int pwm, int pinA, int pinB, int encA, int encB) {
    int motor = findEmptyMotor();
    if (motor < 0) {
        b->println("#not enough motor slots");
        return;
    }

    motors[motor] = new Motor(name, pinA, pinB, pwm, true);
}

void setup() {
    Serial.begin(9600);
    
    buf1 = new Buffer(&Serial);
    buf2 = new Buffer(&Serial1);

    for (int i=0; i<MAX_MOTORS; i++) {
        motors[i] = 0;
    }
    
    setupInterrupt(2, hallA, CHANGE);  
    setupInterrupt(3, hallB, CHANGE);

    buf1->println("!");
    buf2->println("!");
}

void hallA() {}
void hallB() {}

bool isCommand(const char* line, const char* cmd) {
    return strncmp(line, cmd, strlen(cmd)) == 0;
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

    
    b->println(line);
    b->println("#unknown command");
}

void loop() {
    processBuffer(buf1);
    processBuffer(buf2);
}
