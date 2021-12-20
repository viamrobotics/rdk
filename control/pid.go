package control

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

func newPID(config ControlBlockConfig, logger golog.Logger) (ControlBlock, error) {
	p := &basicPID{cfg: config}
	err := p.reset()
	if err != nil {
		return nil, err
	}
	return p, nil
}

// BasicPID is the standard implementation of a PID controller
type basicPID struct {
	mu       sync.Mutex
	cfg      ControlBlockConfig
	error    float64
	Ki       float64
	Kd       float64
	Kp       float64
	int      float64
	sat      int
	y        []Signal
	satLimUp float64
	limUp    float64
	satLimLo float64
	limLo    float64
	tuner    pidTuner
	tuning   bool
}

// Output returns the discrete step of the PID controller, dt is the delta time between two subsequent call, setPoint is the desired value, measured is the measured value. Returns false when the output is invalid (the integral is saturating) in this case continue to use the last valid value
func (p *basicPID) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tuning {
		out, done := p.tuner.pidTunerStep(x[0].GetSignalValueAt(0), dt)
		if done {
			p.Kd = p.tuner.Kd
			p.Ki = p.tuner.Ki
			p.Kp = p.tuner.Kp
			p.tuning = false
		}
		p.y[0].SetSignalValueAt(0, out)
	} else {
		dtS := dt.Seconds()
		error := x[0].GetSignalValueAt(0)
		if (p.sat > 0 && error > 0) || (p.sat < 0 && error < 0) {
			return p.y, false
		}
		p.int += p.Ki * p.error * dtS
		if p.int > p.satLimUp {
			p.int = p.satLimUp
			p.sat = 1
		} else if p.int < p.satLimLo {
			p.int = p.limLo
			p.sat = -1
		} else {
			p.sat = 0
		}
		deriv := (error - p.error) / dtS
		output := p.Kp*error + p.int + p.Kd*deriv
		p.error = error
		if output > p.limUp {
			output = p.limUp
		} else if output < p.limLo {
			output = p.limLo
		}
		p.y[0].SetSignalValueAt(0, output)
	}
	return p.y, true

}

func (p *basicPID) reset() error {
	p.int = 0
	p.error = 0
	p.sat = 0

	if !p.cfg.Attribute.Has("Ki") &&
		!p.cfg.Attribute.Has("Kd") &&
		!p.cfg.Attribute.Has("Kp") {
		return errors.Errorf("pid block %s should have at least one Ki, Kp or Kd field", p.cfg.Name)
	}
	if len(p.cfg.DependsOn) != 1 {
		return errors.Errorf("pid block %s should have 1 input got %d", p.cfg.Name, len(p.cfg.DependsOn))
	}
	p.Ki = p.cfg.Attribute.Float64("Ki", 0.0)
	p.Kd = p.cfg.Attribute.Float64("Kd", 0.0)
	p.Kp = p.cfg.Attribute.Float64("Kp", 0.0)
	p.satLimUp = p.cfg.Attribute.Float64("IntSatLimUp", 255.0)
	p.limUp = p.cfg.Attribute.Float64("LimitUp", 255.0)
	p.satLimLo = p.cfg.Attribute.Float64("IntSatLimLo", 0)
	p.limLo = p.cfg.Attribute.Float64("LimitLo", 0)
	p.tuning = false
	if p.Ki == 0.0 && p.Kd == 0.0 && p.Kp == 0.0 {
		p.tuner = pidTuner{
			limUp:      p.limUp,
			limLo:      p.limLo,
			ssRValue:   p.cfg.Attribute.Float64("TunessRValue", 2.0),
			tuneMethod: tuneCalcMethod(p.cfg.Attribute.String("TuneMethod")),
			stepPct:    p.cfg.Attribute.Float64("TuneStepPct", 0.35),
		}
		err := p.tuner.reset()
		if err != nil {
			return err
		}
		if p.tuner.stepPct > 1 || p.tuner.stepPct < 0 {
			return errors.Errorf("tuner pid block %s should have a percentage value between 0-1 for TuneStepPct", p.cfg.Name)
		}
		p.tuning = true
	}
	p.y = make([]Signal, 1)
	p.y[0] = makeSignal(p.cfg.Name, 1)
	return nil
}

