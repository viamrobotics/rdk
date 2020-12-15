package stream

// TODO: mem management with Ref, Deref, and cleanup?? REFACTOR AND CLEAN; make webpage auto connect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

type RemoteView interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Ready() <-chan struct{}
	InputFrames() chan<- image.Image // TODO(erd): does duration of frame matter?
	SetOnClickHandler(func(x, y int))
}

func NewRemoteView(config RemoteViewConfig) (RemoteView, error) {
	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config.WebRTCConfig)
	if err != nil {
		panic(err)
	}

	//	TODO(erd): MIME type should be configurable
	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: "video/vp8"},
		"video",
		"pion",
	)
	if err != nil {
		return nil, err
	}

	if _, err := peerConnection.AddTrack(videoTrack); err != nil {
		return nil, err
	}

	dcID := uint16(0)
	dc, err := peerConnection.CreateDataChannel("stuff", &webrtc.DataChannelInit{ID: &dcID})
	if err != nil {
		return nil, err
	}

	return &basicRemoteView{
		config:         config,
		readyCh:        make(chan struct{}),
		peerConnection: peerConnection,
		videoTrack:     videoTrack,
		dataChannel:    dc,
		inputFrames:    make(chan image.Image),
		outputFrames:   make(chan []byte),
	}, nil
}

type basicRemoteView struct {
	mu             sync.Mutex
	config         RemoteViewConfig
	readyCh        chan struct{}
	peerConnection *webrtc.PeerConnection
	videoTrack     *webrtc.TrackLocalStaticSample
	dataChannel    *webrtc.DataChannel
	inputFrames    chan image.Image
	outputFrames   chan []byte
	encoder        *VPXEncoder
	onClickHandler func(x, y int)
}

func (brv *basicRemoteView) Ready() <-chan struct{} {
	return brv.readyCh
}

// TODO(erd): implement me
func (brv *basicRemoteView) Stop(ctx context.Context) error {
	return nil
}

func (brv *basicRemoteView) SetOnClickHandler(handler func(x, y int)) {
	brv.mu.Lock()
	defer brv.mu.Unlock()
	brv.onClickHandler = handler
}

func (brv *basicRemoteView) InputFrames() chan<- image.Image {
	return brv.inputFrames
}

func (brv *basicRemoteView) processInputFrames() {
	firstFrame := true
	for frame := range brv.inputFrames {
		if firstFrame {
			bounds := frame.Bounds()
			if err := brv.initCodec(bounds.Dx(), bounds.Dy()); err != nil {
				panic(err) // TODO(erd): log and maybe do not fail
			}
			firstFrame = false
		}

		encodedFrame, err := brv.encoder.Encode(frame)
		if err != nil {
			panic(err) // TODO(erd): log and maybe do not fail
		}
		if encodedFrame != nil {
			brv.outputFrames <- encodedFrame
		}
	}
}

// TODO(erd): refactor and move out unncessary (panickable especially) parts
func (brv *basicRemoteView) processOutputFrames() {
	// Wait for connection established

	brv.dataChannel.OnOpen(func() {
		for {
			time.Sleep(time.Second)
			// println("SEND TEXT")
			if err := brv.dataChannel.SendText("hello"); err != nil {
				panic(err)
			}
			// println("SENT TEXT")
		}
	})
	brv.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		coords := strings.Split(string(msg.Data), ",")
		if len(coords) != 2 {
			panic(len(coords))
		}
		x, err := strconv.ParseFloat(coords[0], 32)
		if err != nil {
			panic(err)
		}
		y, err := strconv.ParseFloat(coords[1], 32)
		if err != nil {
			panic(err)
		}
		brv.onClickHandler(int(x), int(y)) // handler should return fast otherwise it could block
	})

	// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
	// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
	framesSent := 0
	for outputFrame := range brv.outputFrames {
		// now := time.Now()
		if ivfErr := brv.videoTrack.WriteSample(media.Sample{Data: outputFrame, Duration: time.Second}); ivfErr != nil {
			panic(ivfErr)
		}
		framesSent++
		// fmt.Println(framesSent, "write sample took", time.Since(now))
	}
}

func (brv *basicRemoteView) initCodec(width, height int) error {
	if brv.encoder != nil {
		return errors.New("already initialized codec")
	}

	// TODO(erd): Codec configurable
	var err error
	brv.encoder, err = NewVPXEncoder(CodecVP8, width, height)
	return err
}

func (brv *basicRemoteView) Start(ctx context.Context) error {
	iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(ctx)

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	brv.peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	// Wait for the offer to be submitted
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", brv.config.NegotiationConfig.Port),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	mux := http.NewServeMux()
	httpServer.Handler = mux
	offer := webrtc.SessionDescription{}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(viewHTML))
	})
	mux.HandleFunc("/offer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		reader := bufio.NewReader(r.Body)

		var in string
		for {
			var err error
			in, err = reader.ReadString('\n')
			if err != io.EOF {
				if err != nil {
					panic(err)
				}
			}
			in = strings.TrimSpace(in)
			if len(in) > 0 {
				break
			}
		}

		Decode(in, &offer)

		// Set the remote SessionDescription
		if err := brv.peerConnection.SetRemoteDescription(offer); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		// Create answer
		answer, err := brv.peerConnection.CreateAnswer(nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		// Create channel that is blocked until ICE Gathering is complete
		gatherComplete := webrtc.GatheringCompletePromise(brv.peerConnection)

		// Sets the LocalDescription, and starts our UDP listeners
		if err := brv.peerConnection.SetLocalDescription(answer); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		// Block until ICE Gathering is complete, disabling trickle ICE
		// we do this because we only can exchange one signaling message
		// in a production application you should exchange ICE Candidates via OnICECandidate
		<-gatherComplete

		// Output the answer in base64 so we can paste it in browser
		w.Write([]byte(Encode(*brv.peerConnection.LocalDescription())))

		go httpServer.Shutdown(context.Background())
	})
	// TODO(erd): switch to logger
	println("waiting for POST")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}

	<-iceConnectedCtx.Done()

	// Start processing
	// TODO(erd): both need cancellation
	go brv.processInputFrames()
	go brv.processOutputFrames()
	close(brv.readyCh)
	return nil
}
