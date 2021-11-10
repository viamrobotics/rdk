package xarm

import (
	"context"
	"fmt"
	//~ "sort"

	//~ "runtime"
	"testing"
	//~ "time"

	"go.viam.com/core/kinematics"
	pb "go.viam.com/core/proto/api/v1"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"go.viam.com/core/motionplan"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

var home7 = frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0, 0})

func TestSmooth(t *testing.T){
	
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	
	mp, err := motionplan.NewCBiRRTMotionPlanner_petertest (m, logger, 4)
	test.That(t, err, test.ShouldBeNil)
	
	input := [][]frame.Input{}
	for _, step := range steps2 {
		input = append(input, frame.FloatsToInputs(step))
	}
	
	smoothed := mp.SmoothPath(ctx, input)
	fmt.Println("\nsmoothed", smoothed)
	//~ smoothed := input
	
	
	//~ arm, err := NewxArm(ctx, "10.0.0.98", logger, 7)
	//~ arm.MoveToJointPositions(ctx, frame.InputsToJointPos(home7))
	//~ for i, smooth := range smoothed {
		//~ fmt.Println(i, smooth)
		//~ arm.MoveToJointPositions(ctx, frame.InputsToJointPos(smooth))
	//~ }
	
	//~ time.Sleep(1000 * time.Millisecond)
	//~ fmt.Println("done, reversing")
	
	//~ for i := len(smoothed) - 2; i > 0; i-- {
		//~ arm.MoveToJointPositions(ctx, frame.InputsToJointPos(smoothed[i]))
	//~ }
	//~ arm.MoveToJointPositions(ctx, frame.InputsToJointPos(home7))
	
	//~ time.Sleep(1 * time.Millisecond)
}

var wbY = -490.

func TestWrite1(t *testing.T){
	
	fs := frame.NewEmptySimpleFrameSystem("test")
	
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	
	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)
	
	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0,0,135}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, m)
	test.That(t, err, test.ShouldBeNil)
	
	
	
	// Have to be able to update the motion planner from here
	mpFunc := func(f frame.Frame, logger golog.Logger, ncpu int) (motionplan.MotionPlanner, error) {
		// just in case frame changed
		mp, err := motionplan.NewCBiRRTMotionPlanner_petertest(f, logger, 4)
		
		return mp, err
	}
	
	fss := motionplan.NewSolvableFrameSystem(fs, logger)
	
	fss.SetPlannerGen(mpFunc)
	
	arm, err := NewxArm(ctx, "10.0.0.98", logger, 7)
	// home
	//~ arm.MoveToJointPositions(ctx, frame.InputsToJointPos(home7))
	
	// draw pos start
	goal := spatial.NewPoseFromArmPos(&pb.ArmPosition{
		X:  480,
		Y:  wbY,
		Z:  600,
		OY: -1,
	})
	
	seedMap := map[string][]frame.Input{}
	
	jPos, err := arm.CurrentJointPositions(ctx)
	seedMap[m.Name()] = frame.JointPosToInputs(jPos)
	
	steps, err := fss.SolvePose(ctx, seedMap, goal, markerFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)
	
	for _, step := range steps {
		//~ fmt.Println(i, smooth)
		arm.MoveToJointPositions(ctx, frame.InputsToJointPos(step[m.Name()]))
	}
	
	validOV := &spatial.OrientationVector{OX:0, OY:-1, OZ:0}
	oldGoal := goal
	
	goToGoal := func(goal spatial.Pose){
		//~ validFunc, gradFunc := motionplan.NewLineConstraintAndGradient(oldGoal.Point(), goal.Point(), validOV)
		motionplan.NewLineConstraintAndGradient(oldGoal.Point(), goal.Point(), validOV)
		
		// update constraints
		mpFunc = func(f frame.Frame, logger golog.Logger, ncpu int) (motionplan.MotionPlanner, error) {
			// just in case frame changed
			mp, err := motionplan.NewLinearMotionPlanner(f, logger, 4)
			//~ mp, err := motionplan.NewCBiRRTMotionPlanner_petertest(f, logger, 4)
			//~ mp.SetDistFunc(gradFunc)
			//~ mp.AddConstraint("whiteboard", validFunc)
			
			return mp, err
		}
		fss.SetPlannerGen(mpFunc)

		jPos, err = arm.CurrentJointPositions(ctx)
		seedMap[m.Name()] = frame.JointPosToInputs(jPos)
		
		steps, err = fss.SolvePose(ctx, seedMap, goal, markerFrame, fs.World())
		test.That(t, err, test.ShouldBeNil)
		for _, step := range steps {
			//~ fmt.Println(i, smooth)
			arm.MoveToJointPositions(ctx, frame.InputsToJointPos(step[m.Name()]))
		}
		oldGoal = goal
	}
	
	for _, goal = range viamPoints{
		goToGoal(goal)
	}
}

