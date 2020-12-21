package stream

// TODO: mem management with Ref, Deref, and cleanup?? REFACTOR AND CLEAN; make webpage auto connect

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

type RemoteView interface {
	Stop(ctx context.Context) error
	Ready() <-chan struct{}
	InputFrames() chan<- image.Image // TODO(erd): does duration of frame matter?
	SetOnClickHandler(func(x, y int))
	Debug() bool
	HTML() RemoteViewHTML
	SinglePageHTML() string
	Handler() RemoteViewHandler
}

type RemoteViewHTML struct {
	JavaScript string
	Body       string
}

func NewRemoteView(config RemoteViewConfig) (RemoteView, error) {
	return &basicRemoteView{
		config:       config,
		readyCh:      make(chan struct{}),
		inputFrames:  make(chan image.Image),
		outputFrames: make(chan []byte),
	}, nil
}

type basicRemoteView struct {
	mu             sync.Mutex
	config         RemoteViewConfig
	readyOnce      sync.Once
	readyCh        chan struct{}
	videoTracks    []*webrtc.TrackLocalStaticSample
	inputFrames    chan image.Image
	outputFrames   chan []byte
	encoder        *VPXEncoder
	onClickHandler func(x, y int)
}

type RemoteViewHandler struct {
	Name string
	Func http.HandlerFunc
}

func (brv *basicRemoteView) streamNum() int {
	if brv.config.StreamNumber != 0 {
		return brv.config.StreamNumber
	}
	return 0
}

func (brv *basicRemoteView) Handler() RemoteViewHandler {
	handlerName := fmt.Sprintf("offer_%d", brv.streamNum())
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		offer := webrtc.SessionDescription{}
		Decode(in, &offer)

		// Create a new RTCPeerConnection
		peerConnection, err := webrtc.NewPeerConnection(brv.config.WebRTCConfig)
		if err != nil {
			panic(err)
		}

		iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.TODO())

		// Set the handler for ICE connection state
		// This will notify you when the peer has connected/disconnected
		peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
			fmt.Printf("Connection State has changed %s \n", connectionState.String())
			if connectionState == webrtc.ICEConnectionStateConnected {
				iceConnectedCtxCancel()
			}
		})

		//	TODO(erd): MIME type should be configurable
		videoTrack, err := webrtc.NewTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: "video/vp8"},
			"video",
			"pion",
		)
		if err != nil {
			panic(err)
		}

		if _, err := peerConnection.AddTrack(videoTrack); err != nil {
			panic(err)
		}

		// dcID := uint16(0)
		// dc, err := peerConnection.CreateDataChannel("stuff", &webrtc.DataChannelInit{ID: &dcID})
		// if err != nil {
		// 	panic(err)
		// }

		// Set the remote SessionDescription
		if err := peerConnection.SetRemoteDescription(offer); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		// Create answer
		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		// Create channel that is blocked until ICE Gathering is complete
		gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

		// Sets the LocalDescription, and starts our UDP listeners
		if err := peerConnection.SetLocalDescription(answer); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		// Block until ICE Gathering is complete, disabling trickle ICE
		// we do this because we only can exchange one signaling message
		// in a production application you should exchange ICE Candidates via OnICECandidate
		<-gatherComplete

		// Output the answer in base64 so we can paste it in browser
		w.Write([]byte(Encode(*peerConnection.LocalDescription())))

		// go httpServer.Shutdown(context.Background())

		go func() {
			<-iceConnectedCtx.Done()

			// TODO(erd): handle disconnected
			brv.addVideoTrack(videoTrack)

			brv.readyOnce.Do(func() {
				close(brv.readyCh)
				// Start processing
				// TODO(erd): both need cancellation
				go brv.processInputFrames()
				go brv.processOutputFrames()
			})
		}()
	})
	return RemoteViewHandler{handlerName, handlerFunc}
}

