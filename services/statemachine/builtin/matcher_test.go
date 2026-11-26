package builtin

import (
	"testing"

	"go.viam.com/test"
)

func TestMatch(t *testing.T) {
	readings := map[string]interface{}{"pressed": true, "confidence": 0.9, "name": "empty_glass", "n": 3}
	cases := []struct {
		name  string
		cond  interface{}
		value interface{}
		want  bool
	}{
		{"empty matches anything", map[string]interface{}{}, readings, true},
		{"equality", map[string]interface{}{"pressed": true}, readings, true},
		{"inequality", map[string]interface{}{"pressed": false}, readings, false},
		{"missing key", map[string]interface{}{"nope": true}, readings, false},
		{"numeric across types", map[string]interface{}{"n": 3.0}, readings, true},
		{"gt", map[string]interface{}{"confidence": map[string]interface{}{"gt": 0.8}}, readings, true},
		{"gt false", map[string]interface{}{"confidence": map[string]interface{}{"gt": 0.95}}, readings, false},
		{"lte", map[string]interface{}{"n": map[string]interface{}{"lte": 3}}, readings, true},
		{"ne", map[string]interface{}{"name": map[string]interface{}{"ne": "x"}}, readings, true},
		{"in", map[string]interface{}{"name": map[string]interface{}{"in": []interface{}{"a", "empty_glass"}}}, readings, true},
		{"contains", map[string]interface{}{"name": map[string]interface{}{"contains": "glass"}}, readings, true},
		{"exists", map[string]interface{}{
			"name": map[string]interface{}{"exists": true}, "zzz": map[string]interface{}{"exists": false},
		}, readings, true},
		{"not", map[string]interface{}{"pressed": map[string]interface{}{"not": true}}, readings, false},
		{
			"any over list",
			map[string]interface{}{"any": map[string]interface{}{"class_name": "empty_glass", "confidence": map[string]interface{}{"gt": 0.8}}},
			[]interface{}{
				map[string]interface{}{"class_name": "full_glass", "confidence": 0.99},
				map[string]interface{}{"class_name": "empty_glass", "confidence": 0.85},
			},
			true,
		},
		{
			"any over first list field of a map",
			map[string]interface{}{"any": map[string]interface{}{"class_name": "empty_glass"}},
			map[string]interface{}{"detections": []interface{}{map[string]interface{}{"class_name": "empty_glass"}}},
			true,
		},
		{
			"any none match",
			map[string]interface{}{"any": map[string]interface{}{"class_name": "empty_glass"}},
			[]interface{}{map[string]interface{}{"class_name": "full_glass"}},
			false,
		},
		{
			"all",
			map[string]interface{}{"all": map[string]interface{}{"ok": true}},
			[]interface{}{map[string]interface{}{"ok": true}, map[string]interface{}{"ok": false}},
			false,
		},
		{"scalar cond", true, true, true},
		{"typed map value", map[string]interface{}{"a": 1}, map[string]int{"a": 1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, Match(tc.cond, tc.value), test.ShouldEqual, tc.want)
		})
	}
}

func TestResolveRefs(t *testing.T) {
	input := map[string]interface{}{"pour_ml": 150.0, "pose": map[string]interface{}{"x": 1.0}}
	got := resolveRefs(map[string]interface{}{
		"cmd":  "pour",
		"ml":   "$ctx.pour_ml",
		"x":    "$ctx.pose.x",
		"list": []interface{}{"$ctx.pour_ml", "plain"},
		"none": "$ctx.missing",
	}, input)
	test.That(t, got, test.ShouldResemble, map[string]interface{}{
		"cmd":  "pour",
		"ml":   150.0,
		"x":    1.0,
		"list": []interface{}{150.0, "plain"},
		"none": nil,
	})
}
