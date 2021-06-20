var log = msg => {
	document.getElementById('logs').innerHTML += msg + '<br>'
}

function post(url, data, f) {
	log("go");
	let xhr = new XMLHttpRequest();
	xhr.onreadystatechange = () => {

		if (xhr.readyState == 4 && xhr.status == 200) {
			f(xhr);
		}

		log(xhr.status, xhr)
	};
	xhr.open("POST", url);
	//xhr.setRequestHeader('Content-type', 'application/x-www-form-urlencoded');
	xhr.send(data);
}

window.createSession = (isPublisher, userId) => {
	let pc = new RTCPeerConnection({
		iceServers: [
			{
				urls: 'stun:stun.l.google.com:19302'
			}
		]
	})
	pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
	pc.onicecandidate = event => {
		if (event.candidate === null) {
			let v = btoa(JSON.stringify(pc.localDescription));

			post("/sdp/"+userId, v.toString(), (x) => {
				window.sd = x.responseText;
			});

		}
	}

	if (isPublisher) {
		const stream = getScreenShareWithMic();
		stream.then(s => {
			log(s.getTracks());
			s.getTracks().forEach(track => pc.addTrack(track, s));
			document.getElementById('video').srcObject = s;
			pc.createOffer()
				.then(d => pc.setLocalDescription(d))
				.catch(log)
		}).catch(log);
	} else {

		pc.addTransceiver('video', {'direction': 'sendrecv'})
		pc.addTransceiver('audio', {'direction': 'sendrecv'})

		// pc.addTransceiver('video')
		// pc.addTransceiver('audio')
		pc.createOffer()
			.then(d => pc.setLocalDescription(d))
			.catch(log)

		pc.ontrack = function (event) {
		  var el = document.getElementById('video')
		  el.srcObject = event.streams[0]
		  el.autoplay = true
		  el.controls = true
		}
	}

	window.startSession = () => {
		try {
			pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(window.sd))))
		} catch (e) {
			alert(e)
		}
	}

	let btns = document.getElementsByClassName('createSessionButton')
	for (let i = 0; i < btns.length; i++) {
		btns[i].style = 'display: none'
	}

}

async function getScreenShareWithMic() {
	const stream = await navigator.mediaDevices.getDisplayMedia({video: true});
	const audio = await navigator.mediaDevices.getUserMedia({audio: true});
	return new MediaStream([audio.getTracks()[0], stream.getTracks()[0]]);
}

