package jooki

import (
	"bytes"
	//"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
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
	MD5() string
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
	msg := &PlaylistPlay{ID: id, TrackIndex: idx + 1}
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

type ProgressBody struct {
	buf *bytes.Buffer
	size *int
	pos int
	Progress chan float64
	finished bool
}

func NewProgressBody() *ProgressBody {
	return &ProgressBody{
		buf: bytes.NewBuffer([]byte{}),
		pos: 0,
		Progress: make(chan float64, 1024),
		finished: false,
	}
}

func (pb *ProgressBody) Write(data []byte) (int, error) {
	return pb.buf.Write(data)
}

func (pb *ProgressBody) Read(dst []byte) (int, error) {
	if pb.size == nil {
		s := pb.buf.Len()
		pb.size = &s
	}
	n, err := pb.buf.Read(dst)
	pb.pos += n
	if !pb.finished {
		pb.Progress <- pb.UploadProgress()
		if n == 0 || err == io.EOF {
			pb.finished = true
			close(pb.Progress)
		}
	}
	return n, err
}

func (pb *ProgressBody) UploadProgress() float64 {
	return float64(pb.pos) / float64(*pb.size)
}

func (pb *ProgressBody) Len() int {
	if pb.size == nil {
		return pb.buf.Len()
	}
	return *pb.size
}

type ProgressUpdate struct {
	FileName string
	UploadID int
	UploadProgress float64
	Track *Track
	Err error
}

func (c *Client) UploadToPlaylist(id string, track TrackUpload, ch chan ProgressUpdate) (*Track, error) {
	defer close(ch)
	md5 := track.MD5()[:16]
	c.hc.CloseIdleConnections()
	uploadId := int(rand.Intn(1e7))
	progUpdate := ProgressUpdate{FileName: track.FileName(), UploadID: uploadId}
	ch <- progUpdate
	u := &url.URL{
		Scheme: "http",
		Host: c.device.Hostname,
		Path: "/upload",
	}
	body := NewProgressBody()
	go func() {
		for {
			prog, ok := <-body.Progress
			if !ok {
				break
			}
			progUpdate.UploadProgress = prog
			ch <- progUpdate
		}
	}()
	w := multipart.NewWriter(body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%d"; filename="%s"`, uploadId, filepath.Base(track.FileName())))
	h.Set("Content-Type", track.ContentType())
	part, err := w.CreatePart(h)
	if err != nil {
		progUpdate.Err = err
		ch <- progUpdate
		return nil, err
	}
	r, err := track.Reader()
	if err != nil {
		progUpdate.Err = err
		ch <- progUpdate
		return nil, err
	}
	defer r.Close()
	size, err := io.Copy(part, r)
	if err != nil {
		progUpdate.Err = err
		ch <- progUpdate
		return nil, err
	}
	w.Close()

	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		progUpdate.Err = err
		ch <- progUpdate
		return nil, err
	}
	req.ContentLength = int64(body.Len())
	req.Header.Set("Content-Type", w.FormDataContentType())

	prevTracks := map[string]*Track{}
	state := c.GetState()
	if state != nil && state.Library != nil {
		prevTracks = state.Library.Tracks
	}

	res, err := c.hc.Do(req)
	if err != nil {
		log.Printf("error uploading %d: %s", uploadId, err)
		progUpdate.Err = err
		ch <- progUpdate
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		log.Println("track upload failed with HTTP %d", res.StatusCode)
		progUpdate.Err = err
		ch <- progUpdate
		return nil, fmt.Errorf("track upload failed with HTTP %d", res.StatusCode)
	}
	log.Printf("track upload %d success", uploadId)
	msg := &PlaylistAddUpload{
		ID: id,
		UploadID: uploadId,
		Filename: filepath.Base(track.FileName()),
	}
	a, err := c.AddAwaiter()
	if err != nil {
		progUpdate.Err = err
		ch <- progUpdate
		return nil, err
	}
	defer a.Close()
	log.Printf("send mqtt message: %#v", msg)
	err = c.publish("/j/web/input/PLAYLIST_ADD_UPLOAD", msg)
	if err != nil {
		progUpdate.Err = err
		ch <- progUpdate
		return nil, err
	}
	timer := time.NewTimer(time.Minute)
	for {
		log.Println("looking for new track id")
		update, ok := a.Read(timer)
		if !ok {
			log.Println("read failed, can't find newly uploaded track")
			return nil, errors.New("can't find newly uploaded track")
		}
		if update.After.Library == nil || update.After.Library.Tracks == nil {
			continue
		}
		v, ok := update.After.Library.Tracks[md5]
		if ok {
			v.ID = &md5
			log.Printf("found uploaded track %s = %s", md5, v)
			progUpdate.Track = v
			ch <- progUpdate
			return v, nil
		}
		for k, v := range update.After.Library.Tracks {
			if _, ok := prevTracks[k]; !ok {
				if v.Size != nil && int64(*v.Size) == size {
					v.ID = &k
					log.Printf("found uploaded track %s = %s", k, v)
					progUpdate.Track = v
					ch <- progUpdate
					return v, nil
				} else if v.Size != nil {
					log.Printf("new track %s has wrong size (%d != %d): %s", k, *v.Size, size, v)
				} else {
					log.Printf("new track %s missing size: %s", k, v)
				}
			}
		}
	}
	log.Println("failed to find newly uploaded track")
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

func (c *Client) SetPlayMode(mode int) (*Audio, error) {
	shuffleOn := (mode & PlayModeShuffle) != 0
	repeatMode := RepeatModeOff
	if (mode & PlayModeRepeat) != 0 {
		repeatMode = RepeatModeOnce
	}
	_, err := c.SetShuffleMode(shuffleOn)
	if err != nil {
		return nil, err
	}
	return c.SetRepeatMode(repeatMode)
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
