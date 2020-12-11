

class Rover {
public:
    Rover(HardwareSerial* front, HardwareSerial* back)
        : _front(front), _back(back) {
    }

    void init() {
        _front->begin(115200);
        _back->begin(115200);


        Serial.println("Giving the Roboteq some time to boot-up.");
        for (int i=0; i < 10; i++ ){
            Serial.println(".");
            delay(250);
        }
    }


    void allSameDirection(int value) {
        go(0, value);
        go(1, -1 * value);
        go(2, -1 * value);
        go(3, value);
    }

    void spin(int value) {
        go(0, value);
        go(1, value);
        go(2, value);
        go(3, value);
    }

    void shift(int value) {
        go(0, value);
        go(1, value);
        go(2, -1 * value);
        go(3, -1 * value);
    }
    
    void pipe(char* notes) {
        Serial.println(notes);
        _pipe(_front);
        _pipe(_back);
        Serial.println("----");
    }

    void query(char* q) {
        _front->println(q);
        _back->println(q);
        pipe(q);
    }
    
private:

    void _pipe(HardwareSerial* s) {
        while (s->available()) {
            int inByte = s->read();
            Serial.write(inByte);
        }
    }
    
    // motor is 0 -> 3
    void go(int motor, int value) {

        // Command Structure
        //  !G [nn] mm
        //  | - "!G" is the "Go" commnand
        //      | - nn = Motor Channel
        //          | - mm = Motor Power (-1000 to 1000)
        //  NOTE: Each command is completed by a carriage
        //        return. CR = dec 12, hex 0x0D
        
        HardwareSerial* s = motor >= 2 ? _front : _back;
        s->print("!G ");
        s->print(1 + (motor%2));
        s->print(" ");
        s->println(value);

        if (false) {
            Serial.print("!G ");
            Serial.print(1 + (motor%2));
            Serial.print(" ");
            Serial.println(value);
        }

    }

    HardwareSerial* _front;
    HardwareSerial* _back;
};

Rover* theRover;

void setup() {
    pinMode(LED_BUILTIN, OUTPUT);
    Serial.begin(9600);  
    Serial.println("hi3");

    theRover = new Rover(&Serial1, &Serial2);
    theRover->init();
}

void loop() {

    digitalWrite(LED_BUILTIN, HIGH);   // turn the LED on (HIGH is the voltage level)
    play();
    theRover->query("?T");

    digitalWrite(LED_BUILTIN, LOW);    // turn the LED off by making the voltage LOW
    delay(1000);                       // wait for a second
}

void play() {

    // Fowrard
    for (int x = 200; x < 400; x=x+10) {
        theRover->allSameDirection(x);
        delay(100);    
    }
    theRover->pipe("done Forward");

    // Ramp Backward
    for (int x = 200; x < 400; x=x+10) {
        theRover->allSameDirection(-1 * x);
        delay(100);    
    }
    theRover->pipe("done Backward");

    // Shift right
    for (int x = 200; x < 400; x=x+10) {
        theRover->shift(x);
        delay(100);    
    }
    theRover->pipe("shift right");

    // Shift left
    for (int x = 200; x < 400; x=x+10) {
        theRover->shift(-1 * x);
        delay(100);    
    }
    theRover->pipe("shift left");


    // spin right
    for (int x = 200; x < 400; x=x+10) {
        theRover->spin(x);
        delay(100);    
    }
    theRover->pipe("spin right");

    // spin left
    for (int x = 200; x < 400; x=x+10) {
        theRover->spin(-1 * x);
        delay(100);    
    }
    theRover->pipe("sping left");

  
}
