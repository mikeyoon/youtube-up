package main

import (
	"github.com/alecthomas/kingpin"
	. "github.com/mikeyoon/goyoutube-upload/lib"
	"log"
	"os"
)

const UPLOAD_URL = "https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status,contentDetails"

var (
	title       = kingpin.Flag("title", "Title of the video").String()
	description = kingpin.Flag("description", "Description of the video").String()
	playlist    = kingpin.Flag("playlist", "Playlist to add the video to").String()
	tags        = kingpin.Flag("tags", "Tags for the video").Strings()
	check       = kingpin.Flag("check", "Check progress of the filename").Bool()
	privacy     = kingpin.Flag("privacy", "Privacy settings [public, unlisted, private]").Default("public").String()

	filename = kingpin.Arg("filename", "Video file to upload").Required().String()
)

func main() {
	var session *UploadSession

	kingpin.Parse()

	if *check {
		if session, err := OpenSession(*filename); err == nil {
			if err != nil {
				log.Fatalf("Error opening session: %v", err)
			}

			progress, err := session.CheckSessionProgress()
			if err != nil {
				log.Fatalf("Error checking session progress: %v", err)
			}

			if progress < 0 {
				log.Println("Upload complete")
			} else {
				log.Printf("Progress: %d/%d bytes (%d%%)\n", progress, session.Size, progress/session.Size)
			}
		}
	} else {
		meta := Metadata{Snippet: Snippet{}, Status: Status{}}
		meta.Snippet.Title = *title
		meta.Snippet.Description = *description
		meta.Snippet.Tags = *tags
		meta.Status.PrivacyStatus = *privacy

		stat, err := os.Stat(*filename)
		size := stat.Size()

		if err == nil {
			// Check if session already exists
			if session, err = OpenSession(*filename); err == nil {
				// Reopen session and continue upload if so
				offset, err := session.CheckSessionProgress()

				if err == nil {
					log.Printf("Resuming Upload at %d of %d bytes\n", offset, session.Size)
					err = session.Upload(*filename, offset)
				}
			} else {
				// Open a new session
				session, err = CreateUploadSession(meta, size, UPLOAD_URL)
				if (err != nil) {
					log.Fatalf("Error creating session: %v", err)
				}
				session.Save(*filename)

				log.Println("Starting new upload")
				err = session.Upload(*filename, 0)
			}
		}

		if err == nil {
			log.Println("Upload Successful")
			os.Remove(*filename + ".session")
			// Start/Resume upload
			// If failed due to connection issue, retry every 60 seconds
		}

		if err != nil {
			log.Fatalf("Error uploading video: %v", err)
		}
	}
}
