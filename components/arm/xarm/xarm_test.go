package xarm

import (
	"context"
	"net"
	"strconv"
	"testing"
	"fmt"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var (
	home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})
	wbY   = -426.
)

// This will test solving the path to write the word "VIAM" on a whiteboard.
func TestWriteViam(t *testing.T) {
	fs := frame.NewEmptyFrameSystem("test")

	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerOriginFrame, err := frame.NewStaticFrame(
		"marker_origin",
		spatial.NewPoseFromOrientation(&spatial.OrientationVectorDegrees{OY: -1, OZ: 1}),
	)
	test.That(t, err, test.ShouldBeNil)
	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerOriginFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, markerOriginFrame)
	test.That(t, err, test.ShouldBeNil)

	eraserOriginFrame, err := frame.NewStaticFrame(
		"eraser_origin",
		spatial.NewPoseFromOrientation(&spatial.OrientationVectorDegrees{OY: 1, OZ: 1}),
	)
	test.That(t, err, test.ShouldBeNil)
	eraserFrame, err := frame.NewStaticFrame("eraser", spatial.NewPoseFromPoint(r3.Vector{0, 0, 160}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(eraserOriginFrame, m)
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(eraserFrame, eraserOriginFrame)
	test.That(t, err, test.ShouldBeNil)

	moveFrame := eraserFrame

	// draw pos start
	goal := spatial.NewPoseFromProtobuf(&pb.Pose{
		X:  230,
		Y:  wbY + 10,
		Z:  600,
		OY: -1,
	})

	seedMap := map[string][]frame.Input{}

	seedMap[m.Name()] = home7

	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             logger,
		Goal:               frame.NewPoseInFrame(frame.World, goal),
		Frame:              moveFrame,
		StartConfiguration: seedMap,
		FrameSystem:        fs,
	})
	test.That(t, err, test.ShouldBeNil)

	opt := map[string]interface{}{"motion_profile": motionplan.LinearMotionProfile}
	goToGoal := func(seedMap map[string][]frame.Input, goal spatial.Pose) map[string][]frame.Input {
		plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
			Logger:             logger,
			Goal:               frame.NewPoseInFrame(fs.World().Name(), goal),
			Frame:              moveFrame,
			StartConfiguration: seedMap,
			FrameSystem:        fs,
			Options:            opt,
		})
		test.That(t, err, test.ShouldBeNil)
		return plan.Trajectory()[len(plan.Trajectory())-1]
	}

	seed := plan.Trajectory()[len(plan.Trajectory())-1]
	for _, goal = range viamPoints {
		seed = goToGoal(seed, goal)
	}
}

var viamPoints = []spatial.Pose{
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 200, Y: wbY + 1.5, Z: 595, OY: -1}),
	spatial.NewPoseFromProtobuf(&pb.Pose{X: 120, Y: wbY + 1.5, Z: 595, OY: -1}),
}

