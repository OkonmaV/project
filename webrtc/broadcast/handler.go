package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"thin-peak/httpservice"
	"thin-peak/logs/logger"
	"time"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

type Handler struct {
	conn                 *memcache.Client
	peerConnectionConfig webrtc.Configuration
	broadcast            map[string]*broadcast
	decoder              *httpservice.InnerService
}

type broadcast struct {
	userId    string
	audio     chan *webrtc.TrackLocalStaticRTP
	video     chan *webrtc.TrackLocalStaticRTP
	doneaudio *webrtc.TrackLocalStaticRTP
	donevideo *webrtc.TrackLocalStaticRTP
	ctx       context.Context
	cancel    context.CancelFunc
}

type cookieData struct {
	UserId  string `json:"Login"`
	MetaId  string `json:"metaid"`
	Surname string `json:"surname"`
	Name    string `json:"name"`
}

func NewHandler(memcs string, decoder *httpservice.InnerService) (*Handler, error) {
	conn := memcache.New(memcs)
	err := conn.Ping()
	if err != nil {
		return nil, err
	}
	logger.Info("Checking memcached", memcs)
	peerConnectionConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	return &Handler{conn, peerConnectionConfig, make(map[string]*broadcast), decoder}, nil
}

func (conf *Handler) Handle(r *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {

	if r.GetMethod() != suckhttp.POST {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	userid := strings.Trim(r.Uri.Path, "/")
	if userid == "" {
		return suckhttp.NewResponse(400, "Bad request"), nil
	}

	if bc, ok := conf.broadcast[userid]; ok {
		return conf.clientBroadcast(string(r.Body), bc, l)
	}

	koki, ok := r.GetCookie("koki")
	if !ok || len(koki) < 5 {
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}

	tokenDecoderReq, err := conf.decoder.CreateRequestFrom(suckhttp.GET, suckutils.Concat("/", koki), r)
	if err != nil {
		l.Error("CreateRequestFrom", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	tokenDecoderReq.AddHeader(suckhttp.Accept, "application/json")
	tokenDecoderResp, err := conf.decoder.Send(tokenDecoderReq)
	if err != nil {
		l.Error("Send", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if i, t := tokenDecoderResp.GetStatus(); i/100 != 2 {
		l.Debug("Resp from tokendecoder", t)
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	if len(tokenDecoderResp.GetBody()) == 0 {
		l.Debug("Resp from tokendecoder", "empty body")
		return suckhttp.NewResponse(403, "Forbidden"), nil
	}
	userData := &cookieData{}

	if err = json.Unmarshal(tokenDecoderResp.GetBody(), userData); err != nil {
		l.Error("Unmarshal", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	l.Info("UserId", userData.UserId)
	if userData.MetaId == userid {
		// start
		var innerErr error
		ctx, cancel := context.WithCancel(context.Background())
		bc := &broadcast{userId: userid, audio: make(chan *webrtc.TrackLocalStaticRTP, 1), video: make(chan *webrtc.TrackLocalStaticRTP, 1), ctx: ctx, cancel: cancel}
		token, err := sdpExchange(&conf.peerConnectionConfig, string(r.Body), func(peerConnection *webrtc.PeerConnection) {
			if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
				innerErr = err
				return
			}
			if _, err := peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
				innerErr = err
				return
			}

			peerConnection.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
				if is == webrtc.ICEConnectionStateDisconnected {
					cancel()
					peerConnection.Close()
				}
				logger.Info("State", is.String())
			})
			// Set a handler for when a new remote track starts, this handler saves buffers to disk as
			// an ivf file, since we could have multiple video tracks we provide a counter.
			// In your application this is where you would handle/process video
			peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
				// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
				go func() {
					ticker := time.NewTicker(time.Second * 3)
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}})
							if errSend != nil {
								logger.Error("Send", errSend)
							}
						}
					}
				}()

				codec := track.Codec()
				logger.Info("Codec", codec.MimeType)

				var localTrack *webrtc.TrackLocalStaticRTP
				// var t string
				var err error
				if strings.EqualFold(codec.MimeType, webrtc.MimeTypeOpus) {
					localTrack, err = webrtc.NewTrackLocalStaticRTP(codec.RTPCodecCapability, "audio", "pion")
					if err != nil {
						innerErr = err
						return
					}
					// t = "audio"
					bc.audio <- localTrack
				} else {
					localTrack, err = webrtc.NewTrackLocalStaticRTP(codec.RTPCodecCapability, "video", "pion")
					if err != nil {
						innerErr = err
						return
					}
					// t = "video"
					bc.video <- localTrack
				}

				logger.Info("Broadcast", "started")
				rtpBuf := make([]byte, 1500)
				for {
					select {
					case <-ctx.Done():
						return
					default:
						i, _, readErr := track.Read(rtpBuf)
						if readErr != nil {
							innerErr = err
							return
						}

						// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
						if _, err = localTrack.Write(rtpBuf[:i]); err != nil && !errors.Is(err, io.ErrClosedPipe) {
							innerErr = err
							return
						}
						// fmt.Println("Writed", t, i)
					}
				}
			})
		})
		if innerErr != nil {
			l.Error("PeerConnection", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		if err != nil {
			l.Error("PeerConnection", err)
			return suckhttp.NewResponse(500, "Internal Server Error"), nil
		}
		conf.broadcast[userid] = bc
		return suckhttp.NewResponse(200, "OK").SetBody([]byte(token)), nil
	} else {
		if bc, ok := conf.broadcast[userid]; ok {
			return conf.clientBroadcast(string(r.Body), bc, l)
		}
	}

	return suckhttp.NewResponse(403, "Unauthorized"), nil
}

func (conf *Handler) clientBroadcast(token string, bc *broadcast, l *logger.Logger) (*suckhttp.Response, error) {
	l.Info("Client", "connecting")
	var innerErr error
	token, err := sdpExchange(&conf.peerConnectionConfig, token, func(pc *webrtc.PeerConnection) {
		logger.Debug("Video", strconv.Itoa(len(bc.video)))
		if bc.donevideo == nil {
			bc.donevideo = <-bc.video
		}
		logger.Debug("Video", strconv.Itoa(len(bc.video)))
		videoRtpSender, err := pc.AddTrack(bc.donevideo)
		logger.Debug("Audio", "get")
		if bc.doneaudio == nil {
			bc.doneaudio = <-bc.audio
		}
		logger.Debug("Audio", "done")
		audioRtpSender, err := pc.AddTrack(bc.doneaudio)
		if err != nil {
			innerErr = err
			return
		}
		if err != nil {
			innerErr = err
			return
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := audioRtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				} else {
					// fmt.Println("Audio Readed", n)
				}
			}
		}()

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := videoRtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				} else {
					// fmt.Println("Video Readed", n)
				}

			}
		}()
	})

	if innerErr != nil {
		l.Error("PeerConnection", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	if err != nil {
		l.Error("PeerConnection", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}
	l.Info("Client", "connected")
	return suckhttp.NewResponse(200, "OK").SetBody([]byte(token)), nil
}
