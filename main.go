package main

import (
	"fmt"
	"github.com/alecthomas/kingpin"
	. "github.com/mikeyoon/goyoutube-upload/lib"
	"github.com/vektra/errors"
	"golang.org/x/oauth2"
	"google.golang.org/api/youtube/v3"
	"gopkg.in/cheggaaa/pb.v1"
	"log"
	"os"
	"time"
)

const UPLOAD_URL = "https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status,contentDetails"

var (
	title       = kingpin.Flag("title", "Title of the video").String()
	description = kingpin.Flag("description", "Description of the video").String()
	playlist    = kingpin.Flag("playlist", "Playlist to add the video to").String()
	tags        = kingpin.Flag("tags", "Tags for the video").Strings()
	privacy     = kingpin.Flag("privacy", "Privacy settings [public, unlisted, private]").Default("public").String()

	// utility flags
	check        = kingpin.Flag("check", "Check progress of the [filename]").Bool()
	findPlaylist = kingpin.Flag("find-playlist", "Find a playlist by title").String()

	filename = kingpin.Arg("filename", "Video file to upload").String()
)

func main() {
	var session *UploadSession = nil
	kingpin.Parse()

	if *findPlaylist != "" {
		client := GetClient(oauth2.NoContext)
		playlist, err := GetPlaylistByTitle(client, *findPlaylist)
		if err == nil {
			fmt.Printf("Found '%s', id: %s", playlist.Snippet.Title, playlist.Id)
		} else {
			log.Fatalf("Error finding playlist: %v", err)
		}

		return
	}

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

		return
	}

	// Begin standard upload flow
	if *filename == "" {
		log.Fatal("Filename is required")
	}

	meta := Metadata{Snippet: Snippet{}, Status: Status{}}
	meta.Snippet.Title = *title
	meta.Snippet.Description = *description
	meta.Snippet.Tags = *tags
	meta.Status.PrivacyStatus = *privacy

	stat, err := os.Stat(*filename)
	size := stat.Size()

	if err == nil {
		finished := make(chan bool)
		ticker := time.NewTicker(time.Second * 10)
		tickChan := ticker.C

		bar := pb.New64(size)
		bar.Units = pb.U_BYTES

		var video *youtube.Video = nil
		// Check if session already exists
		if session, err = OpenSession(*filename); err == nil {
			// Reopen session and continue upload if so
			offset, err := session.CheckSessionProgress()

			if err == nil {
				log.Printf("Resuming Upload at %d of %d bytes\n", offset, session.Size)
				bar.Set64(offset)
				bar.Start()
				go func() {
					video, err = session.Upload(*filename, offset)
					finished <- true
				}()
			}
		} else {
			// Open a new session
			session, err = CreateUploadSession(meta, size, UPLOAD_URL)
			if err != nil {
				log.Fatalf("Error creating session: %v", err)
			}
			session.Save(*filename)

			log.Println("Starting new upload")
			bar.Start()
			go func() {
				video, err = session.Upload(*filename, 0)
				finished <- true
			}()
		}

		for {
			select {
			case <-finished:
				ticker.Stop()
				bar.Set64(size)
				bar.Finish()
				tickChan = nil
				finished = nil
				break
			case <-tickChan:
				offset, err := session.CheckSessionProgress()
				if err == nil {
					if offset >= 0 {
						bar.Set64(offset + 1)
					} else {
						bar.Set64(size)
					}
				}
			}

			if tickChan == nil && finished == nil {
				break
			}
		}

		if video != nil && *playlist != "" {
			pl, err := GetPlaylistByTitle(session.Client, *playlist)
			if err != nil {
				err = errors.Format("Error adding video to playlist. %v", err)
			}

			pl.AddToPlaylist(session.Client, pl.Id, video.Id)
		}
	}

	if err == nil {
		log.Println("Upload Successful")
		os.Remove(*filename + ".session")
	} else {
		log.Fatalf("Error uploading video: %v", err)
	}
}
