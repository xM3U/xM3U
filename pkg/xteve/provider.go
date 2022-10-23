package src

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	m3u "xteve/internal/m3u-parser"
)

// fileType: Welcher Dateityp soll aktualisiert werden (m3u, hdhr, xml) | fileID: Update einer bestimmten Datei (Provider ID)
func GetProviderData(fileType, fileID string) (err error) {
	var fileExtension, serverFileName string
	body := make([]byte, 0)
	newProvider := false
	dataMap := make(map[string]interface{})

	saveDateFromProvider := func(fileSource, serverFileName, id string, body []byte) (err error) {
		data := make(map[string]interface{})

		if value, ok := dataMap[id].(map[string]interface{}); ok {
			data = value
		} else {
			data["id.provider"] = id
			dataMap[id] = data
		}

		// Default keys für die Providerdaten
		keys := []string{"name", "description", "type", "file." + System.AppName, "file.source", "tuner", "last.update", "compatibility", "counter.error", "counter.download", "provider.availability"}

		for _, key := range keys {
			if _, ok := data[key]; !ok {
				switch key {

				case "name":
					data[key] = serverFileName

				case "description":
					data[key] = ""

				case "type":
					data[key] = fileType

				case "file." + System.AppName:
					data[key] = id + fileExtension

				case "file.source":
					data[key] = fileSource

				case "last.update":
					data[key] = time.Now().Format("2006-01-02 15:04:05")

				case "tuner":
					if fileType == "m3u" || fileType == "hdhr" {
						if _, ok := data[key].(float64); !ok {
							data[key] = 1
						}
					}

				case "compatibility":
					data[key] = make(map[string]interface{})

				case "counter.download":
					data[key] = 0.0

				case "counter.error":
					data[key] = 0.0

				case "provider.availability":
					data[key] = 100
				}
			}
		}

		if _, ok := data["id.provider"]; !ok {
			data["id.provider"] = id
		}

		// Datei extrahieren
		body, err = extractGZIP(body, fileSource)
		if err != nil {
			ShowError(err, 0o00)
			return
		}

		// Daten überprüfen
		ShowInfo("Check File:" + fileSource)
		switch fileType {

		case "m3u":
			_, err = m3u.MakeInterfaceFromM3U(body)

		case "hdhr":
			_, err = jsonToInterface(string(body))

		case "xmltv":
			err = checkXMLCompatibility(id, body)

		}

		if err != nil {
			return
		}

		filePath := System.Folder.Data + data["file."+System.AppName].(string)

		err = writeByteToFile(filePath, body)

		if err == nil {
			data["last.update"] = time.Now().Format("2006-01-02 15:04:05")
			data["counter.download"] = data["counter.download"].(float64) + 1
		}

		return
	}

	switch fileType {

	case "m3u":
		dataMap = Settings.Files.M3U
		fileExtension = ".m3u"

	case "hdhr":
		dataMap = Settings.Files.HDHR
		fileExtension = ".json"

	case "xmltv":
		dataMap = Settings.Files.XMLTV
		fileExtension = ".xml"

	}

	for dataID, d := range dataMap {

		data := d.(map[string]interface{})
		fileSource := data["file.source"].(string)
		newProvider = false

		if _, ok := data["new"]; ok {
			newProvider = true
			delete(data, "new")
		}

		// Wenn eine ID vorhanden ist und nicht mit der aus der Datanbank übereinstimmt, wird die Aktualisierung übersprungen (goto)
		if len(fileID) > 0 && newProvider == false {
			if dataID != fileID {
				goto Done
			}
		}

		switch fileType {

		case "hdhr":

			// Laden vom HDHomeRun Tuner
			ShowInfo("Tuner:" + fileSource)
			tunerURL := "http://" + fileSource + "/lineup.json"
			serverFileName, body, err = downloadFileFromServer(tunerURL)

		default:

			if strings.Contains(fileSource, "http://") || strings.Contains(fileSource, "https://") {

				// Laden vom Remote Server
				ShowInfo("Download:" + fileSource)
				serverFileName, body, err = downloadFileFromServer(fileSource)

			} else {

				// Laden einer lokalen Datei
				ShowInfo("Open:" + fileSource)

				err = checkFile(fileSource)
				if err == nil {
					body, err = ReadByteFromFile(fileSource)
					serverFileName = GetFilenameFromPath(fileSource)
				}

			}

		}

		if err == nil {

			err = saveDateFromProvider(fileSource, serverFileName, dataID, body)
			if err == nil {
				ShowInfo("Save File:" + fileSource + " [ID: " + dataID + "]")
			}

		}

		if err != nil {

			ShowError(err, 0o00)
			downloadErr := err

			if newProvider == false {

				// Prüfen ob ältere Datei vorhanden ist
				file := System.Folder.Data + dataID + fileExtension

				err = checkFile(file)
				if err == nil {

					if len(fileID) == 0 {
						ShowWarning(1011)
					}

					err = downloadErr
				}

				// Fehler Counter um 1 erhöhen
				data := make(map[string]interface{})
				if value, ok := dataMap[dataID].(map[string]interface{}); ok {

					data = value
					data["counter.error"] = data["counter.error"].(float64) + 1
					data["counter.download"] = data["counter.download"].(float64) + 1

				}

			} else {
				return downloadErr
			}

		}

		// Berechnen der Fehlerquote
		if newProvider == false {
			if value, ok := dataMap[dataID].(map[string]interface{}); ok {

				data := make(map[string]interface{})
				data = value

				if data["counter.error"].(float64) == 0 {
					data["provider.availability"] = 100
				} else {
					data["provider.availability"] = int(data["counter.error"].(float64)*100/data["counter.download"].(float64)*-1 + 100)
				}

			}
		}

		switch fileType {

		case "m3u":
			Settings.Files.M3U = dataMap

		case "hdhr":
			Settings.Files.HDHR = dataMap

		case "xmltv":
			Settings.Files.XMLTV = dataMap
			delete(Data.Cache.XMLTV, System.Folder.Data+dataID+fileExtension)

		}

		saveSettings(Settings)

	Done:
	}

	return
}

func downloadFileFromServer(providerURL string) (filename string, body []byte, err error) {
	_, err = url.ParseRequestURI(providerURL)
	if err != nil {
		return
	}

	resp, err := http.Get(providerURL)
	if err != nil {
		return
	}

	resp.Header.Set("User-Agent", Settings.UserAgent)

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf(fmt.Sprintf("%d: %s "+http.StatusText(resp.StatusCode), resp.StatusCode, providerURL))
		return
	}

	// Dateiname aus dem Header holen
	index := strings.Index(resp.Header.Get("Content-Disposition"), "filename")

	if index > -1 {

		headerFilename := resp.Header.Get("Content-Disposition")[index:len(resp.Header.Get("Content-Disposition"))]
		value := strings.Split(headerFilename, `=`)
		f := strings.Replace(value[1], `"`, "", -1)

		f = strings.Replace(f, `;`, "", -1)
		filename = f
		ShowInfo("Header filename:" + filename)

	} else {

		cleanFilename := strings.SplitN(GetFilenameFromPath(providerURL), "?", 2)
		filename = cleanFilename[0]

	}

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	return
}