func (p *basicPID) Reset(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reset()
}

func (p *basicPID) Configure(ctx context.Context, config ControlBlockConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = config
	return p.reset()
}
func (p *basicPID) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = config
	return p.reset()
}

func (p *basicPID) Output(ctx context.Context) []Signal {
	return p.y
}

func (p *basicPID) Config(ctx context.Context) ControlBlockConfig {
	return p.cfg
}

type tuneCalcMethod string

const (
	tuneMethodZiegerNicholsPI            tuneCalcMethod = "ZiegerNicholsPI"
	tuneMethodZiegerNicholsPID           tuneCalcMethod = "ZiegerNicholsPID"
	tuneMethodZiegerNicholsSomeOvershoot tuneCalcMethod = "ZiegerNicholsSomeOvershoot"
	tuneMethodZiegerNicholsNoOvershoot   tuneCalcMethod = "ZiegerNicholsNoOvershoot"
	tuneMethodCohenCoonsPI               tuneCalcMethod = "CohenCoonsPI"
	tuneMethodCohenCoonsPID              tuneCalcMethod = "CohenCoonsPID"
	tuneMethodTyreusLuybenPI             tuneCalcMethod = "TyreusLuybenPI"
	tuneMethodTyreusLuybenPID            tuneCalcMethod = "TyreusLuybenPID"
)

const (
	begin = iota
	step
	relay
	end
)

type pidTuner struct {
	Ki           float64
	Kd           float64
	Kp           float64
	currentPhase int
	stepRsp      []float64
	stepRespT    []time.Time
	tS           time.Time
	xF           float64
	vF           float64
	dF           float64
	pPv          float64
	lastR        time.Time
	avgSpeedSS   float64
	tC           time.Duration
	pPeakH       []float64
	pPeakL       []float64
	pFindDir     int
	tuneMethod   tuneCalcMethod
	stepPct      float64
	limUp        float64
	limLo        float64
	ssRValue     float64
	ccT2         time.Duration
	out          float64
}

func (p *pidTuner) computeGains() {
	stepPwr := p.limUp * p.stepPct
	i := 0
	a := 0.0
	for ; i < int(math.Min(float64(len(p.pPeakH)), float64(len(p.pPeakL)))); i++ {
		a += math.Abs(p.pPeakH[i] - p.pPeakL[i])
	}
	a /= (2.0 * float64(i+1))
	d := 0.5 * stepPwr
	kU := (4 * d) / (math.Pi * a)
	pU := (p.tC * 2.0).Seconds()
	switch p.tuneMethod {
	case tuneMethodZiegerNicholsPI:
		p.Kp = 0.4545 * kU
		p.Ki = 0.5454 * (kU / pU)
		p.Kd = 0
	case tuneMethodZiegerNicholsPID:
		p.Kp = 0.6 * kU
		p.Ki = 1.2 * (kU / pU)
		p.Kd = 0.075 * kU * pU
	case tuneMethodZiegerNicholsSomeOvershoot:
		p.Kp = 0.333 * kU
		p.Ki = 0.66666 * (kU / pU)
		p.Kd = 0.1111 * kU * pU
	case tuneMethodZiegerNicholsNoOvershoot:
		p.Kp = 0.2 * kU
		p.Ki = 0.4 * (kU / pU)
		p.Kd = 0.0666 * kU * pU
	case tuneMethodTyreusLuybenPI:
		p.Kp = 0.3215 * kU
		p.Ki = 0.1420 * (kU / pU)
		p.Kd = 0.0
	case tuneMethodTyreusLuybenPID:
		p.Kp = 0.4545 * kU
		p.Ki = 0.2066 * (kU / pU)
		p.Kd = 0.0721 * kU * pU
	case tuneMethodCohenCoonsPI:
		t1 := (p.ccT2.Seconds() - math.Log2(2.0)*p.tC.Seconds()) / (1.0 - math.Log2(2.0))
		tau := p.tC.Seconds() - t1
		tauD := t1
		K := (p.avgSpeedSS / stepPwr)
		r := tauD / tau
		p.Kp = (1.0 / (K * r)) * (0.9 + r/12)
		p.Ki = p.Kp * (tauD) * (30 + 3*r) / (9 + 20*r)
	case tuneMethodCohenCoonsPID:
		t1 := (p.ccT2.Seconds() - math.Log2(2.0)*p.tC.Seconds()) / (1.0 - math.Log2(2.0))
		tau := p.tC.Seconds() - t1
		tauD := t1
		K := (p.avgSpeedSS / stepPwr)
		r := tauD / tau
		p.Kp = (1.0 / (K * r)) * (4.0/3.0 + r/4)
		p.Ki = p.Kp * (tauD) * (32 + 6*r) / (13 + 8*r)
		p.Kd = p.Kp * (4 * tauD / (11 + 2*r))
	default:
		p.Kp = 0.4545 * kU
		p.Ki = 0.5454 * (kU / pU)
		p.Kd = 0
	}
}

