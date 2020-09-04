package jooki

import (
	"bytes"
	//"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"time"
)

type TrackUpload interface {
	ContentType() string
	FileName() string
	Reader() (io.ReadCloser, error)
}

type PlaylistUpdate struct {
	ID string `json:"id"`
	Tracks []string `json:"tracks,omitempty"`
	Title *string `json:"title,omitempty"`
	Token *string `json:"star,omitempty"`
}

type PlaylistUpdateWrapper struct {
	Playlist *PlaylistUpdate `json:"playlist"`
}

type PlaylistCreate struct {
	Title *string `json:"title"`
	Audiobook bool `json:"audiobook"`
}

type PlaylistAddTrack struct {
	ID string `json:"playlistId"`
	TrackID string `json:"trackId"`
}

type PlaylistPlay struct {
	ID string `json:"playlistId"`
	TrackIndex int `json:"trackIndex"`
}

type PlaylistAddUpload struct {
	ID string `json:"playlistId"`
	UploadID int `json:"uploadId"`
	Filename string `json:"filename"`
}

type PlaylistDelete struct {
	ID string `json:"playlistId"`
}

type SetVol struct {
	Volume int `json:"vol"`
}

type SetShuffle struct {
	ShuffleMode bool `json:"shuffle_mode"`
}

type SetRepeat struct {
	RepeatMode int `json:"repeat_mode"`
}

type SetSeek struct {
	Position int `json:"position_ms"`
}

func (c *Client) CreatePlaylist(title string) (*Playlist, error) {
	/*
	tmpTitleBytes := make([]byte, 12)
	_, err := rand.Read(tmpTitleBytes)
	if err != nil {
		return nil, err
	}
	tmpTitle := base64.StdEncoding.EncodeToString(tmpTitleBytes)
	*/
	msg := &PlaylistCreate{
		//Title: &tmpTitle,
		Title: &title,
		Audiobook: false,
	}
	prevPlaylists := map[string]*Playlist{}
	state := c.GetState()
	if state != nil && state.Library != nil {
		prevPlaylists = state.Library.Playlists
	}
	a, err := c.publishWithAwaiter("/j/web/input/PLAYLIST_NEW", msg)
	if err != nil {
		return nil, err
	}
	defer a.Close()
	timer := time.NewTimer(time.Second * 10)
	for {
		update, ok := a.Read(timer)
		if !ok {
			return nil, errors.New("can't find newly created playlist")
		}
		if update.After.Library == nil || update.After.Library.Playlists == nil {
			continue
		}
		for k, v := range update.After.Library.Playlists {
			if _, ok := prevPlaylists[k]; !ok {
				if v.Name == *msg.Title {
					v.ID = &k
					return v, nil
				}
			}
		}
	}
	return nil, errors.New("can't find newly created playlist")
	/*
	for k, pl := range update.After.Library.Playlists {
		if _, ok := update.Before.Library.Playlists[k]; !ok {
			if pl.Title == tmpTitle {
				playlistId = k
				err = c.RenamePlaylist(playlistId, title)
				return playlistId, err
			}
		}
	}
	return "", errors.New("can't find newly created playlist")
	*/
}

func (c *Client) PlayPlaylist(id string, idx int) (*Audio, error) {
	msg := &PlaylistPlay{ID: id, TrackIndex: idx}
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.NowPlaying == nil || state.Audio.Playback == nil {
			return false
		}
		if state.Audio.Playback.State != PlaybackStatePlaying {
			return false
		}
		if state.Audio.NowPlaying.PlaylistID == nil || *state.Audio.NowPlaying.PlaylistID != id {
			return false
		}
		return true
	}
	state, err := c.publishAndWaitFor("/j/web/input/PLAYLIST_PLAY", msg, f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}

