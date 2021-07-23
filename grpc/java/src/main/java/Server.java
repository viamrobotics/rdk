import com.viam.core.proto.api.v1.Robot;
import com.viam.core.proto.api.v1.Robot.StatusRequest;
import com.viam.core.proto.api.v1.Robot.StatusResponse;
import com.viam.core.proto.api.v1.RobotServiceGrpc;
import io.grpc.ServerBuilder;

import java.io.IOException;

public class Server {
    public static void main(final String[] args) {
        int port = 50051;
        if (args.length > 0) {
            port = Integer.parseInt(args[0]);
        }

        System.out.printf("Serving on localhost:%d\n", port);
        final io.grpc.Server server = ServerBuilder.forPort(port).addService(new RobotService()).build();
        try {
            server.start();
            server.awaitTermination();
        } catch (final IOException | InterruptedException e) {
            e.printStackTrace();
        }
    }

    private static class RobotService extends RobotServiceGrpc.RobotServiceImplBase {
        public void status(final StatusRequest request,
                           final io.grpc.stub.StreamObserver<StatusResponse> responseObserver) {
            final Robot.Status status = Robot.Status.newBuilder().putBases("base1", true).build();
            responseObserver.onNext(StatusResponse.newBuilder().setStatus(status).build());
            responseObserver.onCompleted();
        }
    }
}
