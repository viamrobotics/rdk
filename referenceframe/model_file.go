package referenceframe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ReadKinematicModelFile reads a kinematic model file into the message it encodes. The parse is
// strict, so an unknown field, a non finite number, a parent that is not in the model, a cycle or
// a mimic of something that is not a joint all fail the read. A mesh that names a source path and
// carries no bytes is read from that path, relative to the model file, and the bytes, the content
// type taken from the extension and the path all end up on the message. A mesh that already
// carries bytes is left alone, so a model written out from the wire reads back without its files.
func ReadKinematicModelFile(modelPath string) (*commonpb.KinematicModel, error) {
	//nolint:gosec
	data, err := os.ReadFile(modelPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read kinematic model file")
	}
	pb, err := unmarshalKinematicModel(data, filepath.Dir(modelPath))
	if err != nil {
		return nil, errors.Wrapf(err, "kinematic model file %s", modelPath)
	}
	return pb, nil
}

// unmarshalKinematicModel parses the proto JSON strictly and then checks what the parser cannot,
// that every number is finite, that the tree holds together and that every mesh can be read. An
// empty dir means the caller has bytes only, so a mesh that needs a file is an error.
func unmarshalKinematicModel(data []byte, dir string) (*commonpb.KinematicModel, error) {
	pb := &commonpb.KinematicModel{}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(data, pb); err != nil {
		return nil, err
	}
	if err := rejectNonFinite(pb.ProtoReflect(), ""); err != nil {
		return nil, err
	}
	if err := validateKinematicModel(pb); err != nil {
		return nil, err
	}
	if err := loadMeshes(pb, dir); err != nil {
		return nil, err
	}
	return pb, nil
}

