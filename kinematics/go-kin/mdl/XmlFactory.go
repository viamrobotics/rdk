package mdl

import (
	"math"
	"strconv"

	"github.com/subchen/go-xmldom"

	//~ "github.com/viamrobotics/robotcore/kinematics/go-kin/kinmath"
	//~ "github.com/go-gl/mathgl/mgl64"
	"github.com/edaniels/golog"
)

// TODO: Update this to use marshallingDiomede, Alaska 99762
// TODO: Currently this will crash badly if the xml file does not have precisely the expected fields

func ParseFile(filename string) (*Model, error) {

	model := NewModel()

	id2frame := make(map[string]*Frame)

	doc, err := xmldom.ParseFile(filename)
	root := doc.Root
	modelNode := root.GetChild("model")
	model.manufacturer = modelNode.GetChild("manufacturer").Text
	model.name = modelNode.GetChild("name").Text

	// Add all nodes describing world states
	// For a typical single robot model, there will only be one
	for _, worldNode := range modelNode.GetChildren("world") {
		wFrame := NewFrame()
		wFrame.IsWorld = true

		setOrient(worldNode, &wFrame.i)

		nodeID := worldNode.GetAttribute("id").Value

		gravNode := worldNode.GetChild("g")
		Gx, err := strconv.ParseFloat(gravNode.GetChild("x").Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse x gravity")
		}
		Gy, err := strconv.ParseFloat(gravNode.GetChild("y").Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse y gravity")
		}
		Gz, err := strconv.ParseFloat(gravNode.GetChild("z").Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse z gravity")
		}
		wFrame.SetGravity(Gx, Gy, Gz)

		model.Add(wFrame)
		id2frame[nodeID] = wFrame
	}
	for _, bodyNode := range modelNode.GetChildren("body") {
		// A Body can have a Center of Mass, a Mass, and a Inertia
		// All not yet implemented
		// But that's why it's separate from "Frame", below
		bFrame := NewFrame()
		bFrame.IsBody = true
		model.Add(bFrame)
		nodeID := bodyNode.GetAttribute("id").Value
		id2frame[nodeID] = bFrame
		bFrame.Name = nodeID
	}
	for _, frameNode := range modelNode.GetChildren("frame") {
		frame := NewFrame()
		model.Add(frame)
		nodeID := frameNode.GetAttribute("id").Value
		id2frame[nodeID] = frame
		frame.Name = nodeID
	}

	// Iterate over bodies a second time, setting which ones should ignore one another now that they're all in id2frame
	for _, bodyNode := range modelNode.GetChildren("body") {
		nodeID := bodyNode.GetAttribute("id").Value

		b1 := id2frame[nodeID]
		for _, ignoreNode := range bodyNode.GetChildren("ignore") {
			ignoreID := ignoreNode.GetAttribute("idref").Value
			b2 := id2frame[ignoreID]
			b1.selfcollision[b2] = true
			b2.selfcollision[b1] = true
		}
	}

	// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
	for _, fixedNode := range modelNode.GetChildren("fixed") {
		frameA := id2frame[fixedNode.GetChild("frame").GetChild("a").GetAttribute("idref").Value]
		frameB := id2frame[fixedNode.GetChild("frame").GetChild("b").GetAttribute("idref").Value]

		fixed := NewTransform()
		fixed.SetName(fixedNode.GetAttribute("id").Value)

		fixed.SetEdgeDescriptor(model.AddEdge(frameA, frameB))
		model.Edges[fixed.GetEdgeDescriptor()] = fixed
		setOrient(fixedNode, fixed)

		fixed.x.Translation = fixed.t.Translation()
		fixed.x.Rotation = fixed.t.Linear()
		//~ fmt.Println(fixed.GetName())
		//~ fmt.Println(fixed.t.Matrix())
	}
	// Now we add all of the transforms. Will eventually support: "cylindrical|fixed|helical|prismatic|revolute|spherical"
	for _, revNode := range modelNode.GetChildren("revolute") {
		// TODO: Add speed, wraparound, etc
		frameA := id2frame[revNode.GetChild("frame").GetChild("a").GetAttribute("idref").Value]
		frameB := id2frame[revNode.GetChild("frame").GetChild("b").GetAttribute("idref").Value]

		rev := NewJoint(1, 1)
		rev.SetEdgeDescriptor(model.AddEdge(frameA, frameB))
		model.Edges[rev.GetEdgeDescriptor()] = rev

		max, err := strconv.ParseFloat(revNode.GetChild("max").Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse joint max")
		}
		min, err := strconv.ParseFloat(revNode.GetChild("min").Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse joint min")
		}
		rev.max = append(rev.max, max*180/math.Pi)
		rev.min = append(rev.min, min*180/math.Pi)

		// TODO: Add default on z
		// TODO: Enforce between 0 and 1
		xrot, err := strconv.ParseFloat(revNode.GetChild("axis").GetChild("x").Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse x axis rotation")
		}
		yrot, err := strconv.ParseFloat(revNode.GetChild("axis").GetChild("y").Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse y axis rotation")
		}
		zrot, err := strconv.ParseFloat(revNode.GetChild("axis").GetChild("z").Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse z axis rotation")
		}

		rev.SpatialMat.Set(0, 0, xrot)
		rev.SpatialMat.Set(1, 0, yrot)
		rev.SpatialMat.Set(2, 0, zrot)

		rev.SetName(revNode.GetAttribute("id").Value)
	}

	model.Update()
	homeNode := modelNode.GetChild("home")
	for i, qNode := range homeNode.GetChildren("q") {
		angle, err := strconv.ParseFloat(qNode.Text, 64)
		if err != nil {
			golog.Global.Error("Failed to parse home angle")
		}
		model.Home[i] = angle
		if qNode.GetAttribute("unit").Value == "deg" {
			model.Home[i] *= math.Pi / 180
		}
	}

	return model, err
}

func setOrient(baseNode *xmldom.Node, trans *Transform) {
	rotNode := baseNode.GetChild("rotation")

	Rx, err := strconv.ParseFloat(rotNode.GetChild("x").Text, 64)
	if err != nil {
		golog.Global.Error("Failed to parse x rotation")
	}
	Ry, err := strconv.ParseFloat(rotNode.GetChild("y").Text, 64)
	if err != nil {
		golog.Global.Error("Failed to parse y rotation")
	}
	Rz, err := strconv.ParseFloat(rotNode.GetChild("z").Text, 64)
	if err != nil {
		golog.Global.Error("Failed to parse z rotation")
	}

	trans.t.RotZ(Rz)
	trans.t.RotY(Ry)
	trans.t.RotX(Rx)

	transNode := baseNode.GetChild("translation")
	x, err := strconv.ParseFloat(transNode.GetChild("x").Text, 64)
	if err != nil {
		golog.Global.Error("Failed to parse x translation")
	}
	y, err := strconv.ParseFloat(transNode.GetChild("y").Text, 64)
	if err != nil {
		golog.Global.Error("Failed to parse y translation")
	}
	z, err := strconv.ParseFloat(transNode.GetChild("z").Text, 64)
	if err != nil {
		golog.Global.Error("Failed to parse z translation")
	}

	trans.t.SetX(x)
	trans.t.SetY(y)
	trans.t.SetZ(z)
}