func TestReconfigure(t *testing.T) {
	listener1, err := net.Listen("tcp4", "127.0.0.1:0")
	test.That(t, err, test.ShouldBeNil)
	defer listener1.Close()
	addr1 := listener1.Addr().String()
	listener2, err := net.Listen("tcp4", "127.0.0.1:0")
	test.That(t, err, test.ShouldBeNil)
	defer listener2.Close()
	addr2 := listener2.Addr().String()
	host1, port1Str, err := net.SplitHostPort(addr1)
	test.That(t, err, test.ShouldBeNil)
	host2, port2Str, err := net.SplitHostPort(addr2)
	test.That(t, err, test.ShouldBeNil)

	port1, err := strconv.ParseInt(port1Str, 10, 32)
	test.That(t, err, test.ShouldBeNil)
	port2, err := strconv.ParseInt(port2Str, 10, 32)
	test.That(t, err, test.ShouldBeNil)

	cfg := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:        0.3,
			Host:         host1,
			Port:         int(port1),
			Acceleration: 0.1,
		},
	}

	shouldNotReconnectCfg := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:        0.5,
			Host:         host1,
			Port:         int(port1),
			Acceleration: 0.3,
		},
	}

	shouldReconnectCfg := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:        0.6,
			Host:         host2,
			Port:         int(port2),
			Acceleration: 0.34,
		},
	}

	conf, err := resource.NativeConfig[*Config](cfg)
	test.That(t, err, test.ShouldBeNil)
	confNotReconnect, ok := shouldNotReconnectCfg.ConvertedAttributes.(*Config)
	test.That(t, ok, test.ShouldBeTrue)

	conn1, err := net.Dial("tcp", listener1.Addr().String())
	test.That(t, err, test.ShouldBeNil)
	xArm := &xArm{
		speed:  utils.DegToRad(float64(conf.Speed)),
		logger: logging.NewTestLogger(t),
	}
	xArm.mu.Lock()
	xArm.conn = conn1
	xArm.mu.Unlock()

	ctx := context.Background()

	// scenario where we do no nothing
	prevSpeed := xArm.speed
	test.That(t, xArm.Reconfigure(ctx, nil, cfg), test.ShouldBeNil)

	xArm.mu.Lock()
	currentConn := xArm.conn
	xArm.mu.Unlock()
	test.That(t, currentConn, test.ShouldEqual, conn1)
	test.That(t, xArm.speed, test.ShouldEqual, prevSpeed)

	// scenario where we do not reconnect
	test.That(t, xArm.Reconfigure(ctx, nil, shouldNotReconnectCfg), test.ShouldBeNil)

	xArm.mu.Lock()
	currentConn = xArm.conn
	xArm.mu.Unlock()
	test.That(t, currentConn, test.ShouldEqual, conn1)
	test.That(t, xArm.speed, test.ShouldEqual, float32(utils.DegToRad(float64(confNotReconnect.Speed))))

	// scenario where we have to reconnect
	err = xArm.Reconfigure(ctx, nil, shouldReconnectCfg)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "failed to start")

	xArm.mu.Lock()
	currentConn = xArm.conn
	xArm.mu.Unlock()
	test.That(t, currentConn, test.ShouldNotEqual, conn1)
	test.That(t, xArm.speed, test.ShouldEqual, float32(utils.DegToRad(float64(confNotReconnect.Speed))))
}

