# Kinematic model fixtures

One directory of kinematic model files with what a loader has to make of each, shared by the
Go, C++ and Python SDKs so the three loaders agree. The Go runner is
`referenceframe/kinematic_model_fixtures_test.go`. The other SDKs keep a pinned copy of this
directory and run the same checks with their own loader.

## Layout

Every case is a directory holding `model.json`, the kinematic model file under test, with any
meshes it names beside it by relative path. A case ends in exactly one of two ways.

- `expected.json` is the message the loader has to produce, as proto JSON of
  `viam.common.v1.KinematicModel`, with every mesh's bytes inline and its content type in the
  extension spelling. Parse it with the protobuf JSON parser of the SDK and compare messages,
  do not compare text.
- `error.json` says the loader has to refuse the file, with the reason as a `kind` from the table
  below and `mentions`, strings the error message has to contain, usually the id or path at
  fault. The wording of the message is the SDK's own.

A runner walks the directory, loads each `model.json` through the SDK's loader, and checks
whichever file is present. A case with neither is a mistake.

## Expected files

`expected.json` is written by the Go loader. After a deliberate change to the loader, regenerate
with

```
go test ./referenceframe/ -run TestKinematicModelFixtures -update-kinematic-model-fixtures
```

and read the diff, since every SDK compares against the same files. A loader that produces
something else on an existing case is wrong until the fixture is changed on purpose.

Two things the expected files pin that are easy to get wrong. A mesh named by path comes back
with its bytes inline and the path kept as it was written, and a content type is lowercased,
taken from the extension when absent, with `model/gltf-binary` read as `glb`.

## Error kinds

| kind | the file |
|---|---|
| `unknown_field` | has a field the message does not have, usually a typo |
| `infinite` | has an infinite number |
| `nan` | has a number that is not a number |
| `malformed_json` | is not JSON |
| `empty_model` | has no links and no joints |
| `missing_id` | has a link or joint without an id |
| `reserved_id` | uses `world` as an id |
| `shared_id` | uses one id for a link and a joint |
| `duplicate_id` | uses one id twice for the same kind |
| `missing_parent` | names a parent that is not in the model |
| `cycle` | has a link or joint that is its own ancestor |
| `mimic_self` | has a joint that mimics itself |
| `mimic_not_joint` | has a joint that mimics something that is not a joint |
| `mimic_with_limits` | has a mimic joint carrying limits of its own |
| `mesh_missing` | names a mesh file that is not there |
| `mesh_path_absolute` | names a mesh by an absolute path |
| `mesh_path_uri` | names a mesh by a URI, `package://` or `file://` |
| `mesh_path_backslash` | names a mesh with backslashes in the path |
| `content_type_mismatch` | gives a mesh a content type its extension disagrees with |
| `collision_format` | has a collision mesh that is not STL or PLY |
| `mesh_no_extension` | names a mesh by a path with no extension to take the type from |
| `inline_without_type` | has mesh bytes inline with no content type |
| `mesh_empty` | has a mesh with neither bytes nor a path |

## The cases

Handwritten: `single_link`, `orientations` for every orientation form a link pose can take,
`geometries` for every shape a collision or visual can be, `mesh_by_path` for meshes read from
beside the file including a shared path, a `..` in a path and the `model/gltf-binary` alias, and
`prismatic_with_properties` for a prismatic joint with its own geometry, velocity and
acceleration limits and component properties.

Written by `viam kinematics convert`: `dh_converted` from `testfiles/ur5eDH.json`,
`mimic_gripper_converted` from `testfiles/test_mimic_gripper.json`, `urdf_converted` from a URDF
with a collision mesh and a visual referenced through `package://`, and
`continuous_joint_converted` from a URDF with a continuous joint.

Every `error_*` case holds the smallest file that fails for its kind.
