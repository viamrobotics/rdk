
#include "core.h"

Motor* m;

void setup() {
  Serial.begin(9600);

  m = new Motor(29, 30, 28);

  setupInterrupt(2, hallA, CHANGE);  
  setupInterrupt(3, hallB, CHANGE);

}

void hallA() {
//  Serial.println("a");
}

void hallB() {
 // Serial.println("b");
}

void loop() {
 
  Serial.println("hi2");

  m->forward(255, 0);

  delay(2000);

  m->stop();
  
  delay(1000);        // delay in between reads for stability
}