var path1 = [][]frame.Input{
	{{1.7490355968475342}, {0.05868128314614296}, {-1.3770853281021118}, {3.1071577072143555}, {-0.8553634881973267}, {3.1812498569488525}},
	{{1.7989929566549079}, {0.07654615402577017}, {-1.3868921468304942}, {3.0117821375343548}, {-0.8858003257392425}, {3.275497188922893}},
	{{1.7989929566549079}, {0.07654615402577017}, {-1.3868921468304942}, {3.0117821375343548}, {-0.8858003257392425}, {3.275497188922893}},
	{{1.846140290428772}, {0.09824014549498251}, {-1.3964934389141097}, {2.9262947972118876}, {-0.9206225379823832}, {3.36028982512987}},
	{{1.846140290428772}, {0.09824014549498251}, {-1.3964934389141097}, {2.9262947972118876}, {-0.9206225379823832}, {3.36028982512987}},
	{{1.8900996478645613}, {0.12369735920798652}, {-1.4066655442485894}, {2.8505584995517834}, {-0.9590599355498262}, {3.435621471310069}},
	{{1.8900996478645613}, {0.12369735920798652}, {-1.4066655442485894}, {2.8505584995517834}, {-0.9590599355498262}, {3.435621471310069}},
	{{1.9307786019610271}, {0.1527293829645062}, {-1.4178675915867145}, {2.7838615304663192}, {-1.0002394071804466}, {3.5020209892258674}},
	{{1.9307786019610271}, {0.1527293829645062}, {-1.4178675915867145}, {2.7838615304663192}, {-1.0002394071804466}, {3.5020209892258674}},
	{{1.9681461125158146}, {0.18522316205627426}, {-1.4306856330792652}, {2.725644514262645}, {-1.04357827422021}, {3.5601918256821232}},
	{{1.9681461125158146}, {0.18522316205627426}, {-1.4306856330792652}, {2.725644514262645}, {-1.04357827422021}, {3.5601918256821232}},
	{{2.0023983889104797}, {0.2210115453147712}, {-1.4453998561717656}, {2.6748208102060294}, {-1.0885220923908263}, {3.611074182234416}},
	{{2.0023983889104797}, {0.2210115453147712}, {-1.4453998561717656}, {2.6748208102060294}, {-1.0885220923908263}, {3.611074182234416}},
	{{2.033628163390064}, {0.2599162939351743}, {-1.462313645897574}, {2.6305664882450905}, {-1.1346885331465941}, {3.6552544216025074}},
	{{2.033628163390064}, {0.2599162939351743}, {-1.462313645897574}, {2.6305664882450905}, {-1.1346885331465941}, {3.6552544216025074}},
	{{2.0620219603867143}, {0.30176352556833125}, {-1.4816074657339349}, {2.59203067450223}, {-1.181733714803789}, {3.693516558802132}},
	{{2.0620219603867143}, {0.30176352556833125}, {-1.4816074657339349}, {2.59203067450223}, {-1.181733714803789}, {3.693516558802132}},
	{{2.0878340524318983}, {0.3464326905756526}, {-1.5034172478029764}, {2.558332764333502}, {-1.2294451581051111}, {3.726688098643356}},
	{{2.0878340524318983}, {0.3464326905756526}, {-1.5034172478029764}, {2.558332764333502}, {-1.2294451581051111}, {3.726688098643356}},
	{{2.1111744362349754}, {0.3936841413608136}, {-1.5277793981655028}, {2.5287831064823925}, {-1.2775731229088714}, {3.7551484013427165}},
	{{2.1111744362349754}, {0.3936841413608136}, {-1.5277793981655028}, {2.5287831064823925}, {-1.2775731229088714}, {3.7551484013427165}},
	{{2.1321946520339257}, {0.4435369303093421}, {-1.555108362370631}, {2.5028524803057515}, {-1.3262389610196539}, {3.7792953667345905}},
	{{2.1321946520339257}, {0.4435369303093421}, {-1.555108362370631}, {2.5028524803057515}, {-1.3262389610196539}, {3.7792953667345905}},
	{{2.1510684805585045}, {0.49587475588901625}, {-1.5854846124921587}, {2.4798621342722047}, {-1.3753913188138343}, {3.79948926911724}},
	{{2.1510684805585045}, {0.49587475588901625}, {-1.5854846124921587}, {2.4798621342722047}, {-1.3753913188138343}, {3.79948926911724}},
	{{2.16795849672154}, {0.5505047278019725}, {-1.618852307867196}, {2.459295637170888}, {-1.424783527105031}, {3.8163030917636656}},
	{{2.16795849672154}, {0.5505047278019725}, {-1.618852307867196}, {2.459295637170888}, {-1.424783527105031}, {3.8163030917636656}},
	{{2.182950706084461}, {0.6075323635542813}, {-1.655691881103493}, {2.440587407863786}, {-1.4747985848491776}, {3.8294994549546537}},
}
var path2 = [][]frame.Input{
	{{3.042862892150879}, {0.1075424998998642}, {-0.08584814518690109}, {3.042861223220825}, {1.5923360586166382}, {3.139451265335083}},
	{{3.0496066790308882}, {0.12209198020856224}, {-0.11763856374607821}, {3.0495825385619355}, {1.575142473535384}, {3.141184937173432}},
	{{3.0496066790308882}, {0.12209198020856224}, {-0.11763856374607821}, {3.0495825385619355}, {1.575142473535384}, {3.141184937173432}},
	{{3.055494962717451}, {0.13709963755979754}, {-0.14964684998863004}, {3.0554366109780284}, {1.5582317239942707}, {3.1426661054497274}},
	{{3.055494962717451}, {0.13709963755979754}, {-0.14964684998863004}, {3.0554366109780284}, {1.5582317239942707}, {3.1426661054497274}},
	{{3.0607237879315274}, {0.15233571899183002}, {-0.1818581554426255}, {3.0606393101066707}, {1.5412861104098483}, {3.1439790926159095}},
	{{3.0607237879315274}, {0.15233571899183002}, {-0.1818581554426255}, {3.0606393101066707}, {1.5412861104098483}, {3.1439790926159095}},
	{{3.065326784829946}, {0.16794368997671633}, {-0.21428508987012831}, {3.0651857154298456}, {1.5245438290662428}, {3.1451110720042417}},
	{{3.065326784829946}, {0.16794368997671633}, {-0.21428508987012831}, {3.0651857154298456}, {1.5245438290662428}, {3.1451110720042417}},
	{{3.0694539576993547}, {0.18369911562288946}, {-0.24692208571736587}, {3.069250862923244}, {1.5076601999455768}, {3.146152752260632}},
	{{3.0694539576993547}, {0.18369911562288946}, {-0.24692208571736587}, {3.069250862923244}, {1.5076601999455768}, {3.146152752260632}},
	{{3.073170723438771}, {0.19977318992613546}, {-0.2797827519405359}, {3.0729034091453187}, {1.4909192935117823}, {3.1470715680564494}},
	{{3.073170723438771}, {0.19977318992613546}, {-0.2797827519405359}, {3.0729034091453187}, {1.4909192935117823}, {3.1470715680564494}},
	{{3.0764825564835294}, {0.21609163548783414}, {-0.312872455098027}, {3.076104011612212}, {1.4742023090503906}, {3.1479131986599174}},
	{{3.0764825564835294}, {0.21609163548783414}, {-0.312872455098027}, {3.076104011612212}, {1.4742023090503906}, {3.1479131986599174}},
	{{3.079540956369508}, {0.2325034166417384}, {-0.34619411638550923}, {3.0790831775134877}, {1.45724522582298}, {3.14867590337679}},
	{{3.079540956369508}, {0.2325034166417384}, {-0.34619411638550923}, {3.0790831775134877}, {1.45724522582298}, {3.14867590337679}},
	{{3.082341957868354}, {0.24934834195242916}, {-0.3797632704184457}, {3.0818190764122053}, {1.4406529748017143}, {3.1493550808199355}},
	{{3.082341957868354}, {0.24934834195242916}, {-0.3797632704184457}, {3.0818190764122053}, {1.4406529748017143}, {3.1493550808199355}},
	{{3.0848095559712863}, {0.2661776445467059}, {-0.4135964534521988}, {3.0841078165790705}, {1.4235846852224017}, {3.150021607394902}},
	{{3.0848095559712863}, {0.2661776445467059}, {-0.4135964534521988}, {3.0841078165790705}, {1.4235846852224017}, {3.150021607394902}},
	{{3.087157275293423}, {0.2832648719647611}, {-0.44769662999856763}, {3.086350817185959}, {1.4065478397473117}, {3.1506293814200164}},
	{{3.087157275293423}, {0.2832648719647611}, {-0.44769662999856763}, {3.086350817185959}, {1.4065478397473117}, {3.1506293814200164}},
	{{3.089327952339996}, {0.300688663024497}, {-0.48208033182864474}, {3.088423204022694}, {1.389678018076631}, {3.1511817662441035}},
	{{3.089327952339996}, {0.300688663024497}, {-0.48208033182864474}, {3.088423204022694}, {1.389678018076631}, {3.1511817662441035}},
	{{3.0912886595622604}, {0.31818841997407726}, {-0.5167686205213017}, {3.0902167471318656}, {1.372451495145806}, {3.15172006150047}},
	{{3.0912886595622604}, {0.31818841997407726}, {-0.5167686205213017}, {3.0902167471318656}, {1.372451495145806}, {3.15172006150047}},
	{{3.0931329844626085}, {0.33597658102980166}, {-0.5517721994297932}, {3.0919211399399993}, {1.3552663284244422}, {3.152207537563143}},
	{{3.0931329844626085}, {0.33597658102980166}, {-0.5517721994297932}, {3.0919211399399993}, {1.3552663284244422}, {3.152207537563143}},
	{{3.094853734758339}, {0.3539772349548478}, {-0.5871067253193913}, {3.0934990175901507}, {1.3379592399888482}, {3.1526700821057276}},
}

func TestJointSteps(t *testing.T) {
	model, err := frame.UnmarshalModelJSON(xArm6modeljson, "")
	test.That(t, err, test.ShouldBeNil)
	x := &xArm{
		speed: utils.DegToRad(50),
		acceleration: utils.DegToRad(1),
		moveHZ: 100,
		model: model,
	}
	//~ startJoints := frame.FloatsToInputs([]float64{2, 0, 0, 0, 0, 0, 0})
	//~ goalJoints1 := frame.FloatsToInputs([]float64{2.1, 0, 0, 0.2, 0, 0, 0})
	//~ goalJoints2 := frame.FloatsToInputs([]float64{4, 2, 0, 0, 0, 0, 0})
	//~ fmt.Println(x.createRawJointSteps(path1[0], path1))
	fmt.Println(x.createRawJointSteps(path2[0], path2))
}
