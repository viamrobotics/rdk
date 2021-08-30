#include "pwm.h"

#define ARRAY_SIZE(a)									\
	((sizeof(a) / sizeof(*(a))) /						\
	 static_cast<size_t>(!(sizeof(a) % sizeof(*(a)))))

#if defined(__AVR_ATmega2560__) || defined(__AVR_ATmega328P__)
PWMChannel::PWMChannel() :
	_prescaler(0),
	_frequency(0),
	_top(0),
	_pwm_mode(PWM_PHASE_FREQUENCY_CORRECT_MODE)
{
}

template <size_t N>
PWMChannel16bits::PWMChannel16bits(volatile uint8_t *base_addr,volatile uint16_t *ocrn_addr,volatile uint16_t *icrn_addr, const int (&a)[N])
	: _tccrnA((volatile union TCCRnA*)base_addr),
	  _tccrnB((volatile union TCCRnB*)base_addr+1),
	  _ocrnA(ocrn_addr),
	  _ocrnB(ocrn_addr+1),
#if defined(__AVR_ATmega2560__)
	  _ocrnC(ocrn_addr+2),
#endif
	  _icrn(icrn_addr),
	  _dutyA(0),
	  _dutyB(0),
	  _dutyC(0)
{
	_frequency = 0;
	_top = 0;
	_tccrnA->reg = 0;
	_tccrnB->reg = 0;
	*_ocrnA = 0;
	*_ocrnB = 0;
#if defined(__AVR_ATmega2560__)
	*_ocrnC = 0;
#endif
	for(int i : a)
	{
		pinMode(i, OUTPUT);
	}
}


void PWMChannel16bits::setChannelDutyCycle(uint8_t channel, uint8_t duty_cycle)
{
	if(channel > PWM_CHANNEL_C){
		return;
	}
	uint32_t ocrn = 0;
	ocrn = (_top*((uint32_t)duty_cycle)) >> 8; /*To represent the range [0,icrn] in [0;255] we multiply by duty_cyle then divide by 256*/
	switch(channel){
	case PWM_CHANNEL_A:
		_dutyA = duty_cycle;
		_tccrnA->comn_A1 = 1;
		_tccrnA->comn_A0 = 0;
		*_ocrnA = ocrn & 0xFFFF;
		break;
	case PWM_CHANNEL_B:
		_dutyB = duty_cycle;
		_tccrnA->comn_B1 = 1;
		_tccrnA->comn_B0 = 0;
		*_ocrnB = ocrn & 0xFFFF;
		break;
#if defined(__AVR_ATmega2560__)
	case PWM_CHANNEL_C:
		_dutyC = duty_cycle;
		_tccrnA->comn_C1 = 1;
		_tccrnA->comn_C0 = 0;
		*_ocrnC = ocrn & 0xFFFF;
		break;
#endif
	}
}
bool PWMChannel16bits::setPWMFrequency(uint32_t frequency)
{
	uint32_t top = 0;
	uint16_t prescaler[] = {1,8,64,256,1024};
	uint8_t n = 0;
	if(!frequency) return false;
	switch(_pwm_mode){
	case PWM_FAST_PWM_MODE:
	case PWM_PHASE_CORRECT_MODE:
	case PWM_PHASE_FREQUENCY_CORRECT_MODE:
		do{
			top = BASE_CLK_FREQ/(2*prescaler[n]*frequency); /* See datasheet for frequency equation */
			if(top > 0x3 && top <= 0xFFFF) break;
			++n;
		}while(n < ARRAY_SIZE(prescaler));
		if((n < ARRAY_SIZE(prescaler)) && top && top < 0xFFFF)
		{
			_tccrnB->csn = 0; // Stop clocking PWM when changing its registers
			*_icrn = top & 0xFFFF;
			_tccrnB->wgmn3 = 1;
			_tccrnB->wgmn2 = 0;
			_tccrnA->wgmn1 = 0;
			_tccrnA->wgmn0 = 0;
			_top = top;
			_frequency = frequency;
			setChannelDutyCycle(PWM_CHANNEL_A,_dutyA);
			setChannelDutyCycle(PWM_CHANNEL_B,_dutyB);
#if defined(__AVR_ATmega2560__)
			setChannelDutyCycle(PWM_CHANNEL_C,_dutyC);
#endif
			_tccrnB->csn = n + 1;
			return true;
		}
		break;
	}
	return false;
}

template <size_t N>
PWMChannel8bits::PWMChannel8bits(volatile uint8_t *base_addr,volatile uint8_t *ocrn_addr, const int (&a)[N])
	: _tccrnA((volatile union TCCRnA*)base_addr),
	  _tccrnB((volatile union TCCRnB*)base_addr+1),
	  _ocrnA(ocrn_addr),
	  _ocrnB(ocrn_addr+1),
	  _dutyB(0)
{
	_tccrnA->reg = 0;
	_tccrnB->reg = 0;
	*_ocrnA = 0;
	*_ocrnB = 0;
	_frequency = 0;
	_top = 0;
	for(int i : a)
	{
		pinMode(i, OUTPUT);
	}
}