func TestWrite2(t *testing.T){
	
	fs := frame.NewEmptySimpleFrameSystem("test")
	
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := kinematics.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	
	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)
	
	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0,0,135}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, m)
	test.That(t, err, test.ShouldBeNil)
	
	validFunc, gradFunc := motionplan.NewPlaneConstraintAndGradient(r3.Vector{0,1,0}, r3.Vector{0,wbY,0})
	
	// update constraints
	mpFunc := func(f frame.Frame, logger golog.Logger, ncpu int) (motionplan.MotionPlanner, error) {
		// just in case frame changed
		mp, err := motionplan.NewCBiRRTMotionPlanner_petertest(f, logger, 4)
		mp.SetDistFunc(gradFunc)
		mp.AddConstraint("whiteboard", validFunc)
		
		return mp, err
	}
	fss := motionplan.NewSolvableFrameSystem(fs, logger)
	
	fss.SetPlannerGen(mpFunc)
	
	arm, err := NewxArm(ctx, "10.0.0.98", logger, 7)
	
	goal := &pb.ArmPosition{
		X:  320,
		Y:  wbY,
		Z:  500,
		OY: -1,
	}
	
	seedMap := map[string][]frame.Input{}
	
	jPos, err := arm.CurrentJointPositions(ctx)
	seedMap[m.Name()] = frame.JointPosToInputs(jPos)
	
	steps, err := fss.SolvePose(ctx, seedMap, spatial.NewPoseFromArmPos(goal), markerFrame, fs.World())
	test.That(t, err, test.ShouldBeNil)
	
	for _, step := range steps {
		//~ fmt.Println(i, smooth)
		arm.MoveToJointPositions(ctx, frame.InputsToJointPos(step[m.Name()]))
	}
}





var viamPoints = []spatial.Pose{
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  440,Y:  wbY,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  400,Y:  wbY,Z:  600,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  400,Y:  wbY+10,Z:  600,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  380,Y:  wbY+10,Z:  600,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  380,Y:  wbY,Z:  600,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  380,Y:  wbY,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  380,Y:  wbY+10,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  360,Y:  wbY+10,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  360,Y:  wbY,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  320,Y:  wbY,Z:  600,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  280,Y:  wbY,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  280,Y:  wbY+10,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  340,Y:  wbY+10,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  340,Y:  wbY+10,Z:  550,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  340,Y:  wbY,Z:  550,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  300,Y:  wbY,Z:  550,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  300,Y:  wbY+10,Z:  550,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  260,Y:  wbY+10,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  260,Y:  wbY,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  260,Y:  wbY,Z:  600,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  230,Y:  wbY,Z:  500,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  200,Y:  wbY,Z:  600,OY: -1,}),
spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  170,Y:  wbY,Z:  500,OY: -1,}),
}










