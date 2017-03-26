package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/alecthomas/kingpin"
	"github.com/vektra/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
)

const UPLOAD_URL = "https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status,contentDetails"

var (
	title       = kingpin.Flag("title", "Title of the video").String()
	description = kingpin.Flag("description", "Description of the video").String()
	playlist    = kingpin.Flag("playlist", "Playlist to add the video to").String()
	tags        = kingpin.Flag("tags", "Tags for the video").Strings()
	filename    = kingpin.Arg("filename", "Video file to upload").Required().String()
	privacy     = kingpin.Arg("privacy", "Privacy settings [public, unlisted, private]").Default("public").String()
)

type Snippet struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	CategoryId  int      `json:"categoryId,omitempty"`
}

type Status struct {
	PrivacyStatus string `json:",omitempty"`
	Embeddable    bool   `json:"embeddable,omitempty"`
	License       string `json:"license,omitempty"`
}

type Metadata struct {
	Snippet Snippet `json:"snippet,omitempty"`
	Status  Status  `json:"status,omitempty"`
}

type UploadSession struct {
	Url string
}

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}
	return tok
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) {
	f, err := os.Create(file)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

// tokenCacheFile generates credential file path/filename.
// It returns the generated credential path/filename.
func tokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir,
		url.QueryEscape("goyoutube-upload.json")), err
}

// tokenFromFile retrieves a Token from a given file path.
// It returns the retrieved Token and any read error encountered.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// getClient uses a Context and Config to retrieve a Token
// then generate a Client. It returns the generated Client.
func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

func saveSession(session *UploadSession) {
	f, err := os.Create(*filename + ".session")
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(session)
}

func openSession(filename string) (*UploadSession, error) {
	f, err := os.Open(filename + ".session")
	if err != nil {
		return nil, err
	}
	t := &UploadSession{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

func createUploadSession(client *http.Client, meta Metadata, size int64) (*UploadSession, error) {
	options, err := json.Marshal(meta)
	if err != nil {
		fmt.Println(err)
	}
	req, _ := http.NewRequest("POST", UPLOAD_URL, bytes.NewBuffer(options))
	req.Header.Add("content-type", "application/json; charset=utf-8")
	req.Header.Add("x-upload-content-length", fmt.Sprintf("%d", size))
	req.Header.Add("x-Upload-Content-Type", "video/mp4")

	response, err := client.Do(req)
	if err == nil {
		defer response.Body.Close()
		content, err := ioutil.ReadAll(response.Body)

		if response.StatusCode != 200 {
			err = errors.New(string(content))
		}

		if err == nil {
			return &UploadSession{Url: response.Header.Get("Location")}, nil
		}
	}

	return nil, err
}

func resumeSession(client *http.Client, session *UploadSession, size int64) (int64, error) {
	req, err := http.NewRequest("PUT", session.Url, bytes.NewBufferString(""))
	if err == nil {
		req.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", size))

		//raw, err := httputil.DumpRequestOut(req, true)
		//log.Println(string(raw))

		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}

		if resp.StatusCode == 308 {
			log.Print("Resuming Session...")
			rangeStr := resp.Header.Get("Range")
			if rangeStr != "" {
				r, _ := regexp.Compile(`bytes=[0-9]+-(?P<start>[0-9]+)`)
				matches := r.FindStringSubmatch(rangeStr)
				fmt.Println(matches[1])

				start, _ := strconv.Atoi(matches[1])
				log.Println("at " + string(start))
				return int64(start), nil
			} else {
				log.Println("at 0")
			}
		} else {
			log.Println("Resume received an unknown error code")
			raw, err := httputil.DumpResponse(resp, true)
			if err == nil {
				log.Fatalf(string(raw))
			}
		}
	}

	return 0, err
}

func uploadFile(client *http.Client, session *UploadSession, start int64, size int64) error {
	file, err := os.Open(*filename)
	if err != nil {
		return err
	}

	if _, err := file.Seek(start, 0); err != nil {
		return err
	}

	if err != nil {
		return err

	}

	if req, err := http.NewRequest("PUT", session.Url, file); err == nil {
		req.ContentLength = size - start
		req.Header.Set("Content-Range", fmt.Sprintf("%d-%d/%d", start, size-1, size))

		resp, err := client.Do(req)

		if err != nil {
			return err
		}

		if resp.StatusCode != 201 {
			body, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				log.Fatalf(string(body))
			}
		}

		fmt.Println(resp.Status)
	}

	return err
}

func main() {
	var client *http.Client
	var session *UploadSession

	kingpin.Parse()

	meta := Metadata{Snippet: Snippet{}, Status: Status{}}
	meta.Snippet.Title = *title
	meta.Snippet.Description = *description
	meta.Snippet.Tags = *tags
	meta.Status.PrivacyStatus = *privacy

	cred, err := ioutil.ReadFile("./client_secrets.json")
	if err != nil {
		fmt.Println(err)
	}

	oauth, err := google.ConfigFromJSON(cred, "https://www.googleapis.com/auth/youtube.upload")
	if err != nil {
		fmt.Println(err)
	}

	if err == nil {
		client = getClient(oauth2.NoContext, oauth)
	}

	stat, err := os.Stat(*filename)
	size := stat.Size()

	if err == nil {
		// Check if session already exists
		if session, err = openSession(*filename); err == nil {
			// Reopen session and continue upload if so
			start, err := resumeSession(client, session, size)

			if err == nil {
				err = uploadFile(client, session, start, size)
			}
		} else {
			// Open a new session
			session, err = createUploadSession(client, meta, size)
			saveSession(session)
			err = uploadFile(client, session, 0, size)
		}
	}

	if err == nil {
		log.Printf("Upload Successful")
		os.Remove(*filename + ".session")
		// Start/Resume upload
		// If failed due to connection issue, retry every 60 seconds
	}

	if err != nil {
		log.Fatalf("Error uploading video: %v", err)
	}

	// Delete session if successful
}
