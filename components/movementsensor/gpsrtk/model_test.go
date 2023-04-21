package gpsrtk_test

import (
	"context"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor/gpsrtk"
	"go.viam.com/rdk/config"
)

func TestGPSModels(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	t.Run("rover", func(t *testing.T) {
		cfg1 := `{
		"components": [
			{
				"model": "gps-rtk",
				"name": "rover",
				"type": "movement_sensor",
				"attributes": {
					"correction_source": "something"
				}
			}
					]
		}`
		_, err := config.FromReader(ctx, "", strings.NewReader(cfg1), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, gpsrtk.ErrRoverValidation.Error())
	})

	t.Run("station", func(t *testing.T) {
		cfg2 := `{
			"components": [ 
				{
				"model": "rtk-station",
				"name": "station",
				"type": "movement_sensor",
				"attributes": {
					"correction_source": "something"
				}
			}
		]
		}`
		_, err := config.FromReader(ctx, "", strings.NewReader(cfg2), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, gpsrtk.ErrStationValidation.Error())
	})
}
