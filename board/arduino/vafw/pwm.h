/* pwm.h  */
#pragma once

#include <Arduino.h>

#if defined(__AVR_ATmega2560__)
#define BASE_CLK_FREQ 16000000
#elif defined(__AVR_ATmega328P__)
#define BASE_CLK_FREQ 16000000
#else
#define BASE_CLK_FREQ 0
#endif

union TCCRnA{
	struct{
		uint8_t wgmn0:1;
		uint8_t wgmn1:1;
#if defined(__AVR_ATmega2560__)
		uint8_t comn_C0:1;
		uint8_t comn_C1:1;
#elif defined(__AVR_ATmega328P__)
		uint8_t reserved0:1;
		uint8_t reserved1:1;
#endif
		uint8_t comn_B0:1;
		uint8_t comn_B1:1;
		uint8_t comn_A0:1;
		uint8_t comn_A1:1;
	};
	uint8_t reg;
};
#if defined(__AVR_ATmega2560__) || defined(__AVR_ATmega328P__)
union TCCRnB{
	struct{
		uint8_t csn:3;
		uint8_t wgmn2:1;
		uint8_t wgmn3:1;
		uint8_t reserved:1;
		uint8_t ices1:1;
		uint8_t icnc1:1;
	};
	uint8_t reg;
};

enum pwm_mode{
	PWM_FAST_PWM_MODE = 0,
	PWM_PHASE_CORRECT_MODE = 1,
	PWM_PHASE_FREQUENCY_CORRECT_MODE = 2
};
enum pwm_channel{
	PWM_CHANNEL_A = 0,
	PWM_CHANNEL_B = 1,
	PWM_CHANNEL_C = 2
};

class PWMChannel{
public:
	PWMChannel();
	virtual bool setPWMFrequency(uint32_t frequency)=0;
	virtual void setChannelDutyCycle(uint8_t channel, uint8_t duty_cycle)=0;
protected:
	uint8_t _prescaler;
	uint32_t _frequency;
	uint16_t _top;
	enum pwm_mode _pwm_mode;
};

class PWMChannel16bits : public PWMChannel{
public:
	template <size_t N>
		PWMChannel16bits(volatile uint8_t *base_addr,
				   volatile uint16_t *ocrn_addr,
				   volatile uint16_t *icrn_addr,
				   const int (&a)[N]);
	bool setPWMFrequency(uint32_t frequency);
	void setChannelDutyCycle(uint8_t channel, uint8_t duty_cycle);
	void print();
private:
	volatile union TCCRnA *_tccrnA;
	volatile union TCCRnB *_tccrnB;
	volatile uint16_t *_ocrnA;
	volatile uint16_t *_ocrnB;
#if defined(__AVR_ATmega2560__)
	volatile uint16_t *_ocrnC;
#endif
	volatile uint16_t *_icrn;
};

class PWMChannel8bits : public PWMChannel{
public:
	template <size_t N>
		PWMChannel8bits(volatile uint8_t *base_addr,
						 volatile uint8_t *ocrn_addr,
						 const int (&a)[N]);
	bool setPWMFrequency(uint32_t frequency);
	void setChannelDutyCycle(uint8_t channel, uint8_t duty_cycle);
private:
	volatile union TCCRnA *_tccrnA;
	volatile union TCCRnB *_tccrnB;
	volatile uint8_t *_ocrnA;
	volatile uint8_t *_ocrnB;
};
#endif
class PWM{
public:
	PWM();
	void analogWrite(uint8_t pin, uint8_t value);
	bool setPinFrequency(uint8_t pin, uint32_t frequency);
private:
#if defined(__AVR_ATmega2560__)
	PWMChannel* _channels[5];
#elif defined(__AVR_ATmega328P__)
	PWMChannel* _channels[3];
#endif
};
