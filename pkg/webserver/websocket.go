package webserver

import (
	"fmt"
	"net/http"
	"strconv"
	"xteve/internal/authentication"
	src "xteve/pkg/xteve"

	"github.com/gorilla/websocket"
)

// WS : Web Sockets /ws/
func WS(w http.ResponseWriter, r *http.Request) {
	var request src.RequestStruct
	var response src.ResponseStruct
	response.Status = true

	var newToken string

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
