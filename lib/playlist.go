package uploader

import "net/http"

type PlaylistResult struct {
	Items         []Playlist       `json:"items"`
	NextPageToken string           `json:"nextPageToken"`
	PrevPageToken string           `json:"prevPageToken"`
	PageInfo      PlaylistPageInfo `json:"pageInfo"`
}

type PlaylistPageInfo struct {
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

func GetPlaylistByTitle(client *http.Client, title string) (*Playlist, error) {
	return nil, nil
}

func (playlist *Playlist) AddToPlaylist(id string) error {
	return nil
}
