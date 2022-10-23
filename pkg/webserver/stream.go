package webserver

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"xteve/pkg/utils"
	src "xteve/pkg/xteve"
)

// Stream : Web Server /stream/
func Stream(w http.ResponseWriter, r *http.Request) {
	path := strings.Replace(r.RequestURI, "/stream/", "", 1)
	// var stream = strings.SplitN(path, "-", 2)

	streamInfo, err := src.GetStreamInfo(path)
	if err != nil {
		src.ShowError(err, 1203)
		utils.HttpStatusError(w, r, 404)
		return
	}

	// If an UDPxy host is set, and the stream URL is multicast (i.e. starts with 'udp://@'),
	// then streamInfo.URL needs to be rewritten to point to UDPxy.
	if src.Settings.UDPxy != "" && strings.HasPrefix(streamInfo.URL, "udp://@") {
		streamInfo.URL = fmt.Sprintf("http://%s/udp/%s/", src.Settings.UDPxy, strings.TrimPrefix(streamInfo.URL, "udp://@"))
	}

	switch src.Settings.Buffer {

	case "-":
		src.ShowInfo(fmt.Sprintf("Buffer:false [%s]", src.Settings.Buffer))

	case "xteve":
		if strings.Index(streamInfo.URL, "rtsp://") != -1 || strings.Index(streamInfo.URL, "rtp://") != -1 {
			err = errors.New("RTSP and RTP streams are not supported")
			src.ShowError(err, 2004)

			src.ShowInfo("Streaming URL:" + streamInfo.URL)
			http.Redirect(w, r, streamInfo.URL, 302)

			src.ShowInfo("Streaming Info:URL was passed to the client")
			return
		}

		src.ShowInfo(fmt.Sprintf("Buffer:true [%s]", src.Settings.Buffer))

	default:
		src.ShowInfo(fmt.Sprintf("Buffer:true [%s]", src.Settings.Buffer))

	}

	if src.Settings.Buffer != "-" {
		src.ShowInfo(fmt.Sprintf("Buffer Size:%d KB", src.Settings.BufferSize))
	}

	src.ShowInfo(fmt.Sprintf("Channel Name:%s", streamInfo.Name))
	src.ShowInfo(fmt.Sprintf("Client User-Agent:%s", r.Header.Get("User-Agent")))

	// Prüfen ob der Buffer verwendet werden soll
	switch src.Settings.Buffer {

	case "-":
		src.ShowInfo("Streaming URL:" + streamInfo.URL)
		http.Redirect(w, r, streamInfo.URL, 302)

		src.ShowInfo("Streaming Info:URL was passed to the client.")
		src.ShowInfo("Streaming Info:xTeVe is no longer involved, the client connects directly to the streaming server.")

	default:
		src.BufferingStream(streamInfo.PlaylistID, streamInfo.URL, streamInfo.Name, w, r)

	}

	return
}
