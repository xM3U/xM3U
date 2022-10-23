package src

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"xteve/internal/authentication"
)

func activatedSystemAuthentication() (err error) {
	err = authentication.Init(System.Folder.Config, 60)
	if err != nil {
		return
	}

	defaults := make(map[string]interface{})
	defaults["authentication.web"] = false
	defaults["authentication.pms"] = false
	defaults["authentication.xml"] = false
	defaults["authentication.api"] = false
	err = authentication.SetDefaultUserData(defaults)

	return
}

func CreateFirstUserForAuthentication(username, password string) (token string, err error) {
	authenticationErr := func(err error) {
		if err != nil {
			return
		}
	}

	err = authentication.CreateDefaultUser(username, password)
	authenticationErr(err)

	token, err = authentication.UserAuthentication(username, password)
	authenticationErr(err)

	token, err = authentication.CheckTheValidityOfTheToken(token)
	authenticationErr(err)

	userData := make(map[string]interface{})
	userData["username"] = username
	userData["authentication.web"] = true
	userData["authentication.pms"] = true
	userData["authentication.m3u"] = true
	userData["authentication.xml"] = true
	userData["authentication.api"] = false
	userData["defaultUser"] = true

	userID, err := authentication.GetUserID(token)
	authenticationErr(err)

	err = authentication.WriteUserData(userID, userData)
	authenticationErr(err)

	return
}

func TokenAuthentication(token string) (newToken string, err error) {
	if System.ConfigurationWizard == true {
		return
	}

	newToken, err = authentication.CheckTheValidityOfTheToken(token)

	return
}

func BasicAuth(r *http.Request, level string) (username string, err error) {
	err = errors.New("User authentication failed")

	auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)

	if len(auth) != 2 || auth[0] != "Basic" {
		return
	}

	payload, _ := base64.StdEncoding.DecodeString(auth[1])
	pair := strings.SplitN(string(payload), ":", 2)

	username = pair[0]
	password := pair[1]

	token, err := authentication.UserAuthentication(username, password)
	if err != nil {
		return
	}

	err = CheckAuthorizationLevel(token, level)

	return
}

func UrlAuth(r *http.Request, requestType string) (err error) {
	var level, token string

	username := r.URL.Query().Get("username")
	password := r.URL.Query().Get("password")

	switch requestType {

	case "m3u":
		level = "authentication.m3u"
		if Settings.AuthenticationM3U == true {
			token, err = authentication.UserAuthentication(username, password)
			if err != nil {
				return
			}
			err = CheckAuthorizationLevel(token, level)
		}

	case "xml":
		level = "authentication.xml"
		if Settings.AuthenticationXML == true {
			token, err = authentication.UserAuthentication(username, password)
			if err != nil {
				return
			}
			err = CheckAuthorizationLevel(token, level)
		}

	}

	return
}

func CheckAuthorizationLevel(token, level string) (err error) {
	authenticationErr := func(err error) {
		if err != nil {
			return
		}
	}

	userID, err := authentication.GetUserID(token)
	authenticationErr(err)

	userData, err := authentication.ReadUserData(userID)
	authenticationErr(err)

	if len(userData) > 0 {
		if v, ok := userData[level].(bool); ok {
			if v == false {
				err = errors.New("No authorization")
			}
		} else {
			userData[level] = false
			err = authentication.WriteUserData(userID, userData)
			err = errors.New("No authorization")
		}
	} else {
		err = authentication.WriteUserData(userID, userData)
		err = errors.New("No authorization")
	}

	return
}
