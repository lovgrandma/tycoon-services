package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"github.com/pkg/errors"
	"github.com/yutopp/go-flv"
	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
	"tycoon.systems/tycoon-services/security"

	"sync"
)

var _ rtmp.Handler = (*Handler)(nil)
var cache *Cache = NewCache()

type Cache struct {
	mu    sync.RWMutex
	store map[string]string
}

func NewCache() *Cache {
	return &Cache{
		store: make(map[string]string),
	}
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.store[key]
	return value, ok
}

func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

// Handler An RTMP connection handler
type Handler struct {
	rtmp.DefaultHandler
	flvFile  *os.File
	flvEnc   *flv.Encoder
	streamId string
}

func (h *Handler) OnServe(conn *rtmp.Conn) {
}

func (h *Handler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
	log.Printf("OnConnect: %#v", cmd)
	return nil
}

func (h *Handler) OnCreateStream(timestamp uint32, cmd *rtmpmsg.NetConnectionCreateStream) error {
	log.Printf("OnCreateStream: %#v", cmd)
	return nil
}

func (h *Handler) OnPublish(_ *rtmp.StreamContext, timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
	log.Printf("OnPublish: %#v %v", cmd, cmd.PublishingName)
	if reflect.TypeOf(cmd.PublishingName).Kind() == reflect.String {
		log.Printf("Stream Key %v", cmd.PublishingName)
		validStreamKey := isValidStreamKey(cmd.PublishingName)
		if validStreamKey == true {
			log.Printf("Stream Key Valid")
			domain := GetDomainStreamKey(cmd.PublishingName)
			key := GetStreamIdStreamKey(cmd.PublishingName)
			user := security.FindUserpByFieldValue(domain, "key", key)
			var userId string
			if len(user) != 0 {
				if reflect.TypeOf(user["id"]).Kind() == reflect.String {
					userId = user["id"].(string)
				}
			}
			resolvedStream := security.FindLive(domain, "author", userId, "creation", "desc", "", "10")
			log.Printf("Resolved Streams %v", resolvedStream)
			if reflect.TypeOf(resolvedStream["id"]).Kind() == reflect.String {
				h.streamId = resolvedStream["id"].(string)
			}
			log.Printf("Stream h %v", h)
		} else { // Reject when key not valid
			log.Printf("Not authorized to stream")
			return errors.New("Not authorized to stream")
		}
	}
	// (example) Reject a connection when PublishingName is empty
	if cmd.PublishingName == "" {
		return errors.New("PublishingName is empty")
	}

	// Record streams as FLV!
	p := filepath.Join(
		os.TempDir(),
		filepath.Clean(filepath.Join("/", fmt.Sprintf("%s.flv", cmd.PublishingName))),
	)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return errors.Wrap(err, "Failed to create flv file")
	}
	h.flvFile = f

	enc, err := flv.NewEncoder(f, flv.FlagsAudio|flv.FlagsVideo)
	if err != nil {
		_ = f.Close()
		return errors.Wrap(err, "Failed to create flv encoder")
	}
	h.flvEnc = enc

	return nil
}

func (h *Handler) OnSetDataFrame(timestamp uint32, data *rtmpmsg.NetStreamSetDataFrame) error {
	r := bytes.NewReader(data.Payload)

	var script flvtag.ScriptData
	if err := flvtag.DecodeScriptData(r, &script); err != nil {
		log.Printf("Failed to decode script data: Err = %+v", err)
		return nil // ignore
	}

	log.Printf("SetDataFrame: Script = %#v", script)

	if err := h.flvEnc.Encode(&flvtag.FlvTag{
		TagType:   flvtag.TagTypeScriptData,
		Timestamp: timestamp,
		Data:      &script,
	}); err != nil {
		log.Printf("Failed to write script data: Err = %+v", err)
	}

	return nil
}

func (h *Handler) OnAudio(timestamp uint32, payload io.Reader) error {
	var audio flvtag.AudioData
	if err := flvtag.DecodeAudioData(payload, &audio); err != nil {
		return err
	}

	flvBody := new(bytes.Buffer)
	if _, err := io.Copy(flvBody, audio.Data); err != nil {
		return err
	}
	audio.Data = flvBody

	// log.Printf("FLV Audio Data: Timestamp = %d, SoundFormat = %+v, SoundRate = %+v, SoundSize = %+v, SoundType = %+v, AACPacketType = %+v, Data length = %+v",
	// 	timestamp,
	// 	audio.SoundFormat,
	// 	audio.SoundRate,
	// 	audio.SoundSize,
	// 	audio.SoundType,
	// 	audio.AACPacketType,
	// 	len(flvBody.Bytes()),
	// )

	if err := h.flvEnc.Encode(&flvtag.FlvTag{
		TagType:   flvtag.TagTypeAudio,
		Timestamp: timestamp,
		Data:      &audio,
	}); err != nil {
		log.Printf("Failed to write audio: Err = %+v", err)
	}

	return nil
}

func (h *Handler) OnVideo(timestamp uint32, payload io.Reader) error {
	var video flvtag.VideoData
	if err := flvtag.DecodeVideoData(payload, &video); err != nil {
		return err
	}

	flvBody := new(bytes.Buffer)
	if _, err := io.Copy(flvBody, video.Data); err != nil {
		return err
	}
	video.Data = flvBody

	log.Printf("FLV Video Data: Handler Stream: %v, Timestamp = %d, FrameType = %+v, CodecID = %+v, AVCPacketType = %+v, CT = %+v, Data length = %+v",
		h.streamId,
		timestamp,
		video.FrameType,
		video.CodecID,
		video.AVCPacketType,
		video.CompositionTime,
		len(flvBody.Bytes()),
	)

	if err := h.flvEnc.Encode(&flvtag.FlvTag{
		TagType:   flvtag.TagTypeVideo,
		Timestamp: timestamp,
		Data:      &video,
	}); err != nil {
		log.Printf("Failed to write video: Err = %+v", err)
	}

	return nil
}

func (h *Handler) OnClose() {
	log.Printf("OnClose")

	if h.flvFile != nil {
		_ = h.flvFile.Close()
	}
}

// fileName, found := cache.Get(cmd.PublishingName)
// if found {
// 	fmt.Println("S3 file name:", fileName)
// } else {
// 	fmt.Println("File name not found")
// }