// isKinematicModelDocument tells the proto JSON of a KinematicModel apart from an SVA or DH
// document. The two share id, parent, axis and mimic, so the fields only one of them has decide
// it, and a document with none of either reads the same both ways. Keys are matched without
// regard to case, since the SVA reader never cared. Malformed JSON counts as the message so the
// strict parser, which reports a line and column, is the one to complain.
func isKinematicModelDocument(data []byte) bool {
	var doc struct {
		KinParamType json.RawMessage              `json:"kinematic_param_type"`
		DHParams     json.RawMessage              `json:"dhParams"`
		OriginalFile json.RawMessage              `json:"original_file"`
		Properties   json.RawMessage              `json:"properties"`
		Links        []map[string]json.RawMessage `json:"links"`
		Joints       []map[string]json.RawMessage `json:"joints"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return true
	}
	hasAny := func(obj map[string]json.RawMessage, keys ...string) bool {
		for k := range obj {
			if slices.Contains(keys, strings.ToLower(k)) {
				return true
			}
		}
		return false
	}
	legacy := doc.KinParamType != nil || doc.DHParams != nil || doc.OriginalFile != nil
	message := doc.Properties != nil
	for _, link := range doc.Links {
		legacy = legacy || hasAny(link, "translation", "orientation", "geometry", "geometries")
		message = message || hasAny(link, "pose", "collision", "visual")
	}
	for _, joint := range doc.Joints {
		legacy = legacy || hasAny(joint, "min", "max", "max_velocity", "max_acceleration")
		message = message || hasAny(joint, "limits")
		var jointType string
		if raw, ok := lookup(joint, "type"); ok && json.Unmarshal(raw, &jointType) == nil && jointType != "" {
			if strings.HasPrefix(jointType, "JOINT_TYPE_") {
				message = true
			} else {
				legacy = true
			}
		}
	}
	return message || !legacy
}

func lookup(obj map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	for k, v := range obj {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

// rejectNonFinite walks every number in the message, since proto JSON reads the strings
// "Infinity", "-Infinity" and "NaN" into a double and a file has to leave a bound out to say
// unbounded.
func rejectNonFinite(m protoreflect.Message, prefix string) error {
	var err error
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := prefix + string(fd.Name())
		switch {
		case fd.IsList():
			list := v.List()
			for i := 0; i < list.Len() && err == nil; i++ {
				err = checkFinite(fd, list.Get(i), fmt.Sprintf("%s[%d]", name, i))
			}
		case fd.IsMap():
			v.Map().Range(func(k protoreflect.MapKey, mv protoreflect.Value) bool {
				err = checkFinite(fd.MapValue(), mv, fmt.Sprintf("%s[%v]", name, k.Interface()))
				return err == nil
			})
		default:
			err = checkFinite(fd, v, name)
		}
		return err == nil
	})
	return err
}

func checkFinite(fd protoreflect.FieldDescriptor, v protoreflect.Value, name string) error {
	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return rejectNonFinite(v.Message(), name+".")
	}
	if fd.Kind() != protoreflect.DoubleKind && fd.Kind() != protoreflect.FloatKind {
		return nil
	}
	if f := v.Float(); math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Errorf("%s is %v, leave a bound out to make it unbounded", name, f)
	}
	return nil
}

// validateKinematicModel checks the references a file author can get wrong. The frame system
// builder treats a parent it does not know as the world, so a misspelled parent has to be caught
// here or the link quietly moves to the root.
func validateKinematicModel(pb *commonpb.KinematicModel) error {
	type node struct{ kind, id, parent string }
	nodes := make([]node, 0, len(pb.GetLinks())+len(pb.GetJoints()))
	for _, link := range pb.GetLinks() {
		nodes = append(nodes, node{"link", link.GetId(), link.GetParent()})
	}
	for _, joint := range pb.GetJoints() {
		nodes = append(nodes, node{"joint", joint.GetId(), joint.GetParent()})
	}
	if len(nodes) == 0 {
		return errors.New("model has no links or joints")
	}
	kinds := make(map[string]string, len(nodes))
	parents := make(map[string]string, len(nodes))
	for _, n := range nodes {
		switch {
		case n.id == "":
			return fmt.Errorf("a %s has no id", n.kind)
		case n.id == World:
			return NewReservedWordError(n.kind, World)
		}
		if other, dup := kinds[n.id]; dup {
			if other == n.kind {
				return fmt.Errorf("%s %q is defined twice", n.kind, n.id)
			}
			return fmt.Errorf("%q names both a %s and a %s", n.id, other, n.kind)
		}
		kinds[n.id] = n.kind
		parents[n.id] = n.parent
	}
	for _, n := range nodes {
		if n.parent == "" || n.parent == World {
			continue
		}
		if _, ok := kinds[n.parent]; !ok {
			return fmt.Errorf("%s %q: parent %q is not a link or joint in the model", n.kind, n.id, n.parent)
		}
	}
	for _, n := range nodes {
		seen := map[string]bool{}
		for cur := n.id; cur != "" && cur != World; cur = parents[cur] {
			if seen[cur] {
				return fmt.Errorf("%s %q is its own ancestor", kinds[cur], cur)
			}
			seen[cur] = true
		}
	}
	for _, joint := range pb.GetJoints() {
		mimic := joint.GetMimic()
		if mimic == nil {
			continue
		}
		if mimic.GetJoint() == joint.GetId() {
			return fmt.Errorf("joint %q mimics itself", joint.GetId())
		}
		if kinds[mimic.GetJoint()] != "joint" {
			return fmt.Errorf("joint %q mimics %q, which is not a joint in the model", joint.GetId(), mimic.GetJoint())
		}
		// the reader drops the limits of a mimic joint, so a file that writes them is told, as an
		// SVA file is
		if l := joint.GetLimits(); l != nil && (l.Min != nil || l.Max != nil || l.MaxVelocity != nil || l.MaxAcceleration != nil) {
			return fmt.Errorf("%w: joint %q", ErrMimicWithLimits, joint.GetId())
		}
	}
	return nil
}

// loadMeshes fills in every mesh that names a file. A path is read once, so the same file on two
// links is one read. A collision mesh has to be in a format the frame system can parse, while a
// visual mesh is never parsed here, so any format with an extension goes. Content types are
// matched without regard to case and written back in the extension style the message documents,
// so stl, ply and glb, with the MIME spelling of glb accepted.
func loadMeshes(pb *commonpb.KinematicModel, dir string) error {
	cache := map[string][]byte{}
	for _, link := range pb.GetLinks() {
		for i, g := range link.GetCollision() {
			if err := loadMesh(g.GetMesh(), true, dir, cache); err != nil {
				return errors.Wrapf(err, "link %q collision geometry %d", link.GetId(), i)
			}
		}
		for i, g := range link.GetVisual() {
			if err := loadMesh(g.GetMesh(), false, dir, cache); err != nil {
				return errors.Wrapf(err, "link %q visual geometry %d", link.GetId(), i)
			}
		}
	}
	for _, joint := range pb.GetJoints() {
		if err := loadMesh(joint.GetGeometry().GetMesh(), true, dir, cache); err != nil {
			return errors.Wrapf(err, "joint %q geometry", joint.GetId())
		}
	}
	return nil
}

func loadMesh(mesh *commonpb.Mesh, collision bool, dir string, cache map[string][]byte) error {
	if mesh == nil {
		return nil
	}
	source := mesh.GetOriginalFileSourcePathDiagnostic()
	hasBytes := len(mesh.GetMesh()) > 0
	if !hasBytes && source == "" {
		return errors.New("mesh has neither bytes nor a source path")
	}
	contentType := strings.ToLower(mesh.GetContentType())
	if contentType == "model/gltf-binary" {
		// the MIME spelling the viewer sends today, until it moves to the extension style
		contentType = "glb"
	}
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(source)), ".")
	switch {
	case !hasBytes:
		// a mesh read from a file takes its content type from the extension, and the path has to
		// survive a module install and a trip over the wire
		if err := checkMeshPath(source); err != nil {
			return err
		}
		if ext == "" {
			return fmt.Errorf("mesh path %q has no extension to take the content type from", source)
		}
		if contentType != "" && contentType != ext {
			return fmt.Errorf("mesh content type %q disagrees with the extension of %q", mesh.GetContentType(), source)
		}
		contentType = ext
	case contentType == "":
		// a mesh that carries its bytes is not read again and its path is whatever it was when
		// it was written out, which can still name the content type
		contentType = ext
	}
	if contentType == "" {
		return errors.New("inline mesh bytes need a content type")
	}
	if collision && contentType != "stl" && contentType != "ply" {
		return fmt.Errorf("collision mesh content type %q is not stl or ply", contentType)
	}
	mesh.ContentType = contentType
	if hasBytes {
		return nil
	}
	if data, ok := cache[source]; ok {
		mesh.Mesh = data
		return nil
	}
	if dir == "" {
		return fmt.Errorf("mesh %q cannot be read without the directory of the model file", source)
	}
	resolved := filepath.Join(dir, filepath.FromSlash(source))
	//nolint:gosec
	data, err := os.ReadFile(resolved)
	if err != nil {
		return fmt.Errorf("failed to read mesh %q at %s: %w", source, resolved, err)
	}
	mesh.Mesh = data
	cache[source] = data
	return nil
}

// checkMeshPath rejects a source path that would not survive a module install or a trip over the
// wire. A path is relative to the model file, uses forward slashes and is not a URI.
func checkMeshPath(source string) error {
	switch {
	case strings.Contains(source, "://"):
		return fmt.Errorf("mesh path %q is a URI, paths are relative to the model file", source)
	case strings.Contains(source, `\`):
		return fmt.Errorf("mesh path %q uses backslashes, use forward slashes", source)
	case path.IsAbs(source) || filepath.IsAbs(source):
		return fmt.Errorf("mesh path %q is absolute, paths are relative to the model file", source)
	}
	return nil
}

// WriteKinematicModelFile writes the message as a kinematic model file at modelPath, with every
// mesh that carries bytes written beside it and named by a path relative to the file, under
// meshes/<file name>/collision or meshes/<file name>/visual, so the file reads back through
// ReadKinematicModelFile with nothing inline. A mesh keeps the base name of the source path it
// came with, or is named for its link when it has none, with its content type as the extension,
// and a name already taken by different bytes gets a number. The same bytes under the same name
// are written once. Links and joints go out in the order a walk from world meets them, siblings
// by id, and the JSON carries the proto field names indented the same way every time, so a model
// writes the same file however it was built and a converted file diffs cleanly. The message
// passed in is left as it was.
func WriteKinematicModelFile(modelPath string, pb *commonpb.KinematicModel) error {
	if pb == nil {
		return errors.New("cannot write a nil kinematic model")
	}
	out, ok := proto.Clone(pb).(*commonpb.KinematicModel)
	if !ok {
		return errors.New("cannot write a nil kinematic model")
	}
	orderByTree(out)
	base := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))
	w := &meshWriter{dir: filepath.Dir(modelPath), meshDir: path.Join("meshes", base), written: map[string][]byte{}}
	for _, link := range out.GetLinks() {
		for i, g := range link.GetCollision() {
			if err := w.write(g.GetMesh(), "collision", fmt.Sprintf("%s_collision_%d", link.GetId(), i)); err != nil {
				return errors.Wrapf(err, "link %q collision geometry %d", link.GetId(), i)
			}
		}
		for i, g := range link.GetVisual() {
			if err := w.write(g.GetMesh(), "visual", fmt.Sprintf("%s_visual_%d", link.GetId(), i)); err != nil {
				return errors.Wrapf(err, "link %q visual geometry %d", link.GetId(), i)
			}
		}
	}
	for _, joint := range out.GetJoints() {
		if err := w.write(joint.GetGeometry().GetMesh(), "collision", joint.GetId()+"_geometry"); err != nil {
			return errors.Wrapf(err, "joint %q geometry", joint.GetId())
		}
	}
	raw, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(out)
	if err != nil {
		return err
	}
	// protojson varies its whitespace on purpose, so the indentation is applied afterwards
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')
	//nolint:gosec
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return err
	}
	//nolint:gosec
	return os.WriteFile(modelPath, buf.Bytes(), 0o644)
}

type meshWriter struct {
	dir     string
	meshDir string
	written map[string][]byte
}

// write puts the mesh's bytes in a file beside the model and leaves the mesh pointing at it.
func (w *meshWriter) write(mesh *commonpb.Mesh, role, fallback string) error {
	if len(mesh.GetMesh()) == 0 {
		return nil
	}
	source := filepath.ToSlash(mesh.GetOriginalFileSourcePathDiagnostic())
	ext := strings.ToLower(mesh.GetContentType())
	if ext == "model/gltf-binary" {
		ext = "glb"
	}
	if ext == "" {
		ext = strings.TrimPrefix(strings.ToLower(path.Ext(source)), ".")
	}
	if ext == "" {
		return errors.New("mesh has no content type to name its file by")
	}
	name := fallback
	if source != "" {
		name = strings.TrimSuffix(path.Base(source), path.Ext(source))
	}
	rel := path.Join(w.meshDir, role, name+"."+ext)
	for n := 2; ; n++ {
		have, taken := w.written[rel]
		if !taken || bytes.Equal(have, mesh.GetMesh()) {
			break
		}
		rel = path.Join(w.meshDir, role, fmt.Sprintf("%s_%d.%s", name, n, ext))
	}
	if _, done := w.written[rel]; !done {
		full := filepath.Join(w.dir, filepath.FromSlash(rel))
		//nolint:gosec
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		//nolint:gosec
		if err := os.WriteFile(full, mesh.GetMesh(), 0o644); err != nil {
			return err
		}
		w.written[rel] = mesh.GetMesh()
	}
	mesh.OriginalFileSourcePathDiagnostic = proto.String(rel)
	mesh.ContentType = ext
	mesh.Mesh = nil
	return nil
}

// orderByTree puts links and joints in the order a walk from world meets them, siblings by id.
// Anything the walk cannot reach, which the loader will refuse, keeps its place at the end in
// the order it came.
func orderByTree(pb *commonpb.KinematicModel) {
	type node struct {
		id    string
		link  *commonpb.KinematicLink
		joint *commonpb.KinematicJoint
	}
	children := map[string][]node{}
	for _, link := range pb.GetLinks() {
		children[link.GetParent()] = append(children[link.GetParent()], node{id: link.GetId(), link: link})
	}
	for _, joint := range pb.GetJoints() {
		children[joint.GetParent()] = append(children[joint.GetParent()], node{id: joint.GetId(), joint: joint})
	}
	for _, siblings := range children {
		slices.SortFunc(siblings, func(a, b node) int { return strings.Compare(a.id, b.id) })
	}

	seenLink := map[*commonpb.KinematicLink]bool{}
	seenJoint := map[*commonpb.KinematicJoint]bool{}
	var links []*commonpb.KinematicLink
	var joints []*commonpb.KinematicJoint
	queue := append(slices.Clone(children[World]), children[""]...)
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		switch {
		case n.link != nil && !seenLink[n.link]:
			seenLink[n.link] = true
			links = append(links, n.link)
		case n.joint != nil && !seenJoint[n.joint]:
			seenJoint[n.joint] = true
			joints = append(joints, n.joint)
		default:
			continue
		}
		queue = append(queue, children[n.id]...)
	}
	for _, link := range pb.GetLinks() {
		if !seenLink[link] {
			links = append(links, link)
		}
	}
	for _, joint := range pb.GetJoints() {
		if !seenJoint[joint] {
			joints = append(joints, joint)
		}
	}
	pb.Links, pb.Joints = links, joints
}
