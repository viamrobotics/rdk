// general.h

#pragma once

#if defined(__SAM3X8E__) || defined(__SAMD21G18A__)
typedef uint PinNumber;
#else
typedef int PinNumber;
#endif