func (c *Client) UpdatePlaylist(update *PlaylistUpdate) (*Playlist, error) {
	msg := &PlaylistUpdateWrapper{Playlist: update}
	f := func(state *JookiState) bool {
		if state == nil || state.Library == nil {
			return false
		}
		pl, ok := state.Library.Playlists[update.ID]
		if !ok {
			return false
		}
		if update.Title != nil && pl.Name != *update.Title {
			return false
		}
		if update.Token != nil && (pl.Token == nil || *pl.Token != *update.Token) {
			return false
		}
		if len(update.Tracks) > 0 && len(update.Tracks) != len(pl.Tracks) {
			return false
		}
		for i, tr := range update.Tracks {
			if tr != pl.Tracks[i] {
				return false
			}
		}
		return true
	}
	state, err := c.publishAndWaitFor("/j/web/input/PLAYLIST_UPDATE", msg, f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Library.Playlists[update.ID], nil
}

func (c *Client) UpdatePlaylistTracks(id string, trackIds []string) (*Playlist, error) {
	msg := &PlaylistUpdate{
		ID: id,
		Tracks: trackIds,
	}
	return c.UpdatePlaylist(msg)
}

func (c *Client) RenamePlaylist(id, title string) (*Playlist, error) {
	msg := &PlaylistUpdate{
		ID: id,
		Title: &title,
	}
	return c.UpdatePlaylist(msg)
}

func (c *Client) UpdatePlaylistToken(id, token string) (*Playlist, error) {
	msg := &PlaylistUpdate{
		ID: id,
		Token: &token,
	}
	return c.UpdatePlaylist(msg)
}

func (c *Client) UploadToPlaylist(id string, track TrackUpload) (*Track, error) {
	uploadId := rand.Int()
	u := &url.URL{
		Scheme: "http",
		Host: c.device.Hostname,
		Path: "/upload",
	}
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%d"; filename="%s"`, uploadId, filepath.Base(track.FileName())))
	h.Set("Content-Type", track.ContentType())
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, err
	}
	r, err := track.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	size, err := io.Copy(part, r)
	if err != nil {
		return nil, err
	}
	w.Close()

	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	hc := &http.Client{}

	prevTracks := map[string]*Track{}
	state := c.GetState()
	if state != nil && state.Library != nil {
		prevTracks = state.Library.Tracks
	}

	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("track upload failed with HTTP %d", res.StatusCode)
	}
	msg := &PlaylistAddUpload{
		ID: id,
		UploadID: uploadId,
		Filename: filepath.Base(track.FileName()),
	}
	a, err := c.AddAwaiter()
	if err != nil {
		return nil, err
	}
	defer a.Close()
	err = c.publish("/j/web/input/PLAYLIST_ADD_UPLOAD", msg)
	if err != nil {
		return nil, err
	}
	timer := time.NewTimer(time.Minute)
	for {
		update, ok := a.Read(timer)
		if !ok {
			return nil, errors.New("can't find newly uploaded track")
		}
		if update.After.Library == nil || update.After.Library.Tracks == nil {
			continue
		}
		for k, v := range update.After.Library.Tracks {
			if _, ok := prevTracks[k]; !ok {
				if v.Size != nil && int64(*v.Size) == size {
					return v, nil
				}
			}
		}
	}
	return nil, errors.New("can't find newly uploaded track")
}

