
#include <RCore.h>
#include <Servo.h>

HardwareSerial* debugSerial;

void processBuffer(Buffer* b);

int numMotors = 4;
Motor** motors;
Buffer* buf1;
Buffer* buf2;

Servo pan;
Servo tilt;

void setup() {
    Serial.begin(9600);
    debugSerial = &Serial;

    testParseCommand();

    motors = new Motor*[numMotors];
    //                   in1  in2 pwm
    motors[0] = new Motor(51, 50, 5);
    motors[1] = new Motor(52, 53, 4);
    motors[2] = new Motor(24, 25, 6);
    motors[3] = new Motor(23, 22, 7);

    pan.attach(10);
    tilt.attach(11);

    buf1 = new Buffer(&Serial);
    buf2 = new Buffer(&Serial1);

    setupInterrupt(3, interrupt0, RISING);
    setupInterrupt(2, interrupt1, RISING);
    setupInterrupt(20, interrupt2, RISING);
    setupInterrupt(21, interrupt3, RISING);

    debugSerial->println("setup done");
}

void processBuffer(Buffer* b) {
    if (b->readTillNewLine()) {
        const char* line = b->getLineAndReset();

        if (line[0] == 'p') {
            int deg = atoi(line + 1);
            pan.write(deg);
        } else if (line[0] == 't') {
            int deg = atoi(line + 1);
            tilt.write(deg);
        } else if (line[0] == '?') {
            char buf[numMotors + 2];
            buf[0] = '#';

            for (int i = 0; i < numMotors; i++) {
                buf[i + 1] = '0' + motors[i]->moving();
            }
            buf[numMotors + 1] = 0;
            b->println(buf);

        } else {
            int motorNumber = line[0] - '0';
            if (motorNumber < 0 || motorNumber >= numMotors) {
                debugSerial->println("motor number invalid");
                debugSerial->println(motorNumber, DEC);
            }

            motors[motorNumber]->doCommand(line + 1);
        }
    }
}

int temp = 0;

void loop() {
    bool stopped = false;
    for (int i = 0; i < numMotors; i++) {
        stopped = stopped || motors[i]->checkEncoder();
    }

    if (stopped) {
        for (int i = 0; i < numMotors; i++) {
            motors[i]->stop();
        }
    }

    processBuffer(buf1);
    processBuffer(buf2);
}

void interruptCallback(int num) {
    // debugSerial->println(num);
    motors[num]->encoderTick();
}

void interrupt0() { interruptCallback(0); }

void interrupt1() { interruptCallback(1); }

void interrupt2() { interruptCallback(2); }

void interrupt3() { interruptCallback(3); }