// Output returns the discrete step of the PID controller, dt is the delta time between two subsequent call, setPoint is the desired value, measured is the measured value. Returns false when the output is invalid (the integral is saturating) in this case continue to use the last valid value
func (p *pidTuner) pidTunerStep(pv float64, dt time.Duration) (float64, bool) {
	l1 := 0.2
	l2 := 0.1
	l3 := 0.1
	stepPwr := p.limUp * p.stepPct
	switch p.currentPhase {
	case begin:
		p.currentPhase = step
		p.tS = time.Now()
		p.out = stepPwr
		return p.out, false
	case step:
		p.vF = l2*math.Pow(pv-p.xF, 2.0) + (1-l1)*p.vF
		p.dF = l3*(math.Pow(pv-p.pPv, 2.0)) + (1-l3)*p.dF
		p.xF = l1*pv + (1-l1)*p.xF
		r := (2 - l1) * p.vF / p.dF
		p.pPv = pv
		p.stepRsp = append(p.stepRsp, pv)
		p.stepRespT = append(p.stepRespT, time.Now())
		if len(p.stepRsp) > 20 && r < p.ssRValue {
			if p.tuneMethod == tuneMethodCohenCoonsPI || p.tuneMethod == tuneMethodCohenCoonsPID {
				p.out = 0.0
				p.computeGains()
				p.currentPhase = end
			} else {
				p.out = stepPwr + 0.5*stepPwr
				p.currentPhase = relay
			}
			p.tS = time.Now()
			p.lastR = time.Now()
			p.avgSpeedSS = 0.0
			for i := 0; i < 5; i++ {
				p.avgSpeedSS += p.stepRsp[len(p.stepRsp)-6]
			}
			p.avgSpeedSS /= 5
			for i, v := range p.stepRsp {
				if v > 0.5*p.avgSpeedSS && p.ccT2.Seconds() == 0.0 {
					p.ccT2 = p.stepRespT[i].Sub(p.stepRespT[0])
				}
				if v > 0.632*p.avgSpeedSS {
					p.tC = p.stepRespT[i].Sub(p.stepRespT[0])
					break
				}
			}
			p.pFindDir = 1
		}
		return p.out, false
	case relay:
		if time.Since(p.lastR) > p.tC {
			p.lastR = time.Now()
			if p.out > stepPwr {
				p.out -= stepPwr
				p.pFindDir = 1
			} else {
				p.out += stepPwr
				p.pFindDir = -1
			}
		}
		if p.pFindDir == 1 && p.pPv > pv {
			p.pFindDir = 0
			p.pPeakH = append(p.pPeakH, p.pPv)
		} else if p.pFindDir == -1 && p.pPv < pv {
			p.pFindDir = 0
			p.pPeakL = append(p.pPeakL, p.pPv)
		}
		p.pPv = pv
		if time.Since(p.tS) > 4*time.Second {
			p.out = 0
			p.computeGains()
			p.currentPhase = end
		}
		return p.out, false
	case end:
		return 0.0, true
	default:
		return 0.0, false
	}
}

func (p *pidTuner) reset() error {
	p.out = 0.0
	p.Ki = 0.0
	p.Kd = 0.0
	p.Kp = 0.0
	return nil
}
