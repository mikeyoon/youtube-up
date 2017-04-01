package uploader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/vektra/errors"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"strconv"
	"google.golang.org/api/youtube/v3"
	"net"
	"time"
)

type UploadSession struct {
	Url    string      `json:"url"`
	Client *http.Client `json:"-"`
	Size   int64       `json:"size"`
}

func parseRange(rangeHeader string) (int64, error) {
	if rangeHeader == "" {
		return 0, nil
	}

	r, err := regexp.Compile(`bytes=[0-9]+-(?P<start>[0-9]+)`)
	matches := r.FindStringSubmatch(rangeHeader)
	if matches != nil {
		start, err := strconv.Atoi(matches[1])
		return int64(start), err
	} else {
		err = errors.New("Error parsing range header: " + rangeHeader)
	}

	return 0, err
}

func (session *UploadSession) CheckSessionProgress() (int64, error) {
	req, err := http.NewRequest("PUT", session.Url, bytes.NewBufferString(""))
	if err == nil {
		req.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", session.Size))
	}

	resp, err := session.Client.Do(req)
	if err == nil {
		if resp.StatusCode == 308 {
			current, err := parseRange(resp.Header.Get("Range"))
			if err == nil {
				return current, nil // Add one because API returns 0-based number
			}
		} else if resp.StatusCode == 200 || resp.StatusCode == 201 {
			return -1, nil
		} else {
			raw, _ := httputil.DumpResponse(resp, true)
			return 0, errors.New(string(raw))
		}
	}

	return 0, err
}

func (session *UploadSession) Upload(filename string, offset int64) (*youtube.Video, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	if (offset > 0) {
		if _, err := file.Seek(offset + 1, 0); err != nil {
			return nil, err
		}
	}

	if err != nil {
		return nil, err
	}

	if req, err := http.NewRequest("PUT", session.Url, file); err == nil {
		req.Header.Add("Content-Type", "video/*")
		if (offset > 0) {
			firstByte := offset + 1
			req.ContentLength = session.Size - firstByte
			req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", firstByte, session.Size - 1, session.Size))
		} else {
			req.ContentLength = session.Size
		}

		for {
			resp, err := session.Client.Do(req)

			if err != nil {
				if err, ok := err.(net.Error); ok && err.Timeout() {
					time.Sleep(time.Second * 60)
					continue
				}

				return nil, err
			}

			if (resp.StatusCode != 201 && resp.StatusCode != 200) {
				body, err := ioutil.ReadAll(resp.Body)
				if err == nil {
					err = errors.New(fmt.Sprintf("Bad return code after upload: %d, %s", resp.StatusCode, string(body)))
				}
			} else {
				video := &youtube.Video{}
				err := json.NewDecoder(resp.Body).Decode(video)
				return video, err
			}

			break;
		}
	}

	return nil, err
}

func (session *UploadSession) Save(filename string) {
	f, err := os.Create(filename + ".session")
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(session)
}

func OpenSession(filename string) (*UploadSession, error) {
	f, err := os.Open(filename + ".session")
	if err != nil {
		return nil, err
	}

	t := &UploadSession{}
	err = json.NewDecoder(f).Decode(t)

	t.Client = GetClient(oauth2.NoContext)

	defer f.Close()
	return t, err
}

func CreateUploadSession(meta *youtube.Video, size int64, url string) (*UploadSession, error) {
	client := GetClient(oauth2.NoContext)

	options, err := json.Marshal(meta)
	if err != nil {
		log.Fatalf("Error reading options: %v", err)
	}
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(options))
	req.Header.Add("content-type", "application/json; charset=utf-8")
	req.Header.Add("X-Upload-Content-Length", fmt.Sprintf("%d", size))
	req.Header.Add("X-Upload-Content-Type", "video/*")

	response, err := client.Do(req)
	if err == nil {
		defer response.Body.Close()
		content, err := ioutil.ReadAll(response.Body)

		if response.StatusCode != 200 {
			err = errors.New(string(content))
		}

		if err == nil {
			return &UploadSession{Url: response.Header.Get("Location"), Size: size, Client: client}, nil
		}
	}

	return nil, err
}