func (c *Client) AddTrackToPlaylist(playlistId, trackId string) (*Playlist, error) {
	msg := &PlaylistAddTrack{
		ID: playlistId,
		TrackID: trackId,
	}
	f := func(state *JookiState) bool {
		if state == nil || state.Library == nil {
			return false
		}
		pl, ok := state.Library.Playlists[playlistId]
		if !ok || pl == nil || len(pl.Tracks) == 0 {
			return false
		}
		return pl.Tracks[len(pl.Tracks) - 1] == trackId
	}
	state, err := c.publishAndWaitFor("/j/web/input/PLAYLIST_ADD_TRACK", msg, f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Library.Playlists[playlistId], nil
}

func (c *Client) DeletePlaylist(id string) error {
	msg := &PlaylistDelete{ID: id}
	f := func(state *JookiState) bool {
		if state == nil || state.Library == nil {
			return false
		}
		_, ok := state.Library.Playlists[id]
		return ok
	}
	_, err := c.publishAndWaitFor("/j/web/input/PLAYLIST_DELETE", msg, f, time.Second * 5)
	return err
}

func (c *Client) Play() (*Audio, error) {
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.NowPlaying == nil || state.Audio.Playback == nil {
			return false
		}
		return state.Audio.Playback.State == PlaybackStatePlaying
	}
	state, err := c.publishAndWaitFor("/j/web/input/DO_PLAY", "{}", f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}

func (c *Client) Pause() (*Audio, error) {
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.Playback == nil {
			return false
		}
		return state.Audio.Playback.State == PlaybackStatePaused || state.Audio.Playback.State == PlaybackStateEnded
	}
	state, err := c.publishAndWaitFor("/j/web/input/DO_PAUSE", "{}", f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}

func (c *Client) SetVolume(vol int) (*Audio, error) {
	msg := &SetVol{Volume: vol}
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.Config == nil {
			return false
		}
		return state.Audio.Config.Volume == uint8(vol)
	}
	state, err := c.publishAndWaitFor("/j/web/input/SET_VOL", msg, f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}

func (c *Client) SetShuffleMode(on bool) (*Audio, error) {
	msg := &SetShuffle{ShuffleMode: on}
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.Config == nil {
			return false
		}
		return state.Audio.Config.ShuffleMode == on
	}
	state, err := c.publishAndWaitFor("/j/web/input/SET_CFG", msg, f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}

func (c *Client) SetRepeatMode(mode RepeatMode) (*Audio, error) {
	msg := &SetRepeat{RepeatMode: int(mode)}
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.Config == nil {
			return false
		}
		return state.Audio.Config.RepeatMode == mode
	}
	state, err := c.publishAndWaitFor("/j/web/input/SET_CFG", msg, f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}

func (c *Client) SkipNext() (*Audio, error) {
	before := c.GetState()
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.NowPlaying == nil {
			return false
		}
		if before == nil || before.Audio == nil || before.Audio.NowPlaying == nil {
			return true
		}
		if before.Audio.NowPlaying.TrackID == nil {
			return state.Audio.NowPlaying.TrackID != nil
		}
		if state.Audio.NowPlaying.TrackID == nil {
			return true
		}
		return *before.Audio.NowPlaying.TrackID != *state.Audio.NowPlaying.TrackID
	}
	state, err := c.publishAndWaitFor("/j/web/input/DO_NEXT", "{}", f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}

func (c *Client) SkipPrev() (*Audio, error) {
	before := c.GetState()
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.NowPlaying == nil {
			return false
		}
		if before == nil || before.Audio == nil || before.Audio.NowPlaying == nil {
			return true
		}
		if before.Audio.NowPlaying.TrackID == nil {
			return state.Audio.NowPlaying.TrackID != nil
		}
		if state.Audio.NowPlaying.TrackID == nil {
			return true
		}
		return *before.Audio.NowPlaying.TrackID != *state.Audio.NowPlaying.TrackID
	}
	state, err := c.publishAndWaitFor("/j/web/input/DO_PREV", "{}", f, time.Second * 5)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}

func (c *Client) Seek(ms int) (*Audio, error) {
	f := func(state *JookiState) bool {
		if state == nil || state.Audio == nil {
			return false
		}
		if state.Audio.Playback == nil {
			return false
		}
		return state.Audio.Playback.Position >= ms - 1000 && state.Audio.Playback.Position <= ms + 1000
	}
	state, err := c.publishAndWaitFor("/j/web/input/SEEK", &SetSeek{Position: ms}, f, time.Second * 2)
	if err != nil {
		return nil, err
	}
	return state.Audio, nil
}
