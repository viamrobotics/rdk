package resource

import (
	"testing"

	"go.viam.com/test"
)

func TestNewName(t *testing.T) {
	type testCase struct {
		api          API
		nameString   string
		expectedName Name
	}

	camAPI := API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}
	test.That(t, camAPI, test.ShouldResemble, APINamespaceRDK.WithComponentType("camera"))

	motionAPI := API{Type: APIType{Namespace: APINamespace("rdk"), Name: "service"}, SubtypeName: "motion"}
	test.That(t, motionAPI, test.ShouldResemble, APINamespaceRDK.WithServiceType("motion"))

	tcs := []testCase{
		{
			api:          camAPI,
			nameString:   "cam-1",
			expectedName: Name{API: camAPI, Remote: "", Name: "cam-1"},
		},
		{
			api:        camAPI,
			nameString: "remote:cam-1",

			expectedName: Name{API: camAPI, Remote: "remote", Name: "cam-1"},
		},
		{
			api:        camAPI,
			nameString: "remoteA:remoteB:cam-1",

			expectedName: Name{API: camAPI, Remote: "remoteA:remoteB", Name: "cam-1"},
		},
		{
			api:        motionAPI,
			nameString: "builtin",

			expectedName: Name{API: motionAPI, Remote: "", Name: "builtin"},
		},
		{
			api:        motionAPI,
			nameString: "remote:builtin",

			expectedName: Name{API: motionAPI, Remote: "remote", Name: "builtin"},
		},
		{
			api:        motionAPI,
			nameString: "remoteA:remoteB:builtin",

			expectedName: Name{API: motionAPI, Remote: "remoteA:remoteB", Name: "builtin"},
		},
	}
	for _, tc := range tcs {
		test.That(t, NewName(tc.api, tc.nameString), test.ShouldResemble, tc.expectedName)
	}
}

func TestNewFromString(t *testing.T) {
	type testCase struct {
		string       string
		expectedErr  error
		expectedName Name
	}

	camAPI := API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}
	test.That(t, camAPI, test.ShouldResemble, APINamespaceRDK.WithComponentType("camera"))

	motionAPI := API{Type: APIType{Namespace: APINamespace("rdk"), Name: "service"}, SubtypeName: "motion"}
	test.That(t, motionAPI, test.ShouldResemble, APINamespaceRDK.WithServiceType("motion"))

	tcs := []testCase{
		{
			string:       "rdk:component:camera/cam-1",
			expectedName: Name{API: camAPI, Remote: "", Name: "cam-1"},
		},
		{
			string:       "rdk:component:camera/remote:cam-1",
			expectedName: Name{API: camAPI, Remote: "remote", Name: "cam-1"},
		},
		{
			string:       "rdk:component:camera/remoteA:remoteB:cam-1",
			expectedName: Name{API: camAPI, Remote: "remoteA:remoteB", Name: "cam-1"},
		},
		{
			string:       "rdk:service:motion/builtin",
			expectedName: Name{API: motionAPI, Remote: "", Name: "builtin"},
		},
		{
			string:       "rdk:service:motion/remote:builtin",
			expectedName: Name{API: motionAPI, Remote: "remote", Name: "builtin"},
		},
		{
			string:       "rdk:service:motion/remoteA:remoteB:builtin",
			expectedName: Name{API: motionAPI, Remote: "remoteA:remoteB", Name: "builtin"},
		},
	}
	for _, tc := range tcs {
		name, err := NewFromString(tc.string)
		if tc.expectedErr != nil {
			test.That(t, err, test.ShouldBeError, tc.expectedErr)
		} else {
			test.That(t, err, test.ShouldBeNil)
		}
		test.That(t, name, test.ShouldResemble, tc.expectedName)
	}
}

func TestSDPTrackName(t *testing.T) {
	type testCase struct {
		name   Name
		output string
	}

	camAPI := API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}
	test.That(t, camAPI, test.ShouldResemble, APINamespaceRDK.WithComponentType("camera"))

	tcs := []testCase{
		{
			name:   Name{API: camAPI, Remote: "", Name: "cam-1"},
			output: "cam-1",
		},
		{
			name:   Name{API: camAPI, Remote: "remote", Name: "cam-1"},
			output: "remote+cam-1",
		},
		{
			name:   Name{API: camAPI, Remote: "remoteA:remoteB", Name: "cam-1"},
			output: "remoteA+remoteB+cam-1",
		},
	}
	for _, tc := range tcs {
		test.That(t, tc.name.SDPTrackName(), test.ShouldResemble, tc.output)
	}
}

func TestSDPTrackNameToShortName(t *testing.T) {
	type testCase struct {
		input  string
		output string
	}

	tcs := []testCase{
		{
			input:  "cam-1",
			output: "cam-1",
		},
		{
			input:  "remote+cam-1",
			output: "remote:cam-1",
		},
		{
			input:  "remoteA+remoteB+cam-1",
			output: "remoteA:remoteB:cam-1",
		},
	}
	for _, tc := range tcs {
		test.That(t, SDPTrackNameToShortName(tc.input), test.ShouldResemble, tc.output)
	}
}

func TestNamesToStrings(t *testing.T) {
	type testCase struct {
		input  []Name
		output []string
	}

	camAPI := API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}
	test.That(t, camAPI, test.ShouldResemble, APINamespaceRDK.WithComponentType("camera"))

	tcs := []testCase{
		{
			input:  []Name{},
			output: []string{},
		},
		{
			input:  []Name{{API: camAPI, Remote: "", Name: "cam1"}},
			output: []string{"rdk:component:camera/cam1"},
		},
		{
			input:  []Name{{API: camAPI, Remote: "", Name: "cam1"}, {API: camAPI, Remote: "abc", Name: "cam1"}},
			output: []string{"rdk:component:camera/cam1", "rdk:component:camera/abc:cam1"},
		},
	}
	for _, tc := range tcs {
		test.That(t, NamesToStrings(tc.input), test.ShouldResemble, tc.output)
	}
}
