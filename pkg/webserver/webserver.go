package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"xteve/internal/authentication"
	"xteve/pkg/utils"
	webUI "xteve/pkg/webui"
	src "xteve/pkg/xteve"

	"github.com/gorilla/websocket"
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

// WS : Web Sockets /ws/
func WS(w http.ResponseWriter, r *http.Request) {
	var request src.RequestStruct
	var response src.ResponseStruct
	response.Status = true

	var newToken string

	/*
		if r.Header.Get("Origin") != "http://"+r.Host {
			utils.HttpStatusError(w, r, 403)
			return
		}
	*/

	conn, err := websocket.Upgrade(w, r, w.Header(), 1024, 1024)
	if err != nil {
		src.ShowError(err, 0)
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}

	src.SetGlobalDomain(r.Host)

	for {

		err = conn.ReadJSON(&request)

		if err != nil {
			return
		}

		if src.System.ConfigurationWizard == false {
			switch src.Settings.AuthenticationWEB {
			// Token Authentication
			case true:

				var token string
				tokens, ok := r.URL.Query()["Token"]

				if !ok || len(tokens[0]) < 1 {
					token = "-"
				} else {
					token = tokens[0]
				}

				newToken, err = src.TokenAuthentication(token)
				if err != nil {

					response.Status = false
					response.Reload = true
					response.Error = err.Error()
					request.Cmd = "-"

					if err = conn.WriteJSON(response); err != nil {
						src.ShowError(err, 1102)
					}

					return
				}

				response.Token = newToken
				response.Users, _ = authentication.GetAllUserData()
			}
		}

		switch request.Cmd {
		// Daten lesen
		case "getServerConfig":
			// response.Config = Settings

		case "updateLog":
			response = setDefaultResponseData(response, false)
			if err = conn.WriteJSON(response); err != nil {
				src.ShowError(err, 1022)
			} else {
				return
				break
			}
			return

		case "loadFiles":
			// response.Response = src.Settings.Files

		// Daten schreiben
		case "saveSettings":
			authenticationUpdate := src.Settings.AuthenticationWEB
			response.Settings, err = src.UpdateServerSettings(request)
			if err == nil {

				response.OpenMenu = strconv.Itoa(src.IndexOfString("settings", src.System.WEB.Menu))

				if src.Settings.AuthenticationWEB == true && authenticationUpdate == false {
					response.Reload = true
				}

			}

		case "saveFilesM3U":
			err = src.SaveFiles(request, "m3u")
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("playlist", src.System.WEB.Menu))
			}

		case "updateFileM3U":
			err = src.UpdateFile(request, "m3u")
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("playlist", src.System.WEB.Menu))
			}

		case "saveFilesHDHR":
			err = src.SaveFiles(request, "hdhr")
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("playlist", src.System.WEB.Menu))
			}

		case "updateFileHDHR":
			err = src.UpdateFile(request, "hdhr")
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("playlist", src.System.WEB.Menu))
			}

		case "saveFilesXMLTV":
			err = src.SaveFiles(request, "xmltv")
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("xmltv", src.System.WEB.Menu))
			}

		case "updateFileXMLTV":
			err = src.UpdateFile(request, "xmltv")
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("xmltv", src.System.WEB.Menu))
			}

		case "saveFilter":
			response.Settings, err = src.SaveFilter(request)
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("filter", src.System.WEB.Menu))
			}

		case "saveEpgMapping":
			err = src.SaveXEpgMapping(request)

		case "saveUserData":
			err = src.SaveUserData(request)
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("users", src.System.WEB.Menu))
			}

		case "saveNewUser":
			err = src.SaveNewUser(request)
			if err == nil {
				response.OpenMenu = strconv.Itoa(src.IndexOfString("users", src.System.WEB.Menu))
			}

		case "resetLogs":
			src.WebScreenLog.Log = make([]string, 0)
			src.WebScreenLog.Errors = 0
			src.WebScreenLog.Warnings = 0
			response.OpenMenu = strconv.Itoa(src.IndexOfString("log", src.System.WEB.Menu))

		case "xteveBackup":
			file, errNew := src.XteveBackup()
			err = errNew
			if err == nil {
				response.OpenLink = fmt.Sprintf("%s://%s/download/%s", src.System.ServerProtocol.WEB, src.System.Domain, file)
			}

		case "xteveRestore":
			src.WebScreenLog.Log = make([]string, 0)
			src.WebScreenLog.Errors = 0
			src.WebScreenLog.Warnings = 0

			if len(request.Base64) > 0 {

				newWebURL, err := src.XteveRestoreFromWeb(request.Base64)
				if err != nil {
					src.ShowError(err, 0o00)
					response.Alert = err.Error()
				}

				if err == nil {

					if len(newWebURL) > 0 {
						response.Alert = "Backup was successfully restored.\nThe port of the sTeVe URL has changed, you have to restart xTeVe.\nAfter a restart, xTeVe can be reached again at the following URL:\n" + newWebURL
					} else {
						response.Alert = "Backup was successfully restored."
						response.Reload = true
					}
					src.ShowInfo("xTeVe:" + "Backup successfully restored.")
				}

			}

		case "uploadLogo":
			if len(request.Base64) > 0 {
				response.LogoURL, err = src.UploadLogo(request.Base64, request.Filename)

				if err == nil {
					if err = conn.WriteJSON(response); err != nil {
						src.ShowError(err, 1022)
					} else {
						return
					}
				}

			}

		case "saveWizard":
			nextStep, errNew := src.SaveWizard(request)

			err = errNew
			if err == nil {
				if nextStep == 10 {
					src.System.ConfigurationWizard = false
					response.Reload = true
				} else {
					response.Wizard = nextStep
				}
			}

			/*
				case "wizardCompleted":
					src.System.ConfigurationWizard = false
					response.Reload = true
			*/
		default:
			fmt.Println("+ + + + + + + + + + +", request.Cmd)

			requestMap := make(map[string]interface{}) // Debug
			_ = requestMap
			if src.System.Dev == true {
				fmt.Println(src.MapToJSON(requestMap))
			}

		}

		if err != nil {
			response.Status = false
			response.Error = err.Error()
			response.Settings = src.Settings
		}

		response = setDefaultResponseData(response, true)
		if src.System.ConfigurationWizard == true {
			response.ConfigurationWizard = src.System.ConfigurationWizard
		}

		if err = conn.WriteJSON(response); err != nil {
			src.ShowError(err, 1022)
		} else {
			break
		}

	}

	return
}

