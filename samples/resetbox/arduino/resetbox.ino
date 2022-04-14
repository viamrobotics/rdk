// This file is the original non-RDK Arduino-based control for the resetbox.
// It will be obsoleted once porting is finished.
// This is the older prototype resetbox.
// In this, the arm controlled by a Pi and arduino-resetbox.go
// The arduino board (this file) controls the other motors and functions
// Communication between the two is via two GPIO lines.


#include <AccelStepper.h>
#include <Servo.h>
#include <TMCStepper.h>
#include <protothreads.h>



#define GATE_OFFSET 35 //Height from bed to "zero" in mm
#define SQUEEZE_WIDTH 115 //Width of squeeze gap when at full open (zeroed) in mm

#define HAMMER_UP 130
#define HAMMER_DOWN 30

#define SHAKE_STRENGTH 90

#define ELEVATOR_DOWN 58
#define ELEVATOR_UP 800


#define SERIAL_PORT Serial1 // TMC2208/TMC2224 HardwareSerial port
#define DRIVER_ADDRESS 0b00 // TMC2209 Driver address according to MS1 and MS2
#define R_SENSE 0.11f

// Select your stepper driver type
TMC2209Stepper driver(&SERIAL_PORT, R_SENSE, DRIVER_ADDRESS);
//TMC2209Stepper driver(SW_RX, SW_TX, R_SENSE, DRIVER_ADDRESS);


// Stepper motors, DRIVER is because we use seperate chips
// Second param is the stepPin, and 3rd is the dirPin
AccelStepper gateL(AccelStepper::DRIVER, 22, 23);
AccelStepper gateR(AccelStepper::DRIVER, 24, 25);
AccelStepper squeezeL(AccelStepper::DRIVER, 26, 27);
AccelStepper squeezeR(AccelStepper::DRIVER, 28, 29);
AccelStepper elevator(AccelStepper::DRIVER, 34, 35);

#define MICROSTEPS 8

Servo duckHammer;

//PWM pins for DC motors
const int vibePin1 = 5;
const int vibePin2 = 6;
const int vibePin3 = 7;

//Duckhammer
const int servoPin1 = 9;

//Relay controls
const int relayPin1 = 40;
const int relayPin2 = 41;
const int relayPin3 = 42;
const int relayPin4 = 43;

//Diag (limit switch) pins
const int gateSwitchL = 30;
const int gateSwitchR = 31;
const int squeezeSwitchL = 32;
const int squeezeSwitchR = 33;
const int elevatorSwitch = 36;

//Signal Pins to Arm
const int triggerPin = 45;
const int readyPin = 46;

//Button
const int buttonPin1 = 2;
const int buttonPin2 = 3;

int const potPin = A0; // analog pin used to connect the potentiometer
int potVal;  // variable to read the value from the analog pin
int angle;   // variable to hold the angle for the servo motor

int linearDir1 = -1;
int linearDir2 = -1;
long button1Time = 0;
long button2Time = 0;

void toggleLinear1() {
  button1Time = millis();
}

void toggleLinear2() {
  button2Time = millis();
}



//Threads

pt mainPT;

pt elevatorPT;

pt tipPT;
bool tip = 0;

pt whackPT;
int whackCount = 0;
int whackPause = 500;



void setup() {

  duckHammer.attach(servoPin1);
  
  pinMode(13, OUTPUT);
  pinMode(vibePin1, OUTPUT);
  pinMode(vibePin2, OUTPUT);
  pinMode(vibePin3, OUTPUT);

  digitalWrite(vibePin1, LOW);
  digitalWrite(vibePin2, LOW);
  digitalWrite(vibePin3, LOW);

  pinMode(relayPin1, OUTPUT);
  pinMode(relayPin2, OUTPUT);
  pinMode(relayPin3, OUTPUT);
  pinMode(relayPin4, OUTPUT);

  digitalWrite(relayPin1, HIGH);
  digitalWrite(relayPin2, HIGH);
  digitalWrite(relayPin3, HIGH);
  digitalWrite(relayPin4, HIGH);

  pinMode(gateSwitchL, INPUT);
  pinMode(gateSwitchR, INPUT);
  pinMode(squeezeSwitchL, INPUT);
  pinMode(squeezeSwitchR, INPUT);
  pinMode(elevatorSwitch, INPUT);

  pinMode(buttonPin1, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(buttonPin1), toggleLinear1, FALLING);

  pinMode(buttonPin2, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(buttonPin2), toggleLinear2, FALLING);

  pinMode(triggerPin, INPUT);
  pinMode(readyPin, OUTPUT);

  digitalWrite(readyPin, LOW);

  SERIAL_PORT.begin(115200);
  driver.beginSerial(115200);
  driver.begin();
  driver.toff(4);
  driver.blank_time(24);
  driver.rms_current(1500); // mA
  driver.microsteps(MICROSTEPS);
  driver.TCOOLTHRS(0xFFFFF); // 20bit max
//  driver.semin(5);
//  driver.semin(5);
//  driver.semax(2);
//  driver.sedn(0b01);
  driver.SGTHRS(100);

  gateL.setMaxSpeed(800 * MICROSTEPS);
  gateL.setAcceleration(400 * MICROSTEPS);

  gateR.setMaxSpeed(800 * MICROSTEPS);
  gateR.setAcceleration(400 * MICROSTEPS);

  squeezeL.setMaxSpeed(800 * MICROSTEPS);
  squeezeL.setAcceleration(400 * MICROSTEPS);

  squeezeR.setMaxSpeed(800 * MICROSTEPS);
  squeezeR.setAcceleration(400 * MICROSTEPS);

  elevator.setMaxSpeed(3200 * MICROSTEPS);
  elevator.setAcceleration(1600 * MICROSTEPS);

  duckHammer.write(HAMMER_UP);
  homeGate();
  homeSqueeze();
  homeElevator();

  PT_INIT(&mainPT);
  PT_INIT(&elevatorPT);
  PT_INIT(&tipPT);
  PT_INIT(&whackPT);


}


