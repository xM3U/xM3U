package webserver

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"xteve/internal/authentication"
	"xteve/pkg/utils"
	src "xteve/pkg/xteve"
)

// StartWebserver : Startet den Webserver
func StartWebserver() (err error) {
	port := src.Settings.Port

	http.HandleFunc("/", Index)
	http.HandleFunc("/stream/", Stream)
	http.HandleFunc("/xmltv/", xTeVe)
	http.HandleFunc("/m3u/", xTeVe)
	http.HandleFunc("/data/", WS)
	http.HandleFunc("/web/", Web)
	http.HandleFunc("/download/", Download)
	http.HandleFunc("/api/", API)
	http.HandleFunc("/images/", Images)
	http.HandleFunc("/data_images/", DataImages)

	// http.HandleFunc("/auto/", Auto)

	src.ShowInfo("DVR IP:" + src.System.IPAddress + ":" + src.Settings.Port)

	ips := len(src.System.IPAddressesV4) + len(src.System.IPAddressesV6) - 1
	switch ips {

	case 0:
		src.ShowHighlight(fmt.Sprintf("Web Interface:%s://%s:%s/web/", src.System.ServerProtocol.WEB, src.System.IPAddress, src.Settings.Port))

	case 1:
		src.ShowHighlight(fmt.Sprintf("Web Interface:%s://%s:%s/web/ | xTeVe is also available via the other %d IP.", src.System.ServerProtocol.WEB, src.System.IPAddress, src.Settings.Port, ips))

	default:
		src.ShowHighlight(fmt.Sprintf("Web Interface:%s://%s:%s/web/ | xTeVe is also available via the other %d IP's.", src.System.ServerProtocol.WEB, src.System.IPAddress, src.Settings.Port, len(src.System.IPAddressesV4)+len(src.System.IPAddressesV6)-1))

	}

	if err = http.ListenAndServe(":"+port, nil); err != nil {
		src.ShowError(err, 1001)
		return
	}

	return
}

// Index : Web Server /
func Index(w http.ResponseWriter, r *http.Request) {
	var err error
	var response []byte
	path := r.URL.Path
	var debug string

	src.SetGlobalDomain(r.Host)

	debug = fmt.Sprintf("Web Server Request:Path: %s", path)
	src.ShowDebug(debug, 2)

	switch path {

	case "/discover.json":
		response, err = src.GetDiscover()
		w.Header().Set("Content-Type", "application/json")

	case "/lineup_status.json":
		response, err = src.GetLineupStatus()
		w.Header().Set("Content-Type", "application/json")

	case "/lineup.json":
		if src.Settings.AuthenticationPMS == true {

			_, err := src.BasicAuth(r, "authentication.pms")
			if err != nil {
				src.ShowError(err, 0o00)
				utils.HttpStatusError(w, r, 403)
				return
			}

		}

		response, err = src.GetLineup()
		w.Header().Set("Content-Type", "application/json")

	case "/device.xml", "/capability":
		response, err = src.GetCapability()
		w.Header().Set("Content-Type", "application/xml")

	default:
		response, err = src.GetCapability()
		w.Header().Set("Content-Type", "application/xml")
	}

	if err == nil {

		w.WriteHeader(200)
		w.Write(response)
		return

	}

	utils.HttpStatusError(w, r, 500)

	return
}

// Auto : HDHR routing (wird derzeit nicht benutzt)
func Auto(w http.ResponseWriter, r *http.Request) {
	channelID := strings.Replace(r.RequestURI, "/auto/v", "", 1)
	fmt.Println(channelID)

	/*
		switch src.Settings.Buffer {

		case true:
			var playlistID, streamURL, err = getStreamByChannelID(channelID)
			if err == nil {
				bufferingStream(playlistID, streamURL, w, r)
			} else {
				utils.HttpStatusError(w, r, 404)
			}

		case false:
			utils.HttpStatusError(w, r, 423)
		}
	*/

	return
}

// xTeVe : Web Server /xmltv/ und /m3u/
func xTeVe(w http.ResponseWriter, r *http.Request) {
	var requestType, groupTitle, file, content, contentType string
	var err error
	path := strings.TrimPrefix(r.URL.Path, "/")
	groups := []string{}

	src.SetGlobalDomain(r.Host)

	// XMLTV Datei
	if strings.Contains(path, "xmltv/") {

		requestType = "xml"

		file = src.System.Folder.Data + src.GetFilenameFromPath(path)

		content, err = src.ReadStringFromFile(file)
		if err != nil {
			utils.HttpStatusError(w, r, 404)
			return
		}

	}

	// M3U Datei
	if strings.Contains(path, "m3u/") {

		requestType = "m3u"
		groupTitle = r.URL.Query().Get("group-title")

		if src.System.Dev == false {
			// false: Dateiname wird im Header gesetzt
			// true: M3U wird direkt im Browser angezeigt
			w.Header().Set("Content-Disposition", "attachment; filename="+src.GetFilenameFromPath(path))
		}

		if len(groupTitle) > 0 {
			groups = strings.Split(groupTitle, ",")
		}

		content, err = src.BuildM3U(groups)
		if err != nil {
			src.ShowError(err, 0o00)
		}

	}

	// Authentifizierung überprüfen
	err = src.UrlAuth(r, requestType)
	if err != nil {
		src.ShowError(err, 0o00)
		utils.HttpStatusError(w, r, 403)
		return
	}

	contentType = http.DetectContentType([]byte(content))
	if strings.Contains(strings.ToLower(contentType), "xml") {
		contentType = "application/xml; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)

	if err == nil {
		w.Write([]byte(content))
	}

	return
}

// Images : Image Cache /images/
func Images(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	filePath := src.System.Folder.ImagesCache + src.GetFilenameFromPath(path)

	content, err := src.ReadByteFromFile(filePath)
	if err != nil {
		utils.HttpStatusError(w, r, 404)
		return
	}

	w.Header().Add("Content-Type", getContentType(filePath))
	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(content)))
	w.WriteHeader(200)
	w.Write(content)

	return
}