// Web : Web Server /web/
func Web(w http.ResponseWriter, r *http.Request) {
	lang := make(map[string]interface{})
	var err error

	requestFile := strings.Replace(r.URL.Path, "/web", "html", -1)
	var content, contentType, file string

	var language src.LanguageUI

	src.SetGlobalDomain(r.Host)

	if src.System.Dev == true {

		lang, err = src.LoadJSONFileToMap(fmt.Sprintf("html/lang/%s.json", src.Settings.Language))
		if err != nil {
			src.ShowError(err, 0o00)
		}

	} else {

		languageFile := "html/lang/en.json"

		if value, ok := webUI.Assets[languageFile].(string); ok {
			content = utils.GetHTMLString(value)
			lang = src.JsonToMap(content)
		}

	}

	err = json.Unmarshal([]byte(src.MapToJSON(lang)), &language)
	if err != nil {
		src.ShowError(err, 0o00)
		return
	}

	if src.GetFilenameFromPath(requestFile) == "html" {

		if src.System.ScanInProgress == 0 {
			if len(src.Settings.Files.M3U) == 0 && len(src.Settings.Files.HDHR) == 0 {
				src.System.ConfigurationWizard = true
			}
		}

		switch src.System.ConfigurationWizard {

		case true:
			file = requestFile + "configuration.html"
			src.Settings.AuthenticationWEB = false

		case false:
			file = requestFile + "index.html"

		}

		if src.System.ScanInProgress == 1 {
			file = requestFile + "maintenance.html"
		}

		switch src.Settings.AuthenticationWEB {
		case true:

			var username, password, confirm string
			switch r.Method {
			case "POST":
				allUsers, _ := authentication.GetAllUserData()

				username = r.FormValue("username")
				password = r.FormValue("password")

				if len(allUsers) == 0 {
					confirm = r.FormValue("confirm")
				}

				// Erster Benutzer wird angelegt (Passwortbestätigung ist vorhanden)
				if len(confirm) > 0 {

					token, err := src.CreateFirstUserForAuthentication(username, password)
					if err != nil {
						utils.HttpStatusError(w, r, 429)
						return
					}
					// Redirect, damit die Daten aus dem Browser gelöscht werden.
					w = authentication.SetCookieToken(w, token)
					http.Redirect(w, r, "/web", 301)
					return

				}

				// Benutzername und Passwort vorhanden, wird jetzt überprüft
				if len(username) > 0 && len(password) > 0 {

					token, err := authentication.UserAuthentication(username, password)
					if err != nil {
						file = requestFile + "login.html"
						lang["authenticationErr"] = language.Login.Failed
						break
					}

					w = authentication.SetCookieToken(w, token)
					http.Redirect(w, r, "/web", 301) // Redirect, damit die Daten aus dem Browser gelöscht werden.

				} else {
					w = authentication.SetCookieToken(w, "-")
					http.Redirect(w, r, "/web", 301) // Redirect, damit die Daten aus dem Browser gelöscht werden.
				}

				return

			case "GET":
				lang["authenticationErr"] = ""
				_, token, err := authentication.CheckTheValidityOfTheTokenFromHTTPHeader(w, r)
				if err != nil {
					file = requestFile + "login.html"
					break
				}

				err = src.CheckAuthorizationLevel(token, "authentication.web")
				if err != nil {
					file = requestFile + "login.html"
					break
				}

			}

			allUserData, err := authentication.GetAllUserData()
			if err != nil {
				src.ShowError(err, 0o00)
				utils.HttpStatusError(w, r, 403)
				return
			}

			if len(allUserData) == 0 && src.Settings.AuthenticationWEB == true {
				file = requestFile + "create-first-user.html"
			}
		}

		requestFile = file

		if value, ok := webUI.Assets[requestFile]; ok {

			content = utils.GetHTMLString(value.(string))

			if contentType == "text/plain" {
				w.Header().Set("Content-Disposition", "attachment; filename="+src.GetFilenameFromPath(requestFile))
			}

		} else {

			utils.HttpStatusError(w, r, 404)
			return
		}

	}

	if value, ok := webUI.Assets[requestFile].(string); ok {

		content = utils.GetHTMLString(value)
		contentType = getContentType(requestFile)

		if contentType == "text/plain" {
			w.Header().Set("Content-Disposition", "attachment; filename="+src.GetFilenameFromPath(requestFile))
		}

	} else {
		utils.HttpStatusError(w, r, 404)
		return
	}

	contentType = getContentType(requestFile)

	if src.System.Dev == true {
		// Lokale Webserver Dateien werden geladen, nur für die Entwicklung
		content, _ = src.ReadStringFromFile(requestFile)
	}

	w.Header().Add("Content-Type", contentType)
	w.WriteHeader(200)

	if contentType == "text/html" || contentType == "application/javascript" {
		content = src.ParseTemplate(content, lang)
	}

	w.Write([]byte(content))
}

