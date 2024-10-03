package control

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

func (l *Loop) newPID(config BlockConfig, logger logging.Logger) (Block, error) {
	p := &basicPID{cfg: config, logger: logger}
	if err := p.reset(); err != nil {
		return nil, err
	}
	l.pidBlocks = append(l.pidBlocks, p)
	return p, nil
}

// BasicPID is the standard implementation of a PID controller.
type basicPID struct {
	mu     sync.Mutex
	cfg    BlockConfig
	logger logging.Logger

	// MIMO gains + state
	PIDSets []*PIDConfig
	tuners  []*pidTuner

	// used by both
	y        []*Signal
	satLimUp float64 `default:"255.0"`
	limUp    float64 `default:"255.0"`
	satLimLo float64
	limLo    float64
}

// GetTuning returns whether the PID block is currently tuning any signals.
func (p *basicPID) GetTuning() bool {
	// using locks to prevent reading from tuners while the object is being modified
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.getTuning()
}

func (p *basicPID) getTuning() bool {
	multiTune := false
	for _, tuner := range p.tuners {
		// the tuners for MIMO are only initialized when we want to tune
		if tuner != nil {
			multiTune = tuner.tuning || multiTune
		}
	}
	return multiTune
}

// Output returns the discrete step of the PID controller, dt is the delta time between two subsequent call,
// setPoint is the desired value, measured is the measured value.
// Returns false when the output is invalid (the integral is saturating) in this case continue to use the last valid value.
func (p *basicPID) Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.getTuning() {
		// Multi Input/Output Implementation

		// For each PID Set and its respective Tuner Object, Step through an iteration of tuning until done.
		for i := 0; i < len(p.PIDSets); i++ {
			// if we do not need to tune this signal, skip to the next signal
			if !p.tuners[i].tuning {
				continue
			}
			out, done := p.tuners[i].pidTunerStep(math.Abs(x[0].GetSignalValueAt(i)), p.logger)
			if done {
				p.PIDSets[i].D = p.tuners[i].kD
				p.PIDSets[i].I = p.tuners[i].kI
				p.PIDSets[i].P = p.tuners[i].kP
				p.logger.Info("\n\n-------- ***** PID GAINS CALCULATED **** --------")
				p.logger.CInfof(ctx, "Calculated gains for signal %v are p: %1.6f, i: %1.6f, d: %1.6f",
					i, p.PIDSets[i].P, p.PIDSets[i].I, p.PIDSets[i].D)
				p.logger.CInfof(ctx, "You must MANUALLY ADD p, i and d gains to the robot config to use the values after tuning\n\n")
				p.tuners[i].tuning = false
			}
			p.y[0].SetSignalValueAt(i, out)
			// return early to only step this signal
			return p.y, true

		}
	} else {
		for i := 0; i < len(p.PIDSets); i++ {
			output := calculateSignalValue(p, x, dt, i)
			p.y[0].SetSignalValueAt(i, output)
		}
	}
	return p.y, true
}

// For a given signal, compute new signal value based on current signal value, & its respective error.
func calculateSignalValue(p *basicPID, x []*Signal, dt time.Duration, sIndex int) float64 {
	dtS := dt.Seconds()
	pvError := x[0].GetSignalValueAt(sIndex)
	p.PIDSets[sIndex].int += p.PIDSets[sIndex].I * pvError * dtS

	switch {
	case p.PIDSets[sIndex].int >= p.satLimUp:
		p.PIDSets[sIndex].int = p.satLimUp
	case p.PIDSets[sIndex].int <= p.satLimLo:
		p.PIDSets[sIndex].int = p.satLimLo
	default:
	}
	deriv := (pvError - p.PIDSets[sIndex].signalErr) / dtS
	output := p.PIDSets[sIndex].P*pvError + p.PIDSets[sIndex].int + p.PIDSets[sIndex].D*deriv
	p.PIDSets[sIndex].signalErr = pvError
	if output > p.limUp {
		output = p.limUp
	} else if output < p.limLo {
		output = p.limLo
	}

	return output
}

func (p *basicPID) reset() error {
	var ok bool

	// Each PIDSet is taken from the config, if the attribute exists (it's optional).
	// If PID Sets was given as an attribute, we know we're in 'multi' mode. For each
	// set of PIDs we initialize its values to 0 and create a tuner object.
	if p.cfg.Attribute.Has("PIDSets") {
		p.PIDSets, ok = p.cfg.Attribute["PIDSets"].([]*PIDConfig)
		if !ok {
			return errors.New("PIDSet did not initialize correctly")
		}
		if len(p.PIDSets) > 0 {
			p.tuners = make([]*pidTuner, len(p.PIDSets))
			for i := 0; i < len(p.PIDSets); i++ {
				p.PIDSets[i].int = 0
				p.PIDSets[i].signalErr = 0
			}
		}
	} else {
		return errors.Errorf("pid block %s does not have a PID configured", p.cfg.Name)
	}

	if len(p.cfg.DependsOn) != len(p.PIDSets) {
		return errors.Errorf("pid block %s should have %d inputs got %d", p.cfg.Name, len(p.PIDSets), len(p.cfg.DependsOn))
	}

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

	for i := 0; i < len(p.PIDSets); i++ {
		// Create a Tuner object for our PID set. Across all Tuner objects, they share global
		// values (limUp, limLo, ssR, tuneMethod, stepPct). The only values that differ are P,I,D.
		if p.PIDSets[i].NeedsAutoTuning() {
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

			p.tuners[i] = &pidTuner{
				limUp:      p.limUp,
				limLo:      p.limLo,
				ssRValue:   ssrVal,
				tuneMethod: tuneMethod,
				stepPct:    tuneStepPct,
				kP:         p.PIDSets[i].P,
				kI:         p.PIDSets[i].I,
				kD:         p.PIDSets[i].D,
				tuning:     true,
			}

			err := p.tuners[i].reset()
			if err != nil {
				return err
			}

			if p.tuners[i].stepPct > 1 || p.tuners[i].stepPct < 0 {
				return errors.Errorf("tuner pid block %s should have a percentage value between 0-1 for TuneStepPct", p.cfg.Name)
			}
		}
	}
	// Note: In our Signal[] array, we only have one element. For MIMO, within Signal[0],
	// the length of the signal[] array is lengthened to accommodate multiple outputs.
	p.y = make([]*Signal, 1)
	p.y[0] = makeSignals(p.cfg.Name, p.cfg.Type, len(p.PIDSets))

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
	tuneMethodZiegerNicholsPD            tuneCalcMethod = "ziegerNicholsPD"
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
	tuning       bool
}

// reference for computation: https://en.wikipedia.org/wiki/Ziegler%E2%80%93Nichols_method#cite_note-1
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
		p.kD = 0.0
	case tuneMethodZiegerNicholsPD:
		p.kP = 0.8 * kU
		p.kI = 0.0
		p.kD = 0.10 * kU * pU
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
	default: // ziegler nichols PI is the default
		p.kP = 0.4545 * kU
		p.kI = 0.5454 * (kU / pU)
		p.kD = 0.0
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
	p.pPeakH = []float64{}
	p.pPeakL = []float64{}
	return nil
}
