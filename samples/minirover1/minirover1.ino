

class Rover {
public:
    Rover(HardwareSerial* front, HardwareSerial* back)
        : _front(front), _back(back) {
    }

    void init() {
        _front->begin(115200);
        _back->begin(115200);

        // TODO: is there a way to make sure motor controllers are ready?

        // go into torque mode
        query("^MMOD 1 5"); 
        query("^MMOD 2 5");

        // check mode
        query("~MMOD 1");
        query("~MMOD 2");

        // set max power - motors are rated for .1 amps
        // 10 is the minimum
        // so we're just playing in the 0-100 camp below
        query("^ALIM 1 10");
        query("^ALIM 2 10");

        query("~ALIM 1");
        query("~ALIM 2");
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
        Serial.println(q);
        _front->println(q);
        _back->println(q);
        delay(100);
        _pipe(_front);
        _pipe(_back);
        Serial.println("----");
    }
    
    //private:

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
        
        HardwareSerial* s = motor < 2 ? _front : _back;
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

class CommandState {
public:
    CommandState() {
        c = -1;
        p = -1;
    }

    void gotData(int b) {
        if ( b < 0 ) {
            Serial.println("got negative in gotData, bad:(");
            return;
        }

        if (c == -1) {
            c = b;
            return;
        }

        p = 10 * (b - '0');
        _run(c, p );

        c = -1;
        p = -1;
    }

    static void _run(int command, int p) {
        switch (command) {
        case 'w':
            theRover->allSameDirection(p);
            delay(100);
            break;
        case 's':
            theRover->allSameDirection(-p);
            delay(100);
            break;

        case 'a':
            theRover->shift(-p);
            delay(100);
            break;
        case 'd':
            theRover->shift(p);
            delay(100);
            break;

        case ',':
            theRover->spin(-p);
            delay(100);
            break;

        case '.':
            theRover->spin(p);
            delay(100);
            break;

            
        case '0':
            theRover->go(0, p);
            break;
        case '1':
            theRover->go(1, p);
            break;
        case '2':
            theRover->go(2, p);
            break;
        case '3':
            theRover->go(3, p);
            break;
        }
    }
    
    int c; // command
    int p; // power
    
} theCommandState;

void setup() {
    pinMode(LED_BUILTIN, OUTPUT);
    Serial.begin(9600);  
    Serial.println("hi3");

    theRover = new Rover(&Serial1, &Serial2);
    theRover->init();
}

void loop() {

    digitalWrite(LED_BUILTIN, HIGH);   // turn the LED on (HIGH is the voltage level)

    //play();
    //theRover->query("?T");

    while (Serial.available()) {
        theCommandState.gotData(Serial.read());
    }        
    
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
