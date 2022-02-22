import com.viam.rdk.proto.api.v1.Robot.StatusRequest;
import com.viam.rdk.proto.api.v1.Robot.StatusResponse;
import com.viam.rdk.proto.api.v1.RobotServiceGrpc;
import com.viam.rdk.proto.api.v1.RobotServiceGrpc.RobotServiceBlockingStub;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;

public class Client {
    public static void main(final String[] args) {
        if (args.length < 1) {
            System.err.println("must supply grpc address");
            return;
        }
        final String grpcAddress = args[0];
        final ManagedChannel channel = ManagedChannelBuilder.forTarget(grpcAddress).usePlaintext().build();
        final RobotServiceBlockingStub client = RobotServiceGrpc.newBlockingStub(channel);
        final StatusResponse resp = client.status(StatusRequest.newBuilder().build());
        System.out.println(resp);
    }
}
