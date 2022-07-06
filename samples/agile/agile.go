package main

import (
  "context"

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
      logger.Fatal(err)
  }
  defer robot.Close(context.Background())
  logger.Info("Resources:")
  logger.Info(robot.ResourceNames())

  limo, err := robot.ResourceByName(resource.NameFromSubtype(base.Subtype, "limo"))

  limo1 := limo.(base.Base)

  limo1.MoveStraight(ctx, 1000, 100)

  return nil
  
}
