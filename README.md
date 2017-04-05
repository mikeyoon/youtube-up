# Youtube-up - Resumable Youtube video uploader
 
## Overview

Youtube-up is a command-line utility for uploading videos to Youtube.
Its interface is based on the python [youtube-upload](https://github.com/tokland/youtube-upload).
I wrote this version because I wanted to have resume support as well as a
stand-alone executable.

## Prerequisites

This application requires a client_secrets.json file to authenticate with the 
Google API. Directions are available 
[here](https://github.com/jay0lee/GAM/wiki/CreatingClientSecretsFile). Just follow up to
step 13, and name your project however you like. Just copy the resulting client_secrets.json
file to the same folder where youtube-up is.

## Usage

```
usage: youtube-up.exe [<flags>] <command> [<args> ...]                
                                                                            
Flags:                                                                      
  --help  Show context-sensitive help (also try --help-long and --help-man).
                                                                            
Commands:                                                                   
  help [<command>...]                                                       
    Show help.                                                              
                                                                            
  upload [<flags>] <filename>                                               
    Upload a video                                                          
                                                                            
  check <filename>                                                          
    Check progress of the [filename]                                        
                                                                            
  find playlist [<flags>]                                                   
    Find a playlist                                                         
```

There are three basic commands: upload, check, and find.

### Upload

```
usage: youtube-up.exe upload [<flags>] <filename>

Upload a video

Flags:
  --help                     Show context-sensitive help (also try --help-long
                             and --help-man).
  --title=TITLE              Title of the video
  --description=DESCRIPTION  Description of the video
  --playlist=PLAYLIST        Title of the playlist to include this video
  --tags=TAGS ...            Comma-separated list of tags for the video
  --privacy="public"         Privacy settings [public, unlisted, private]
  --interval=30              Timer interval for checking progress, in seconds.
                             Requires an API call each time and Youtube does not
                             update that frequently. Recommended that this not
                             be set lower than 10.

Args:
  <filename>  Path of the video to upload
```

Example: `youtube-up upload --title "Hello World" --playlist "My Playlist" test.mp4`

Note: if you use the playlist option, the title needs to match exactly, 
case and all. Also, the playlist search doesn't support paging, so it'll
need to be in the first 50 playlists are returned by the API.

#### Resuming

When an upload is started, a .session file is created for the video. If you
rerun the same command, the session file will be loaded and the video automatically
resumed.

### Check

If a video is interrupted for whatever reason, you can use this to check its
current progress without starting an upload. I mainly include this for debugging.

### Find

```
usage: youtube-up.exe find playlist [<flags>]

Find a playlist

Flags:
  --help         Show context-sensitive help (also try --help-long and
                 --help-man).
  --title=TITLE  Title of the playlist to find
```

Currently just allows finding the ID of a playlist by title. In the future
this might support other searching options if I find them useful.

Example: `youtube-up find playlist --title "My Playlist"`