// DataImages : Image Pfad für Logos / Bilder die hochgeladen wurden /data_images/
func DataImages(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	filePath := src.System.Folder.ImagesUpload + src.GetFilenameFromPath(path)

	content, err := src.ReadByteFromFile(filePath)
	if err != nil {
		utils.HttpStatusError(w, r, 404)
		return
	}

	w.Header().Add("Content-Type", getContentType(filePath))
	w.Header().Add("Content-Length", fmt.Sprintf("%d", len(content)))
	w.WriteHeader(200)
	w.Write(content)

	return
}

// Download : Datei Download
func Download(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	file := src.System.Folder.Temp + src.GetFilenameFromPath(path)
	w.Header().Set("Content-Disposition", "attachment; filename="+src.GetFilenameFromPath(file))

	content, err := src.ReadStringFromFile(file)
	if err != nil {
		w.WriteHeader(404)
		return
	}

	os.RemoveAll(src.System.Folder.Temp + src.GetFilenameFromPath(path))
	w.Write([]byte(content))
	return
}

func setDefaultResponseData(response src.ResponseStruct, data bool) (defaults src.ResponseStruct) {
	defaults = response

	// Folgende Daten immer an den Client übergeben
	defaults.ClientInfo.ARCH = src.System.ARCH
	defaults.ClientInfo.EpgSource = src.Settings.EpgSource
	defaults.ClientInfo.DVR = src.System.Addresses.DVR
	defaults.ClientInfo.M3U = src.System.Addresses.M3U
	defaults.ClientInfo.XML = src.System.Addresses.XML
	defaults.ClientInfo.OS = src.System.OS
	defaults.ClientInfo.Streams = fmt.Sprintf("%d / %d", len(src.Data.Streams.Active), len(src.Data.Streams.All))
	defaults.ClientInfo.UUID = src.Settings.UUID
	defaults.ClientInfo.Errors = src.WebScreenLog.Errors
	defaults.ClientInfo.Warnings = src.WebScreenLog.Warnings
	defaults.Notification = src.System.Notification
	defaults.Log = src.WebScreenLog

	switch src.System.Branch {

	case "master":
		defaults.ClientInfo.Version = fmt.Sprintf("%s", src.System.Version)

	default:
		defaults.ClientInfo.Version = fmt.Sprintf("%s (%s)", src.System.Version, src.System.Build)
		defaults.ClientInfo.Branch = src.System.Branch

	}

	if data == true {

		defaults.Users, _ = authentication.GetAllUserData()
		// defaults.DVR = src.System.DVRAddress

		if src.Settings.EpgSource == "XEPG" {

			defaults.ClientInfo.XEPGCount = src.Data.XEPG.XEPGCount

			XEPG := make(map[string]interface{})

			if len(src.Data.Streams.Active) > 0 {

				XEPG["epgMapping"] = src.Data.XEPG.Channels
				XEPG["xmltvMap"] = src.Data.XMLTV.Mapping

			} else {

				XEPG["epgMapping"] = make(map[string]interface{})
				XEPG["xmltvMap"] = make(map[string]interface{})

			}

			defaults.XEPG = XEPG

		}

		defaults.Settings = src.Settings

		defaults.Data.Playlist.M3U.Groups.Text = src.Data.Playlist.M3U.Groups.Text
		defaults.Data.Playlist.M3U.Groups.Value = src.Data.Playlist.M3U.Groups.Value
		defaults.Data.StreamPreviewUI.Active = src.Data.StreamPreviewUI.Active
		defaults.Data.StreamPreviewUI.Inactive = src.Data.StreamPreviewUI.Inactive

	}

	return
}

func getContentType(filename string) (contentType string) {
	if strings.HasSuffix(filename, ".html") {
		contentType = "text/html"
	} else if strings.HasSuffix(filename, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(filename, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(filename, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(filename, ".jpg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(filename, ".gif") {
		contentType = "image/gif"
	} else if strings.HasSuffix(filename, ".svg") {
		contentType = "image/svg+xml"
	} else if strings.HasSuffix(filename, ".mp4") {
		contentType = "video/mp4"
	} else if strings.HasSuffix(filename, ".webm") {
		contentType = "video/webm"
	} else if strings.HasSuffix(filename, ".ogg") {
		contentType = "video/ogg"
	} else if strings.HasSuffix(filename, ".mp3") {
		contentType = "audio/mp3"
	} else if strings.HasSuffix(filename, ".wav") {
		contentType = "audio/wav"
	} else {
		contentType = "text/plain"
	}

	return
}
