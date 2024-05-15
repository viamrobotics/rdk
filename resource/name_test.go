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
	tcs := []testCase{
		{
			api:        APINamespaceRDK.WithComponentType("camera"),
			nameString: "cam-1",

			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "", Name: "cam-1"},
		},
		{
			api:        APINamespaceRDK.WithComponentType("camera"),
			nameString: "remote:cam-1",

			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "remote", Name: "cam-1"},
		},
		{
			api:        APINamespaceRDK.WithComponentType("camera"),
			nameString: "remoteA:remoteB:cam-1",

			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "remoteA:remoteB", Name: "cam-1"},
		},
		{
			api:        APINamespaceRDK.WithServiceType("motion"),
			nameString: "builtin",

			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "service"}, SubtypeName: "motion"}, Remote: "", Name: "builtin"},
		},
		{
			api:        APINamespaceRDK.WithServiceType("motion"),
			nameString: "remote:builtin",

			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "service"}, SubtypeName: "motion"}, Remote: "remote", Name: "builtin"},
		},
		{
			api:        APINamespaceRDK.WithServiceType("motion"),
			nameString: "remoteA:remoteB:builtin",

			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "service"}, SubtypeName: "motion"}, Remote: "remoteA:remoteB", Name: "builtin"},
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
	tcs := []testCase{
		{
			string:       "rdk:component:camera/cam-1",
			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "", Name: "cam-1"},
		},
		{
			string:       "rdk:component:camera/remote:cam-1",
			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "remote", Name: "cam-1"},
		},
		{
			string:       "rdk:component:camera/remoteA:remoteB:cam-1",
			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "remoteA:remoteB", Name: "cam-1"},
		},
		{
			string:       "rdk:service:motion/builtin",
			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "service"}, SubtypeName: "motion"}, Remote: "", Name: "builtin"},
		},
		{
			string:       "rdk:service:motion/remote:builtin",
			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "service"}, SubtypeName: "motion"}, Remote: "remote", Name: "builtin"},
		},
		{
			string:       "rdk:service:motion/remoteA:remoteB:builtin",
			expectedName: Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "service"}, SubtypeName: "motion"}, Remote: "remoteA:remoteB", Name: "builtin"},
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
	tcs := []testCase{
		{
			name:   Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "", Name: "cam-1"},
			output: "cam-1",
		},
		{
			name:   Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "remote", Name: "cam-1"},
			output: "remote+cam-1",
		},
		{
			name:   Name{API: API{Type: APIType{Namespace: APINamespace("rdk"), Name: "component"}, SubtypeName: "camera"}, Remote: "remoteA:remoteB", Name: "cam-1"},
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
