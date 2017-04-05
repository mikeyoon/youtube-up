package uploader

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/vektra/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"github.com/skratchdot/open-golang/open"
)

// getTokenFromWeb uses Config to request a Token.
// It returns the retrieved Token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	open.Run(authURL)

	fmt.Printf("Opening browser to get auth token. If it doesn't happen automatically, go to the " +
		"following link in your browser then type the authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		err = errors.Format("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		err = errors.Format("Unable to retrieve token from web %v", err)
	}
	return tok, err
}

// saveToken uses a file path to create a file and store the
// token in it.
func saveToken(file string, token *oauth2.Token) error {
	f, err := os.Create(file)
	if err != nil {
		return errors.Format("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)

	return nil
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
		url.QueryEscape("youtube-up.json")), err
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
func GetClient(ctx context.Context) *http.Client {
	cred, err := ioutil.ReadFile("./client_secrets.json")
	if err != nil {
		panic(errors.Format("Unable to open client_secrets.json.\n"+
			"If you need to create one, see https://github.com/jay0lee/GAM/wiki/CreatingClientSecretsFile\n"+
			"Error: %v", err))
	}

	config, err := google.ConfigFromJSON(cred, "https://www.googleapis.com/auth/youtube")
	if err != nil {
		panic(errors.New("Failed to parse oauth2 config from client_secrets.json."))
	}

	cacheFile, err := tokenCacheFile()
	if err != nil {
		panic(errors.New("Unable to make the path for the credential cache."))
	}

	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)

		if err == nil {
			err = saveToken(cacheFile, tok)
		}

		if err != nil {
			panic(err)
		}
	}

	return config.Client(ctx, tok)
}