var steps1 = [][]float64{
[]float64{-1.1102230246251565e-16, -3.469446951953614e-18, 5.551115123125783e-17, -3.469446951953614e-18, -1.1102230246251565e-16, -3.469446951953614e-18, 5.551115123125783e-17},
[]float64{-0.17850448166863558, -0.06308320274662128, -0.18961604272683463, 0.0076000152667523736, -0.16934243864199913, 0.07056177168778544, -0.19882322049977452},
[]float64{-0.35398371085600466, -0.12595519321255907, -0.3850803048528624, 0.018224416648846196, -0.33745880197156236, 0.14300830147446633, -0.40203716905213466},
[]float64{0.24960888342309973, -0.050767062152303206, -1.041341193522168, 0.07108322594180366, -0.4260707996691682, 0.10617060569468127, -0.3672207720063068},
[]float64{0.1622399531769422, -0.0819548941638717, -1.2541051023544847, 0.09696667505636157, -0.5681441403637507, 0.1450851896100518, -0.5275042399665815},
[]float64{0.14182864487550031, -0.1141746311813289, -1.4908010033196417, 0.13007887015104247, -0.6889306551299612, 0.17960166844026199, -0.6674644097388047},
[]float64{0.3004044058497633, -0.09887941568017214, -1.5791813482679007, 0.2015219828755005, -0.46174484403522364, 0.2234309759898196, -0.827037136532577},
[]float64{0.43401385608381127, -0.05837178704773168, -1.603907266406396, 0.2710624958607913, -0.21622918369537533, 0.2752294256455754, -0.9616207914780164},
[]float64{0.58199460600802, 0.0026539362470191777, -1.5868159123839642, 0.33845697633467264, 0.00799199972447023, 0.3385093253759137, -1.0123598453301792},
[]float64{0.6623472206229041, 0.07334835887630438, -1.5467913740577188, 0.39668088388746764, 0.18866527199428065, 0.40131485178570253, -1.058369011498105},
[]float64{0.6962110664794966, 0.1377088256205983, -1.5121363989602283, 0.4624132462984276, 0.30539561026071405, 0.4732394778705397, -1.0889404383767955},
[]float64{0.4938388056010587, 0.23972176990589453, -1.3370756304992626, 0.5267386039679491, 0.48265228471019894, 0.5208927011247318, -1.2631995314065847},
[]float64{0.6887337101820972, 0.2060803468047055, -1.4940538195494697, 0.5962302982844588, 0.36329393002764593, 0.6115505604287651, -1.1054273728955744},
[]float64{0.8853857972145841, 0.11595356717413362, -1.6759338188690729, 0.661642285478104, 0.18352392382759736, 0.6821385141711714, -0.9343453581820989},
[]float64{1.0818489771414428, 0.010050894662480065, -1.857827888576076, 0.734755642129244, 0.014332590933892487, 0.7376525538928499, -0.7865989514088226},
[]float64{1.2637175868101929, -0.02952391695948204, -2.051041322085078, 0.8090212121235978, -0.03665727846146426, 0.7957142018105133, -0.7618450487557284},
[]float64{1.4659684064489318, -0.03578007601113625, -2.258177250041371, 0.8418748520340865, -0.03784172199969316, 0.8195234840305939, -0.7666866088576683},
[]float64{1.6125770294540054, 0.06286366610380056, -2.1041987242755504, 0.8855819614326498, 0.0681125321257355, 0.9186974084493045, -0.5338624263408369},
[]float64{1.7602121921213703, 0.19281551109536962, -1.9438894257973638, 0.9334152243580319, 0.21168289602687937, 1.0146326655013873, -0.30304030399722004},
[]float64{1.9052636229529567, 0.3249017355039013, -1.790891347548283, 0.9886141747944134, 0.35909627048550735, 1.0895660560014213, -0.06938947387619807},
[]float64{2.05181765218508, 0.4598776980409566, -1.647380964853924, 1.0381282896556236, 0.511052264601541, 1.1309283796001603, 0.16121198496888295},
[]float64{2.1950347501973013, 0.5985438052039634, -1.5134519995741686, 1.1096949200691761, 0.6595219477311005, 1.1630888929250638, 0.3953583059530076},
[]float64{2.341111852437192, 0.7402320566856019, -1.396577730667268, 1.1890187825761163, 0.8027415844420457, 1.1771407544603092, 0.6263232841160185},
[]float64{2.5644245641952685, 0.8247663375469966, -1.338117191541001, 1.1628165719430017, 0.909721636349595, 1.132132996834272, 0.8297336455156122},
[]float64{2.7140437628810443, 0.738658725207892, -1.207153713491964, 1.1191463102489974, 0.8429448136585075, 1.0025373172549457, 1.0752695680917959},
[]float64{2.4680281076943418, 0.7269061886349731, -1.0624383486343925, 1.1650728993190187, 0.8043701904295169, 0.937094996403385, 0.986566138836319},
[]float64{2.1988512590365703, 0.7798780991139577, -0.8935646029684822, 1.2216790052832192, 0.8141391146721496, 0.8535984797450866, 0.8666582148507429},
[]float64{1.9461906221263399, 0.833325326754612, -0.7368549579565583, 1.2699260650661712, 0.8034101277452695, 0.7629392710684565, 0.755298787838703},
[]float64{1.6972100453229801, 0.87664122290806, -0.5796111112757676, 1.3466254828268447, 0.7191567228479467, 0.6932291924075511, 0.7079132906826412},
[]float64{1.714227672556582, 0.8865553510552693, -0.5882340489506896, 1.4574012804355165, 0.6590807557897879, 0.7784301478110053, 0.811194385993179}}

