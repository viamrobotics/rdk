#include <Wire.h>
#include <LSM303.h>

LSM303 compass;

void setup() {
  Serial.begin(9600);
  Wire.begin();
  compass.init();
  compass.enableDefault();
  compass.m_min = (LSM303::vector<int16_t>){-29, -703, +10};
  compass.m_max = (LSM303::vector<int16_t>){386, -364, 85};
}

void loop() {
  compass.read();
  float heading = compass.heading();
  Serial.println(heading);
  delay(100);
}
