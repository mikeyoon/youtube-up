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
	upload      = kingpin.Command("upload", "Upload a video")
	filename    = upload.Arg("filename", "Path of the video to upload").Required().String()
	title       = upload.Flag("title", "Title of the video").String()
	description = upload.Flag("description", "Description of the video").String()
	playlist    = upload.Flag("playlist", "Playlist to add the video to").String()
	tags        = upload.Flag("tags", "Comma-separated list of tags for the video").Strings()
	privacy     = upload.Flag("privacy", "Privacy settings [public, unlisted, private]").Default("public").String()
	interval    = upload.Flag("interval", "Timer interval for checking progress, in seconds. "+
		"Requires an API call each time and Youtube does not update that frequently. Recommended "+
		"that this not be set lower than 10.").Default("30").Int()

	check         = kingpin.Command("check", "Check progress of the [filename]")
	checkFilename = check.Arg("filename", "Path of the video to check").Required().String()

	find              = kingpin.Command("find", "Find items in Youtube. Currently only supports playlists")
	findPlaylist      = find.Command("playlist", "Find a playlist")
	findPlaylistTitle = findPlaylist.Flag("title", "Title of the playlist to find").String()
)

func main() {
	var session *UploadSession = nil
	switch kingpin.Parse() {
	case "find playlist":
		client := GetClient(oauth2.NoContext)
		playlist, err := GetPlaylistByTitle(client, *findPlaylistTitle)
		if err == nil {
			fmt.Printf("Found '%s', id: %s", playlist.Snippet.Title, playlist.Id)
		} else {
			log.Fatalf("Error finding playlist: %v", err)
		}

		return
	case "check":
		if session, err := OpenSession(*checkFilename); err == nil {
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

	meta := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       *title,
			Description: *description,
			Tags:        *tags,
		},
		Status: &youtube.VideoStatus{PrivacyStatus: *privacy},
	}

	stat, err := os.Stat(*filename)
	size := stat.Size()

	if err == nil {
		finished := make(chan bool)
		ticker := time.NewTicker(time.Second * time.Duration(*interval))
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
