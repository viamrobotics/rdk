#pragma once

// interruptCallback calls through to the go linked interrupt callback.
void setupInterrupt(int gpio);
void teardownInterrupt(int gpio);