int mmToSteps(float mm) {
  // mm * (fullsteps * microsteps) / leadscrew pitch
  return mm * ((200 * MICROSTEPS) / 8);
}

int mmToStepsElevator(float mm) {
  // mm * (fullsteps * microsteps) / (belt pitch) * pully teeth)
  return mm * ((200 * MICROSTEPS) / (2 * 30));
}

void homeElevator() {
  unsigned long min_time;
  elevator.setSpeed(800 * MICROSTEPS);

  min_time = millis() + 1000;
  while ( millis() < min_time ) {
    elevator.runSpeed();
  }
  
  elevator.setSpeed(-800 * MICROSTEPS);

  min_time = millis() + 500;
  while ( !digitalRead(elevatorSwitch) || millis() < min_time ) {
    elevator.runSpeed();
  }
  elevator.setCurrentPosition(0);
}


void homeGate() {
  unsigned long min_time;
  gateL.setSpeed(400 * MICROSTEPS);
  gateR.setSpeed(400 * MICROSTEPS);

  min_time = millis() + 1000;
  while ( millis() < min_time ) {
    gateL.runSpeed();
    gateR.runSpeed();
  }
  
  gateL.setSpeed(-400 * MICROSTEPS);
  gateR.setSpeed(-400 * MICROSTEPS);

  min_time = millis() + 500;
  while ( !digitalRead(gateSwitchL) || !digitalRead(gateSwitchR) || millis() < min_time ) {
    if (!digitalRead(gateSwitchL) || millis() < min_time ) { gateL.runSpeed(); }
    if (!digitalRead(gateSwitchR) || millis() < min_time ) { gateR.runSpeed(); }
  }
  gateL.setCurrentPosition(0);
  gateR.setCurrentPosition(0);
}


void homeSqueeze() {
  unsigned long min_time;
  squeezeL.setSpeed(400 * MICROSTEPS);
  squeezeR.setSpeed(400 * MICROSTEPS);

  min_time = millis() + 1000;
  while ( millis() < min_time ) {
    squeezeL.runSpeed();
    squeezeR.runSpeed();
  }
  
  squeezeL.setSpeed(-400 * MICROSTEPS);
  squeezeR.setSpeed(-400 * MICROSTEPS);

  min_time = millis() + 500;

  
  while ( !digitalRead(squeezeSwitchL) || !digitalRead(squeezeSwitchR) || millis() < min_time ) {
    if (!digitalRead(squeezeSwitchL) || millis() < min_time ) { squeezeL.runSpeed(); }
    if (!digitalRead(squeezeSwitchR) || millis() < min_time ) { squeezeR.runSpeed(); }
  }
  squeezeL.setCurrentPosition(0);
  squeezeR.setCurrentPosition(0);
}

void setElevator(float height) {
  int pos = mmToStepsElevator(height);
  if (pos < 0) { pos = 0; }
  elevator.moveTo(pos);
}


void setGate(float height) {
  int pos = mmToSteps(height - GATE_OFFSET);
  if (pos < 0) { pos = 0; }
  gateL.moveTo(pos);
  gateR.moveTo(pos);
  while (gateL.distanceToGo() != 0 || gateR.distanceToGo() != 0) {
    gateL.run();
    gateR.run();
  }
}

void setSqueeze(float width) {
  int pos = mmToSteps((SQUEEZE_WIDTH - width)/2);
  if (pos < 0) { pos = 0; }
  squeezeL.moveTo(pos);
  squeezeR.moveTo(pos);
  while (squeezeL.distanceToGo() != 0 || squeezeR.distanceToGo() != 0) {
    squeezeL.run();
    squeezeR.run();
  }
}

void shakeTable(int level) {
  if (level < 32) {
    digitalWrite(vibePin1, LOW);
  }else{
    analogWrite(vibePin1, level);
  }
}