var steps2 = [][]float64{
[]float64{5.082197683525802e-21, -2.6020852139652106e-18, -8.470329472543003e-22, 2.6020852139652106e-18, 5.082197683525802e-21, -4.0657581468206416e-20, 8.470329472543003e-21},
[]float64{-0.2922357756322573, 0.09030547187270056, -0.08240685600717744, 0.04456795140486346, -0.14886415840420358, -0.04628516767734889, -0.2255972668225324},
[]float64{-0.6079839505032247, 0.17977214192603108, -0.1551771574544053, 0.08989109666917068, -0.300532344470554, -0.09282396888325506, -0.4613453697181443},
[]float64{-0.926698136682469, 0.2686781770977384, -0.24128503400146806, 0.13801552664703123, -0.47295889528686264, -0.13892221759340853, -0.690561063453397},
[]float64{-1.0842158072072599, 0.13514972742965187, 0.3655183706193516, 0.23303747465382646, -0.4946212193387884, 0.1150073995303326, -0.22935929814569644},
[]float64{-1.175778590132179, 0.14014356951655277, 0.5308500379150536, 0.2948221301744257, -0.3879625343137206, 0.18726403056289784, -0.26740069087788976},
[]float64{-1.1779924157420631, 0.14193826003050058, 0.6147140502589425, 0.36224758875573065, -0.3268488918816245, 0.2588168301921421, -0.2513791541782518},
[]float64{-1.1678920579721264, 0.14243903975230127, 0.6949810934974331, 0.4306348720732357, -0.28003245627079576, 0.3338997802508795, -0.2125369154912604},
[]float64{-1.1649269947333751, 0.14414193860023763, 0.7856353833877363, 0.4984615452346479, -0.2581526129179782, 0.4087393628754173, -0.14671389899472342},
[]float64{-1.1760127411450476, 0.14789005655482768, 0.8821636344498753, 0.5651740425881386, -0.2479626934606976, 0.4829418168627976, -0.07872647541843482},
[]float64{-1.1257544229210865, 0.19570111919093988, 0.9654889374607503, 0.6276798192880331, -0.3172671978761546, 0.5376297663834774, 0.10550823396748914},
[]float64{-1.1254458635834643, 0.2662432505721215, 1.1278774879381486, 0.8495592511682671, -0.35024846846771807, 0.7636549490089842, 0.24611746904908244},
[]float64{-1.1185917179722373, 0.27314174279981324, 1.1308147218852782, 0.9567686039025124, -0.32703233993370306, 0.8640984158166958, 0.21413218077958868},
[]float64{-1.0171132068711484, 0.2765453857612751, 1.1260800133097755, 1.0040550656340324, -0.3186606974775735, 0.9068538204324437, 0.2941546733352949},
[]float64{-0.8358841634685366, 0.28163358750276096, 1.1256076934638861, 1.0578203176090943, -0.31267296204510664, 0.9567697415433376, 0.4579164923316697},
[]float64{-0.6570726277701862, 0.31794443247426457, 1.1164905430925174, 1.1048746606330073, -0.34208457785565, 0.9893385636655625, 0.6314953470829086},
[]float64{-0.4886331292273004, 0.37804704057057875, 1.128165814327012, 1.1519357293009447, -0.4014869922305723, 1.0203324569135783, 0.828903179315582},
[]float64{-0.30654512505903625, 0.4493718574786155, 1.2150614835191726, 1.220217681650905, -0.4740485316035476, 1.1014509624404054, 1.1006155494630676},
[]float64{-0.30654512505903625, 0.4493718574786155, 1.2150614835191726, 1.220217681650905, -0.4740485316035476, 1.1014509624404054, 1.1006155494630676},
[]float64{-0.09055945065313854, 0.48893581523565616, 1.0134371186171327, 1.2756638417463504, -0.4768207511489623, 1.0527971987300737, 1.1155961769936158},
[]float64{0.11130118806328657, 0.5419598626959136, 0.8248982804007534, 1.3300867676651307, -0.46899464170103433, 0.9946346644334474, 1.1279850182644318},
[]float64{0.2998829654318754, 0.6261198771860563, 1.0216003340026238, 1.4296323892436815, -0.5818286026507044, 1.1400838271846074, 1.49147297829883},
[]float64{0.4335586775326105, 0.6972799577031231, 0.8650699580906399, 1.3246373364566078, -0.6499098083408403, 0.9384386168976093, 1.5883308834864385},
[]float64{0.5847377690275875, 0.7875607985213744, 0.7106761436418133, 1.391905315754722, -0.6572157477067642, 0.8574547234951939, 1.5981369947362214},
[]float64{0.7258105307544123, 0.8783945031751598, 0.5685890754131488, 1.4553725561993567, -0.6375980166767857, 0.7701472626690627, 1.601862507946982},
}
