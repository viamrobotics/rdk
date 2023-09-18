import { dialWebRTC } from "@viamrobotics/rpc";
import { AddStreamRequest, AddStreamResponse, RemoveStreamRequest, RemoveStreamResponse, ListStreamsRequest, ListStreamsResponse } from "./gen/proto/stream/v1/stream_pb";
import { ServiceError, StreamServiceClient } from "./gen/proto/stream/v1/stream_pb_service";

const signalingAddress = `${window.location.protocol}//${window.location.host}`;
const host = "local";

declare global {
	interface Window {
		allowSendAudio: boolean;
	}
}

async function startup() {
	const webRTCConn = await dialWebRTC(signalingAddress, host);
	const streamClient = new StreamServiceClient(host, { transport: webRTCConn.transportFactory });

	let pResolve: (value: string[]) => void;
	let pReject: (reason?: any) => void;
	let namesPromise = new Promise<string[]>((resolve, reject) => {
		pResolve = resolve;
		pReject = reject;
	});
	const listRequest = new ListStreamsRequest();
	streamClient.listStreams(listRequest, (err: ServiceError, resp: ListStreamsResponse) => {
		if (err) {
			pReject(err);
			return
		}
		pResolve(resp.getNamesList());
	});
	const names = await namesPromise;

	const makeButtonClick = (button: HTMLButtonElement, streamName: string, add: boolean) => async (e: Event) => {
		e.preventDefault();

		button.disabled = true;

		if (add) {
			const addRequest = new AddStreamRequest();
			addRequest.setName(streamName);
			streamClient.addStream(addRequest, (err: ServiceError, resp: AddStreamResponse) => {
				if (err) {
					console.error(err);
					button.disabled = false;
				}
			});
		} else {
			const removeRequest = new RemoveStreamRequest();
			removeRequest.setName(streamName);
			streamClient.removeStream(removeRequest, (err: ServiceError, resp: RemoveStreamResponse) => {
				if (err) {
					console.error(err);
					button.disabled = false;
				}
			});
		}
	};

	webRTCConn.peerConnection.ontrack = async event => {
		const mediaElementContainer = document.createElement('div');
		mediaElementContainer.id = event.track.id;
		const mediaElement = document.createElement(event.track.kind);
		if (mediaElement instanceof HTMLVideoElement || mediaElement instanceof HTMLAudioElement) {
			mediaElement.srcObject = event.streams[0];
			mediaElement.autoplay = true;
			if (mediaElement instanceof HTMLVideoElement) {
				mediaElement.playsInline = true;				
				mediaElement.controls = false;
			} else {
				mediaElement.controls = true;
			}
		}

		const stream = event.streams[0];
		const streamName = stream.id;
		const streamContainer = document.getElementById(`stream-${streamName}`)!;
		let btns = streamContainer.getElementsByTagName("button");
		if (btns.length) {
			const button = btns[0];
			button.innerText = `Stop ${streamName}`;
			button.onclick = makeButtonClick(button, streamName, false);
			button.disabled = false;

			let audioSender: RTCRtpSender;
			stream.onremovetrack = async event => {
				const mediaElementContainer = document.getElementById(event.track.id)!;
				const mediaElement = mediaElementContainer.getElementsByTagName(event.track.kind)[0];
				if (audioSender) {
					webRTCConn.peerConnection.removeTrack(audioSender);
				}
				if (mediaElement instanceof HTMLVideoElement || mediaElement instanceof HTMLAudioElement) {
					mediaElement.pause();
					mediaElement.removeAttribute('srcObject');
					mediaElement.removeAttribute('src');
					mediaElement.load();
				}
				mediaElementContainer.remove();

				button.innerText = `Start ${streamName}`
				button.onclick = makeButtonClick(button, streamName, true);
				button.disabled = false;
			};

			if (mediaElement instanceof HTMLAudioElement && window.allowSendAudio) {
				const button = document.createElement("button");
				button.innerText = `Send audio`
				button.onclick = async (e) => {
					e.preventDefault();

					button.remove();

					navigator.mediaDevices.getUserMedia({
						audio: {
							deviceId: 'default',
							autoGainControl: false,
							channelCount: 2,
							echoCancellation: false,
							latency: 0,
							noiseSuppression: false,
							sampleRate: 48000,
							sampleSize: 16,
							volume: 1.0
						},
						video: false
					}).then((stream) => {
						audioSender = webRTCConn.peerConnection.addTrack(stream.getAudioTracks()[0]);
					}).catch((err) => {
						console.error(err)
					});
				}
				mediaElementContainer.appendChild(button);
				mediaElementContainer.appendChild(document.createElement("br"));
			}
		}
		mediaElementContainer.appendChild(document.createElement("br"));
		mediaElementContainer.appendChild(mediaElement);
		streamContainer.appendChild(mediaElementContainer);
	}

	for (const name of names) {
		const container = document.createElement("div");
		container.id = `stream-${name}`;
		const button = document.createElement("button");
		button.innerText = `Start ${name}`
		button.onclick = makeButtonClick(button, name, true);
		container.appendChild(button);
		document.body.appendChild(container);
	}
}
startup().catch(e => {
	console.error(e);
});
