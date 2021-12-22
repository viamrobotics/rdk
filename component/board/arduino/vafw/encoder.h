// encoder.h

#pragma once

typedef long EncoderCount;

class HallEncoder {
   public:
    HallEncoder();

    void encoderTick(bool a);
    void zero(long offset);

    EncoderCount position() const { return _position; }

    void setA(bool high) { _a = high; }
    void setB(bool high) { _b = high; }

   private:
    bool _a;
    bool _b;

    EncoderCount _position;
};
