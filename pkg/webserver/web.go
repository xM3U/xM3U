package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"xteve/internal/authentication"
	"xteve/pkg/utils"
	webUI "xteve/pkg/webui"
	src "xteve/pkg/xteve"
)

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