// API : API request /api/
func API(w http.ResponseWriter, r *http.Request) {
	/*
			API Bedingungen (ohne Authentifizierung):
			- API muss in den Einstellungen aktiviert sein

			Beispiel API Request mit curl
			Status:
			curl -X POST -H "Content-Type: application/json" -d '{"cmd":"status"}' http://localhost:34400/api/

			- - - - -

			API Bedingungen (mit Authentifizierung):
			- API muss in den Einstellungen aktiviert sein
			- API muss bei den Authentifizierungseinstellungen aktiviert sein
			- Benutzer muss die Berechtigung API haben

			Nach jeder API Anfrage wird ein Token generiert, dieser ist einmal in 60 Minuten gültig.
			In jeder Antwort ist ein neuer Token enthalten

			Beispiel API Request mit curl
			Login:
			curl -X POST -H "Content-Type: application/json" -d '{"cmd":"login","username":"plex","password":"123"}' http://localhost:34400/api/

			Antwort:
			{
		  	"status": true,
		  	"token": "U0T-NTSaigh-RlbkqERsHvUpgvaaY2dyRGuwIIvv"
			}

			Status mit Verwendung eines Tokens:
			curl -X POST -H "Content-Type: application/json" -d '{"cmd":"status","token":"U0T-NTSaigh-RlbkqERsHvUpgvaaY2dyRGuwIIvv"}' http://localhost:4400/api/

			Antwort:
			{
			  "epg.source": "XEPG",
			  "status": true,
			  "streams.active": 7,
			  "streams.all": 63,
			  "streams.xepg": 2,
			  "token": "mXiG1NE1MrTXDtyh7PxRHK5z8iPI_LzxsQmY-LFn",
			  "url.dvr": "localhost:34400",
			  "url.m3u": "http://localhost:34400/m3u/xteve.m3u",
			  "url.xepg": "http://localhost:34400/xmltv/xteve.xml",
			  "version.api": "1.1.0",
			  "version.xteve": "1.3.0"
			}
	*/

	src.SetGlobalDomain(r.Host)
	var request src.APIRequestStruct
	var response src.APIResponseStruct

	responseAPIError := func(err error) {
		var response src.APIResponseStruct

		response.Status = false
		response.Error = err.Error()
		w.Write([]byte(src.MapToJSON(response)))
		return
	}

	response.Status = true

	if src.Settings.API == false {
		utils.HttpStatusError(w, r, 423)
		return
	}

	if r.Method == "GET" {
		utils.HttpStatusError(w, r, 404)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		utils.HttpStatusError(w, r, 400)
		return

	}

	err = json.Unmarshal(b, &request)
	if err != nil {
		utils.HttpStatusError(w, r, 400)
		return
	}

	w.Header().Set("content-type", "application/json")

	if src.Settings.AuthenticationAPI == true {
		var token string
		switch len(request.Token) {
		case 0:
			if request.Cmd == "login" {
				token, err = authentication.UserAuthentication(request.Username, request.Password)
				if err != nil {
					responseAPIError(err)
					return
				}

			} else {
				err = errors.New("Login incorrect")
				if err != nil {
					responseAPIError(err)
					return
				}

			}

		default:
			token, err = src.TokenAuthentication(request.Token)
			fmt.Println(err)
			if err != nil {
				responseAPIError(err)
				return
			}

		}
		err = src.CheckAuthorizationLevel(token, "authentication.api")
		if err != nil {
			responseAPIError(err)
			return
		}

		response.Token = token

	}

	switch request.Cmd {
	case "login": // Muss nichts übergeben werden

	case "status":

		response.VersionXteve = src.System.Version
		response.VersionAPI = src.System.APIVersion
		response.StreamsActive = int64(len(src.Data.Streams.Active))
		response.StreamsAll = int64(len(src.Data.Streams.All))
		response.StreamsXepg = int64(src.Data.XEPG.XEPGCount)
		response.EpgSource = src.Settings.EpgSource
		response.URLDvr = src.System.Domain
		response.URLM3U = src.System.ServerProtocol.M3U + "://" + src.System.Domain + "/m3u/xteve.m3u"
		response.URLXepg = src.System.ServerProtocol.XML + "://" + src.System.Domain + "/xmltv/xteve.xml"

	case "update.m3u":
		err = src.GetProviderData("m3u", "")
		if err != nil {
			break
		}

		err = src.BuildDatabaseDVR()
		if err != nil {
			break
		}

	case "update.hdhr":

		err = src.GetProviderData("hdhr", "")
		if err != nil {
			break
		}

		err = src.BuildDatabaseDVR()
		if err != nil {
			break
		}

	case "update.xmltv":
		err = src.GetProviderData("xmltv", "")
		if err != nil {
			break
		}

	case "update.xepg":
		src.BuildXEPG(false)

	default:
		err = errors.New(src.GetErrMsg(5000))

	}

	if err != nil {
		responseAPIError(err)
	}

	w.Write([]byte(src.MapToJSON(response)))

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
