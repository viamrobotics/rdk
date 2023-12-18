package control

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

func newPID(config BlockConfig, logger logging.Logger) (Block, error) {
	p := &basicPID{cfg: config, logger: logger}
	if err := p.reset(); err != nil {
		return nil, err
	}
	return p, nil
}

// BasicPID is the standard implementation of a PID controller.
type basicPID struct {
	mu       sync.Mutex
	cfg      BlockConfig
	error    float64
	kI       float64
	kD       float64
	kP       float64
	int      float64
	sat      int
	y        []*Signal
	satLimUp float64 `default:"255.0"`
	limUp    float64 `default:"255.0"`
	satLimLo float64
	limLo    float64
	tuner    pidTuner
	tuning   bool
	logger   logging.Logger
}

// Output returns the discrete step of the PID controller, dt is the delta time between two subsequent call,
// setPoint is the desired value, measured is the measured value.
// Returns false when the output is invalid (the integral is saturating) in this case continue to use the last valid value.
func (p *basicPID) Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tuning {
		out, done := p.tuner.pidTunerStep(math.Abs(x[0].GetSignalValueAt(0)), p.logger)
		if done {
			p.kD = p.tuner.kD
			p.kI = p.tuner.kI
			p.kP = p.tuner.kP
			p.logger.CInfof(ctx, "Calculated gains are Kp %1.6f, Ki: %1.6f, Kd: %1.6f", p.kP, p.kI, p.kD)
			p.tuning = false
		}
		p.y[0].SetSignalValueAt(0, out)
	} else {
		dtS := dt.Seconds()
		pvError := x[0].GetSignalValueAt(0)
		if (p.sat > 0 && pvError > 0) || (p.sat < 0 && pvError < 0) {
			return p.y, false
		}
		p.int += p.kI * pvError * dtS
		switch {
		case p.int >= p.satLimUp:
			p.int = p.satLimUp
			p.sat = 1
		case p.int <= p.satLimLo:
			p.int = p.satLimLo
			p.sat = -1
		default:
			p.sat = 0
		}
		deriv := (pvError - p.error) / dtS
		output := p.kP*pvError + p.int + p.kD*deriv
		p.error = pvError
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

	if !p.cfg.Attribute.Has("kI") &&
		!p.cfg.Attribute.Has("kD") &&
		!p.cfg.Attribute.Has("kP") {
		return errors.Errorf("pid block %s should have at least one kI, kP or kD field", p.cfg.Name)
	}
	if len(p.cfg.DependsOn) != 1 {
		return errors.Errorf("pid block %s should have 1 input got %d", p.cfg.Name, len(p.cfg.DependsOn))
	}
	p.kI = p.cfg.Attribute["kI"].(float64)
	p.kD = p.cfg.Attribute["kD"].(float64)
	p.kP = p.cfg.Attribute["kP"].(float64)

	// ensure a default of 255
	p.satLimUp = 255
	if satLimUp, ok := p.cfg.Attribute["int_sat_lim_up"].(float64); ok {
		p.satLimUp = satLimUp
	}

	// ensure a default of 255
	p.limUp = 255
	if limup, ok := p.cfg.Attribute["limit_up"].(float64); ok {
		p.limUp = limup
	}

	// zero float64 for this value is default in the pid struct
	// by golang
	if p.cfg.Attribute.Has("int_sat_lim_lo") {
		p.satLimLo = p.cfg.Attribute["int_sat_lim_lo"].(float64)
	}

	// zero float64 for this value is default in the pid struct
	// by golang
	if p.cfg.Attribute.Has("limit_lo") {
		p.limLo = p.cfg.Attribute["limit_lo"].(float64)
	}

	p.tuning = false
	if p.kI == 0.0 && p.kD == 0.0 && p.kP == 0.0 {
		var ssrVal float64
		if p.cfg.Attribute["tune_ssr_value"] != nil {
			ssrVal = p.cfg.Attribute["tune_ssr_value"].(float64)
		}

		tuneStepPct := 0.35
		if p.cfg.Attribute.Has("tune_step_pct") {
			tuneStepPct = p.cfg.Attribute["tune_step_pct"].(float64)
		}

		tuneMethod := tuneMethodZiegerNicholsPID
		if p.cfg.Attribute.Has("tune_method") {
			tuneMethod = tuneCalcMethod(p.cfg.Attribute["tune_method"].(string))
		}

		p.tuner = pidTuner{
			limUp:      p.limUp,
			limLo:      p.limLo,
			ssRValue:   ssrVal,
			tuneMethod: tuneMethod,
			stepPct:    tuneStepPct,
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
	p.y = make([]*Signal, 1)
	p.y[0] = makeSignal(p.cfg.Name)
	return nil
}

func (p *basicPID) Reset(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reset()
}

func (p *basicPID) UpdateConfig(ctx context.Context, config BlockConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = config
	return p.reset()
}

func (p *basicPID) Output(ctx context.Context) []*Signal {
	return p.y
}

func (p *basicPID) Config(ctx context.Context) BlockConfig {
	return p.cfg
}

type tuneCalcMethod string

const (
	tuneMethodZiegerNicholsPI            tuneCalcMethod = "ziegerNicholsPI"
	tuneMethodZiegerNicholsPID           tuneCalcMethod = "ziegerNicholsPID"
	tuneMethodZiegerNicholsSomeOvershoot tuneCalcMethod = "ziegerNicholsSomeOvershoot"
	tuneMethodZiegerNicholsNoOvershoot   tuneCalcMethod = "ziegerNicholsNoOvershoot"
	tuneMethodCohenCoonsPI               tuneCalcMethod = "cohenCoonsPI"
	tuneMethodCohenCoonsPID              tuneCalcMethod = "cohenCoonsPID"
	tuneMethodTyreusLuybenPI             tuneCalcMethod = "tyreusLuybenPI"
	tuneMethodTyreusLuybenPID            tuneCalcMethod = "tyreusLuybenPID"
)

const (
	begin = iota
	step
	relay
	end
)

type pidTuner struct {
	kI           float64
	kD           float64
	kP           float64
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
	stepPct      float64 `default:".35"`
	limUp        float64
	limLo        float64
	ssRValue     float64 `default:"2.0"`
	ccT2         time.Duration
	ccT3         time.Duration
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
		p.kP = 0.4545 * kU
		p.kI = 0.5454 * (kU / pU)
		p.kD = 0
	case tuneMethodZiegerNicholsPID:
		p.kP = 0.6 * kU
		p.kI = 1.2 * (kU / pU)
		p.kD = 0.075 * kU * pU
	case tuneMethodZiegerNicholsSomeOvershoot:
		p.kP = 0.333 * kU
		p.kI = 0.66666 * (kU / pU)
		p.kD = 0.1111 * kU * pU
	case tuneMethodZiegerNicholsNoOvershoot:
		p.kP = 0.2 * kU
		p.kI = 0.4 * (kU / pU)
		p.kD = 0.0666 * kU * pU
	case tuneMethodTyreusLuybenPI:
		p.kP = 0.3215 * kU
		p.kI = 0.1420 * (kU / pU)
		p.kD = 0.0
	case tuneMethodTyreusLuybenPID:
		p.kP = 0.4545 * kU
		p.kI = 0.2066 * (kU / pU)
		p.kD = 0.0721 * kU * pU
	case tuneMethodCohenCoonsPI:
		t1 := (p.ccT2.Seconds() - math.Log(2.0)*p.ccT3.Seconds()) / (1.0 - math.Log(2.0))
		tau := p.ccT3.Seconds() - t1
		tauD := t1
		K := (p.avgSpeedSS / stepPwr)
		r := tauD / tau
		p.kP = (1.0 / (K * r)) * (0.9 + r/12)
		p.kI = p.kP / (tauD) * (30 + 3*r) / (9 + 20*r)
	case tuneMethodCohenCoonsPID:
		t1 := (p.ccT2.Seconds() - math.Log(2.0)*p.ccT3.Seconds()) / (1.0 - math.Log(2.0))
		tau := p.ccT3.Seconds() - t1
		tauD := t1
		K := (p.avgSpeedSS / stepPwr)
		r := tauD / tau
		p.kP = (1.0 / (K * r)) * (4.0/3.0 + r/4)
		p.kI = p.kP / (tauD) * (32 + 6*r) / (13 + 8*r)
		p.kD = p.kP / (4 * tauD / (11 + 2*r))
	default:
		p.kP = 0.4545 * kU
		p.kI = 0.5454 * (kU / pU)
		p.kD = 0
	}
}

func pidTunerFindTCat(speeds []float64, times []time.Time, speed float64) time.Duration {
	for i, v := range speeds {
		if v > speed {
			return times[i].Sub(times[0])
		}
	}
	return time.Duration(0)
}

func (p *pidTuner) pidTunerStep(pv float64, logger logging.Logger) (float64, bool) {
	l1 := 0.2
	l2 := 0.1
	l3 := 0.1
	stepPwr := p.limUp * p.stepPct
	switch p.currentPhase {
	case begin:
		logger.Infof("starting the PID tunning process method %s SSR value %1.3f", p.tuneMethod, p.ssRValue)
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
			p.tS = time.Now()
			p.lastR = time.Now()
			p.avgSpeedSS = 0.0
			for i := 0; i < 5; i++ {
				p.avgSpeedSS += p.stepRsp[len(p.stepRsp)-6]
			}
			p.avgSpeedSS /= 5
			if p.tuneMethod == tuneMethodCohenCoonsPI || p.tuneMethod == tuneMethodCohenCoonsPID {
				p.out = 0.0
				p.ccT2 = pidTunerFindTCat(p.stepRsp, p.stepRespT, 0.5*p.avgSpeedSS)
				p.ccT3 = pidTunerFindTCat(p.stepRsp, p.stepRespT, 0.632*p.avgSpeedSS)
				p.computeGains()
				p.currentPhase = end
			} else {
				p.out = stepPwr + 0.5*stepPwr
				p.currentPhase = relay
			}
			p.tC = pidTunerFindTCat(p.stepRsp, p.stepRespT, 0.85*p.avgSpeedSS)
			p.pFindDir = 1
		} else if time.Since(p.tS) > 5*time.Second {
			logger.Errorf("couldn't reach steady state  r value %1.4f", r)
			p.out = 0.0
			p.currentPhase = end
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
		if int(pv) == 0 {
			return 0.0, true
		}
		return 0.0, false
	default:
		return 0.0, false
	}
}

func (p *pidTuner) reset() error {
	p.out = 0.0
	p.kI = 0.0
	p.kD = 0.0
	p.kP = 0.0
	return nil
}
