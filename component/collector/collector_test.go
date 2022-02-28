package collector

//func TestReflection(t *testing.T) {
//	logger := golog.NewTestLogger(t)
//	listener1, err := net.Listen("tcp", "localhost:0")
//	test.That(t, err, test.ShouldBeNil)
//
//	cameraClient, err := camera.NewClient(context.Background(), "testCameraName", listener1.Addr().String(), logger)
//
//	sut := Collector{
//		lock:     &sync.Mutex{},
//		metadata: Metadata{
//			component:   "camera",
//			method:      "",
//			destination: nil,
//			params:      nil,
//		},
//		client:   nil,
//		queue:    nil,
//	}
//}
