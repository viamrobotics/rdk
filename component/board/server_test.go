package board_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testBoardName    = "board1"
	fakeBoardName    = "board2"
	missingBoardName = "board3"
)

func newServer() (pb.BoardServiceServer, *inject.Board, error) {
	injectBoard := &inject.Board{}
	boards := map[resource.Name]interface{}{
		board.Named(testBoardName): injectBoard,
		board.Named(fakeBoardName): "notBoard",
	}
	boardSvc, err := subtype.New(boards)
	if err != nil {
		return nil, nil, err
	}
	return board.NewServer(boardSvc), injectBoard, nil
}

func TestServer(t *testing.T) {
	boardServer, injectBoard, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	injectBoard.GPIOSetFunc = func(ctx context.Context, pin string, high bool) error {
		return nil
	}

	//nolint:dupl
	t.Run("Board GPIO set", func(t *testing.T) {
		_, err := boardServer.GPIOSet(context.Background(), &pb.BoardServiceGPIOSetRequest{Name: testBoardName})

		test.That(t, err, test.ShouldBeNil)

		_, err = boardServer.GPIOSet(context.Background(), &pb.BoardServiceGPIOSetRequest{Name: fakeBoardName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not an Board")

		_, err = boardServer.GPIOSet(context.Background(), &pb.BoardServiceGPIOSetRequest{Name: missingBoardName})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no Board")
	})

	//nolint:dupl
	// TODO remaining board server methods
}