void PWMChannel8bits::setChannelDutyCycle(uint8_t channel, uint8_t duty_cycle)
{
	if(channel != PWM_CHANNEL_B){
		return;
	}
	uint32_t ocrn = 0;
	ocrn = (_top*((uint32_t)duty_cycle)) >> 8;
	switch(channel){
	case PWM_CHANNEL_B:
		_dutyB = duty_cycle;
		_tccrnA->comn_B1 = 1;
		_tccrnA->comn_B0 = 0;
		*_ocrnB = ocrn & 0xFF;
		break;
	}
}
bool PWMChannel8bits::setPWMFrequency(uint32_t frequency)
{
	uint32_t top = 0;
	uint16_t prescaler[] = {1,8,64,256,1024};
	uint8_t n = 0;
	switch(_pwm_mode){
	case PWM_FAST_PWM_MODE:
	case PWM_PHASE_CORRECT_MODE:
	case PWM_PHASE_FREQUENCY_CORRECT_MODE:
		do{
			top = BASE_CLK_FREQ/(2*prescaler[n]*frequency);
			if(top > 0x3 && top <= 0xFF) break;
			++n;
		}while(n < ARRAY_SIZE(prescaler));
		if((n < ARRAY_SIZE(prescaler)) && top && top <= 0xFF)
		{
			_tccrnB->csn = 0; // stop clocking PWM when changing its register
			*_ocrnA = top & 0xFF;
			_tccrnB->wgmn3 = 0;
			_tccrnB->wgmn2 = 1;
			_tccrnA->wgmn1 = 0;
			_tccrnA->wgmn0 = 1;
			_top = top;
			_frequency = frequency;
			setChannelDutyCycle(PWM_CHANNEL_B,_dutyB);
			_tccrnB->csn = n + 1;
			return true;
		}
		break;
	}
	return false;
}
#endif
PWM::PWM()
{
#if defined(__AVR_ATmega2560__)
	_channels[0] = new PWMChannel16bits(&TCCR4A,&OCR4A,&ICR4, {6,7,8});
	_channels[1] = new PWMChannel16bits(&TCCR3A,&OCR3A,&ICR3, {5,3,2});
	_channels[2] = new PWMChannel16bits(&TCCR1A,&OCR1A,&ICR1, {11,12,13});
	_channels[3] = new PWMChannel8bits(&TCCR2A,&OCR2A,{9});
#elif defined(__AVR_ATmega328P__)
	_channels[0] = new PWMChannel16bits(&TCCR1A,&OCR1A,&ICR1, {9,10});
	_channels[1] = new PWMChannel8bits(&TCCR2A,&OCR2A, {3});
#endif
}

bool PWM::setPinFrequency(uint8_t pin, uint32_t frequency)
{
#if defined(__AVR_ATmega2560__)
	switch(pin){
	case 11 ... 13:
		return _channels[2]->setPWMFrequency(frequency);
		break;
	case 6 ... 8:
		return _channels[0]->setPWMFrequency(frequency);
		break;
	case 5:
	case 3:
	case 2:
		return _channels[1]->setPWMFrequency(frequency);
		break;
	case 9:
		return _channels[3]->setPWMFrequency(frequency);
	}
#elif defined(__AVR_ATmega328P__)
	switch(pin){
	case 9 ... 10:
		return _channels[0]->setPWMFrequency(frequency);
	case 3:
		return _channels[1]->setPWMFrequency(frequency);
	}
#endif

	return false;
}
void PWM::analogWrite(uint8_t pin, uint8_t value)
{
	if(value == 0){
		digitalWrite(pin,LOW);
	}
	else if(value == 255){
		digitalWrite(pin,HIGH);
	}
	else{
#if defined(__AVR_ATmega2560__)
		switch(pin){
		case 6:
			_channels[0]->setChannelDutyCycle(PWM_CHANNEL_A,value);
			break;
		case 7:
			_channels[0]->setChannelDutyCycle(PWM_CHANNEL_B,value);
			break;
		case 8:
			_channels[0]->setChannelDutyCycle(PWM_CHANNEL_C,value);
			break;
		case 5:
			_channels[1]->setChannelDutyCycle(PWM_CHANNEL_A,value);
			break;
		case 3:
			_channels[1]->setChannelDutyCycle(PWM_CHANNEL_B,value);
			break;
		case 2:
			_channels[1]->setChannelDutyCycle(PWM_CHANNEL_C,value);
			break;
		case 11:
			_channels[2]->setChannelDutyCycle(PWM_CHANNEL_A,value);
			break;
		case 12:
			_channels[2]->setChannelDutyCycle(PWM_CHANNEL_B,value);
			break;
		case 13:
			_channels[2]->setChannelDutyCycle(PWM_CHANNEL_C,value);
			break;
		case 9:
			_channels[3]->setChannelDutyCycle(PWM_CHANNEL_B,value);
			break;
		case 10:
			break;
		default:
			::analogWrite(pin,value);
			break;
		}
#elif defined(__AVR_ATmega328P__)
		switch(pin){
		case 9:
			_channels[0]->setChannelDutyCycle(PWM_CHANNEL_A,value);
			break;
		case 10:
			_channels[0]->setChannelDutyCycle(PWM_CHANNEL_B,value);
			break;
		case 3:
			_channels[1]->setChannelDutyCycle(PWM_CHANNEL_B,value);
			break;
		case 11:
			break;
		default:
			::analogWrite(pin,value);
		}
#else
	::analogWrite(pin,value); /* Default to arduino analog write when pin is unhandeled */
#endif
	}
}
