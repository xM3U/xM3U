package src

import (
	"fmt"
	"math/rand"
	"time"
)

// InitMaintenance : Wartungsprozess initialisieren
func InitMaintenance() (err error) {
	rand.Seed(time.Now().Unix())
	System.TimeForAutoUpdate = fmt.Sprintf("0%d%d", randomTime(0, 2), randomTime(10, 59))

	go maintenance()

	return
}

func maintenance() {
	for {

		t := time.Now()

		// Aktualisierung der Playlist und XMLTV Dateien
		if System.ScanInProgress == 0 {

			for _, schedule := range Settings.Update {
				if schedule == t.Format("1504") {

					ShowInfo("Update:" + schedule)

					// Backup erstellen
					err := xTeVeAutoBackup()
					if err != nil {
						ShowError(err, 0o00)
					}

					// Playlist und XMLTV Dateien aktualisieren
					GetProviderData("m3u", "")
					GetProviderData("hdhr", "")

					if Settings.EpgSource == "XEPG" {
						GetProviderData("xmltv", "")
					}

					// Datenbank für DVR erstellen
					err = BuildDatabaseDVR()
					if err != nil {
						ShowError(err, 0o00)
					}

					if Settings.CacheImages == false && System.ImageCachingInProgress == 0 {
						removeChildItems(System.Folder.ImagesCache)
					}

					// XEPG Dateien erstellen
					Data.Cache.XMLTV = make(map[string]XMLTV)
					BuildXEPG(false)

				}
			}

			// Update xTeVe (Binary)
			if System.TimeForAutoUpdate == t.Format("1504") {
				BinaryUpdate()
			}

		}

		time.Sleep(60 * time.Second)

	}

	return
}

func randomTime(min, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}
