package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"xteve/internal/authentication"
	"xteve/pkg/utils"
	src "xteve/pkg/xteve"
)

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
