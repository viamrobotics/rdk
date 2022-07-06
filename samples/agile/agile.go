package main

import (
  "context"
  "math"

  "github.com/edaniels/golog"
  "go.viam.com/rdk/grpc/client"
  "go.viam.com/utils"

  utilsrdk "go.viam.com/rdk/utils"
  "go.viam.com/utils/rpc"
  "go.viam.com/rdk/component/base"
  "go.viam.com/rdk/resource"

)

var logger = golog.NewDevelopmentLogger("agile")



func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
  robot, err := client.New(
      context.Background(),
      "agilex-limo-main.60758fe0f6.viam.cloud",
      logger,
      client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
          Type:    utilsrdk.CredentialsTypeRobotLocationSecret,
          Payload: "pem1epjv07fq2cz2z5723gq6ntuyhue5t30boohkiz3iqht4",
      })),
  )
  if err != nil {
      logger.Debug(err)
      return err
  }
  defer robot.Close(context.Background())
  logger.Info("Resources:")
  logger.Info(robot.ResourceNames())

  limo, err := robot.ResourceByName(resource.NameFromSubtype(base.Subtype, "limo"))

  limo1 := limo.(base.Base)

  err = MoveToWaypoint(ctx, limo1, 0, 0, 1, 0)
  if err!=nil {
    logger.Debug(err)
    return err
  }

  return nil
  
}

func MoveToWaypoint(ctx context.Context, limo base.Base, x1 float64, y1 float64, x2 float64, y2 float64) error {
  dir := 0.0  //get direction from state, but assume we're pointing at x direction for rn
  dist := math.Sqrt(math.Pow((y2-y1), 2) + math.Pow((x2-x1), 2))*500  //each grid square is half a meter
  theta := math.Acos((x2-x1)/dist)  //angle to x axis

  moves := base.Move{DistanceMm: int(dist), MmPerSec: 100, AngleDeg: (dir-theta)*(180/math.Pi), DegsPerSec: 20}
  err := base.DoMove(ctx, moves, limo)
  if err!=nil {
    return err
  }

  return nil

}
