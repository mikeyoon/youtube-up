package uploader

import (
	"bytes"
	"encoding/json"
	"github.com/vektra/errors"
	"io/ioutil"
	"net/http"
)

type playlistResult struct {
	Items         []Playlist        `json:"items"`
	NextPageToken string            `json:"nextPageToken"`
	PrevPageToken string            `json:"prevPageToken"`
	PageInfo      *playlistPageInfo `json:"pageInfo,omitempty"`
}

type playlistPageInfo struct {
	TotalResults   int `json:"totalResults"`
	ResultsPerPage int `json:"resultsPerPage"`
}

type Playlist struct {
	Id      string          `json:"id"`
	Snippet PlaylistSnippet `json:"snippet"`
}

type PlaylistSnippet struct {
	Title string `json:"title"`
}

type playlistItem struct {
	Snippet playlistItemSnippet `json:"snippet"`
}

type playlistItemSnippet struct {
	PlaylistId string               `json:"playlistId"`
	ResourceId playlistItemResource `json:"resourceId"`
}

type playlistItemResource struct {
	VideoId string `json:"videoId"`
	Kind    string `json:"kind"`
}

func GetPlaylistByTitle(client *http.Client, title string) (*Playlist, error) {
	url := "https://content.googleapis.com/youtube/v3/playlists?part=snippet,id&mine=true&maxResults=50"

	resp, err := client.Get(url)
	if err == nil {
		if resp.StatusCode == 200 {
			result := &playlistResult{}
			err = json.NewDecoder(resp.Body).Decode(result)

			if err == nil {
				for ii := range result.Items {
					if result.Items[ii].Snippet.Title == title {
						return &result.Items[ii], nil
					}
				}
			}

			err = errors.New("Playlist not found")
		} else {
			content, _ := ioutil.ReadAll(resp.Body)
			err = errors.New(string(content))
		}
	}

	return nil, err
}

func (playlist *Playlist) AddToPlaylist(client *http.Client, playlistId string, videoId string) error {
	url := "https://content.googleapis.com/youtube/v3/playlistItems?part=snippet&alt=json"

	item := playlistItem{Snippet: playlistItemSnippet{PlaylistId: playlistId, ResourceId: playlistItemResource{VideoId: videoId, Kind: "youtube#video"}}}
	data, _ := json.Marshal(item)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(data))
	if (resp.StatusCode != 200) {
		content, _ := ioutil.ReadAll(resp.Body)
		err = errors.New(string(content))
	}

	return err
}