func (brv *basicRemoteView) iceServers() string {
	var strBuf bytes.Buffer
	strBuf.WriteString("[")
	for _, server := range brv.config.WebRTCConfig.ICEServers {
		strBuf.WriteString("{")
		strBuf.WriteString("urls: ['")
		for _, u := range server.URLs {
			strBuf.WriteString(u)
			strBuf.WriteString("',")
		}
		if len(server.URLs) > 0 {
			strBuf.Truncate(strBuf.Len() - 1)
		}
		strBuf.WriteString("]")
		if server.Username != "" {
			strBuf.WriteString(",username:'")
			strBuf.WriteString(server.Username)
			strBuf.WriteString("'")
		}
		if cred, ok := server.Credential.(string); ok {
			strBuf.WriteString(",credential:'")
			strBuf.WriteString(cred)
			strBuf.WriteString("'")
		}
		strBuf.WriteString("},")
	}
	if len(brv.config.WebRTCConfig.ICEServers) > 0 {
		strBuf.Truncate(strBuf.Len() - 1)
	}
	strBuf.WriteString("]")
	return strBuf.String()
}

func (brv *basicRemoteView) SinglePageHTML() string {
	return fmt.Sprintf(viewHTML, brv.streamNum(), brv.iceServers())
}

func (brv *basicRemoteView) HTML() RemoteViewHTML {
	return RemoteViewHTML{
		JavaScript: fmt.Sprintf(viewJS, brv.streamNum(), brv.iceServers()),
		Body:       fmt.Sprintf(viewBody, brv.streamNum()),
	}
}

func (brv *basicRemoteView) Debug() bool {
	return brv.config.Debug
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

	// brv.dataChannel.OnOpen(func() {
	// 	for {
	// 		time.Sleep(time.Second)
	// 		// println("SEND TEXT")
	// 		if err := brv.dataChannel.SendText("hello"); err != nil {
	// 			panic(err)
	// 		}
	// 		// println("SENT TEXT")
	// 	}
	// })
	// brv.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
	// 	coords := strings.Split(string(msg.Data), ",")
	// 	if len(coords) != 2 {
	// 		panic(len(coords))
	// 	}
	// 	x, err := strconv.ParseFloat(coords[0], 32)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	y, err := strconv.ParseFloat(coords[1], 32)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	brv.onClickHandler(int(x), int(y)) // handler should return fast otherwise it could block
	// })

	// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
	// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
	framesSent := 0
	for outputFrame := range brv.outputFrames {
		now := time.Now()
		for _, videoTrack := range brv.getVideoTracks() {
			if ivfErr := videoTrack.WriteSample(media.Sample{Data: outputFrame, Duration: 33 * time.Millisecond}); ivfErr != nil {
				panic(ivfErr)
			}
		}
		framesSent++
		if brv.config.Debug {
			fmt.Println(framesSent, "write sample took", time.Since(now))
		}
	}
}

func (brv *basicRemoteView) initCodec(width, height int) error {
	if brv.encoder != nil {
		return errors.New("already initialized codec")
	}

	// TODO(erd): Codec configurable
	var err error
	brv.encoder, err = NewVPXEncoder(CodecVP8, width, height, brv.config.Debug)
	return err
}

func (brv *basicRemoteView) addVideoTrack(videoTrack *webrtc.TrackLocalStaticSample) {
	brv.mu.Lock()
	defer brv.mu.Unlock()
	brv.videoTracks = append(brv.videoTracks, videoTrack)
}

func (brv *basicRemoteView) getVideoTracks() []*webrtc.TrackLocalStaticSample {
	brv.mu.Lock()
	defer brv.mu.Unlock()
	// make shallow copy
	videoTracks := make([]*webrtc.TrackLocalStaticSample, 0, len(brv.videoTracks))
	for _, videoTrack := range brv.videoTracks {
		videoTracks = append(videoTracks, videoTrack)
	}
	return videoTracks
}
