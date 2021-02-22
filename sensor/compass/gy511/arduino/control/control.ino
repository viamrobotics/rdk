#include <Wire.h>
#include <EEPROM.h>
#include "LSM303.h"

LSM303 compass;
LSM303::vector<int16_t> calibratedMin = {32767, 32767, 32767}, calibratedMax = {-32768, -32768, -32768};

void setup() {
  Serial.begin(9600);
  Wire.begin();
  compass.init();
  compass.enableDefault();

  calibratedMin.x = EEPROM.read(0);
  calibratedMin.x |= EEPROM.read(1) << 8;
  calibratedMin.y = EEPROM.read(2);
  calibratedMin.y |= EEPROM.read(3) << 8;
  calibratedMin.z = EEPROM.read(4);
  calibratedMin.z |= EEPROM.read(5) << 8;
  calibratedMax.x = EEPROM.read(6);
  calibratedMax.x |= EEPROM.read(7) << 8;
  calibratedMax.y = EEPROM.read(8);
  calibratedMax.y |= EEPROM.read(9) << 8;
  calibratedMax.z = EEPROM.read(10);
  calibratedMax.z |= EEPROM.read(11) << 8;
}

bool calibrating = false;

void loop() {
  if (Serial.available() != 0) {
    const char control = Serial.read();
    switch (control) {
      case '0':
        if (calibrating) {
          compass.m_min = calibratedMin;
          compass.m_max = calibratedMax;
          EEPROM.write(0, char(calibratedMin.x));
          EEPROM.write(1, char(calibratedMin.x >> 8));
          EEPROM.write(2, char(calibratedMin.y));
          EEPROM.write(3, char(calibratedMin.y >> 8));
          EEPROM.write(4, char(calibratedMin.z));
          EEPROM.write(5, char(calibratedMin.z >> 8));
          EEPROM.write(6, char(calibratedMax.x));
          EEPROM.write(7, char(calibratedMax.x >> 8));
          EEPROM.write(8, char(calibratedMax.y));
          EEPROM.write(9, char(calibratedMax.y >> 8));
          EEPROM.write(10, char(calibratedMax.z));
          EEPROM.write(11, char(calibratedMax.z >> 8));
        }
        calibrating = false;
        break;
      case '1':
        calibrating = true;
        calibratedMin = {32767, 32767, 32767}, calibratedMax = {-32768, -32768, -32768};
        break;
    }
  }
  compass.read();
  if (calibrating) {
    calibratedMin.x = min(calibratedMin.x, compass.m.x);
    calibratedMin.y = min(calibratedMin.y, compass.m.y);
    calibratedMin.z = min(calibratedMin.z, compass.m.z);
  
    calibratedMax.x = max(calibratedMax.x, compass.m.x);
    calibratedMax.y = max(calibratedMax.y, compass.m.y);
    calibratedMax.z = max(calibratedMax.z, compass.m.z);
  } else {
    const float heading = compass.heading();
    Serial.print(heading, 5);
    Serial.print('\n');
  }

  delay(10);
}