void doButtons() {

  if (button1Time > 0 && button1Time < millis() - 100 && digitalRead(buttonPin1) == LOW) {
    button1Time = 0;
    linearDir1++;
    switch (linearDir1){
      case 0:
        digitalWrite(relayPin1, LOW);
        digitalWrite(relayPin2, HIGH);
        break;
      case 1:
        digitalWrite(relayPin1, HIGH);
        digitalWrite(relayPin2, LOW);
        break;
      default:
        digitalWrite(relayPin1, HIGH);
        digitalWrite(relayPin2, HIGH);
        linearDir1 = -1;
        break;
    }    
  }

  if (button2Time > 0 && button2Time < millis() - 100 && digitalRead(buttonPin2) == LOW) {
    button2Time = 0;
    linearDir2++;
    switch (linearDir2){
      case 0:
        digitalWrite(relayPin3, LOW);
        digitalWrite(relayPin4, HIGH);
        break;
      case 1:
        digitalWrite(relayPin3, HIGH);
        digitalWrite(relayPin4, LOW);
        break;
      default:
        digitalWrite(relayPin3, HIGH);
        digitalWrite(relayPin4, HIGH);
        linearDir2 = -1;
        break;
    }    
  }

}


int whackThread(struct pt* pt) {
  PT_BEGIN(pt);

  while(1) {
    PT_WAIT_UNTIL(pt, whackCount > 0);
    PT_SLEEP(pt, whackPause);
    duckHammer.write(HAMMER_DOWN);
    PT_SLEEP(pt, 500);
    duckHammer.write(HAMMER_UP);
    whackCount--;
  }
  PT_END(pt);
}


int elevatorThread(struct pt* pt) {
  PT_BEGIN(pt);
  while(1){
      elevator.run();
      PT_YIELD(pt);
  }
  PT_END(pt);
}


int tipThread(struct pt* pt){
  PT_BEGIN(pt);
  while(1){
  PT_WAIT_UNTIL(pt, tip);
    // Tip!
    digitalWrite(relayPin3, LOW);
    digitalWrite(relayPin4, HIGH);
    PT_SLEEP(pt, 12000);
  
    //Stop
    digitalWrite(relayPin3, HIGH);
  
    //Back down
    digitalWrite(relayPin4, LOW);
    PT_SLEEP(pt, 16000);
  
    //All Off
    digitalWrite(relayPin3, HIGH);
    digitalWrite(relayPin4, HIGH);
    tip = 0;
  }
  PT_END(pt);
}




int mainThread(struct pt* pt){
  PT_BEGIN(pt);

  while(1){
    //Go for everything
    setGate(46);
    setSqueeze(30);
    setElevator(ELEVATOR_DOWN);

    //Open squeeze after elevator is down
    PT_WAIT_UNTIL(pt, elevator.distanceToGo() == 0);
    setSqueeze(50);

    //Strobe and wait for robot to trigger reset
    while(1){
       digitalWrite(readyPin, HIGH);
       PT_SLEEP(pt, 10);
       digitalWrite(readyPin, LOW);
       PT_SLEEP(pt, 10);
       if (digitalRead(triggerPin) == HIGH) {
          break;
       }
    }
    digitalWrite(readyPin, LOW);
    shakeTable(SHAKE_STRENGTH);
    tip = 1;

    //Should be approximately after the tip-up has finished
    PT_SLEEP(pt, 10000);

    //In case cubes are behind ducks
    whackPause = 1000;
    whackCount = 3;
    PT_WAIT_UNTIL(pt, whackCount == 0);
    PT_SLEEP(pt, 4000);

    //Cubes in, going up
    setElevator(ELEVATOR_UP);

    //Right the duck
    whackPause = 500;
    whackCount = 15;

    //Wait for "robot" to grab
    PT_WAIT_UNTIL(pt, elevator.distanceToGo() == 0);
    digitalWrite(readyPin, HIGH);
    //Loop to debounce, as signals are noisy with duckHammer draining power
    while(1){
      if(digitalRead(triggerPin) == HIGH){
         PT_SLEEP(pt, 10);
         if(digitalRead(triggerPin) == HIGH){
          break;
         }
      }
      PT_SLEEP(pt, 50);
    }
    digitalWrite(readyPin, LOW);

    //Back down
    setElevator(ELEVATOR_DOWN);

    //Whacking done, let the duck start sliding
    PT_WAIT_UNTIL(pt, whackCount == 0);
    setGate(100);


    //Open squeeze after elevator is down
    PT_WAIT_UNTIL(pt, elevator.distanceToGo() == 0);
    setSqueeze(80);
    PT_SLEEP(pt, 4000);

    //STFU!
    shakeTable(0);

    //Send the duck
    setElevator(ELEVATOR_UP);   

    //Wait for "robot" to grab
    PT_WAIT_UNTIL(pt, elevator.distanceToGo() == 0);
    digitalWrite(readyPin, HIGH);
    PT_WAIT_UNTIL(pt, digitalRead(triggerPin) == HIGH);
    digitalWrite(readyPin, LOW);
  }

  PT_END(pt);
}

void loop() {
  elevatorThread(&elevatorPT);
  tipThread(&tipPT);
  whackThread(&whackPT);
  mainThread(&mainPT);
}
