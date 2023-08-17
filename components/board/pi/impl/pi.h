//go:build !no_pigpio
#pragma once

// interruptCallback calls through to the go linked interrupt callback.
int setupInterrupt(int gpio);
int teardownInterrupt(int gpio);
