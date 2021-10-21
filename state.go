package jooki

import (
	"encoding/json"
	//"log"
	"time"
)

type TimeOfDay struct {
	Hour uint8 `json:"hour"`
	Minute uint8 `json:"minute"`
}

func (t *TimeOfDay) Is(tm time.Time) bool {
	if tm.Hour() == int(t.Hour) {
		return tm.Minute() == int(t.Minute)
	}
	return false
}

func (t *TimeOfDay) Next(tm time.Time) time.Time {
	n := time.Date(tm.Year(), tm.Month(), tm.Day(), tm.Hour(), int(t.Minute), 0, 0, tm.Location())
	if n.Before(tm) {
		n = n.Add(time.Hour)
	}
	if n.Hour() == int(t.Hour) {
		return n
	}
	if n.Hour() < int(t.Hour) {
		n = n.Add(time.Hour * time.Duration(int(t.Hour) - n.Hour()))
	} else {
		n = n.Add(time.Hour * time.Duration(24 - (n.Hour() - int(t.Hour))))
	}
	return n
}

func (t *TimeOfDay) Clone() *TimeOfDay {
	if t == nil {
		return nil
	}
	clone := *t
	return &clone
}

type QuietTime struct {
	Active bool `json:"active"`
	Shutdown *TimeOfDay `json:"shutdown"`
}

func (q *QuietTime) Clone() *QuietTime {
	if q == nil {
		return nil
	}
	clone := *q
	clone.Shutdown = q.Shutdown.Clone()
	return &clone
}

type JookiSettings struct {
	QuietTime *QuietTime `json:"quietTime"`
}

func (s *JookiSettings) Clone() *JookiSettings {
	if s == nil {
		return nil
	}
	return &JookiSettings{
		QuietTime: s.QuietTime.Clone(),
	}
}

type RepeatMode uint8

func (rm RepeatMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(uint8(rm))
}

func (rm *RepeatMode) UnmarshalJSON(data []byte) error {
	var i uint8
	err := json.Unmarshal(data, &i)
	if err == nil {
		*rm = RepeatMode(i)
		return nil
	}
	var b bool
	err = json.Unmarshal(data, &b)
	if err != nil {
		return err
	}
	if b {
		*rm = RepeatMode(1)
	} else {
		*rm = RepeatMode(0)
	}
	return nil
}

const (
	PlayModeShuffle = 1
	PlayModeRepeat = 2
	RepeatModeOff = RepeatMode(0)
	RepeatModeOn = RepeatMode(1)
	RepeatModeOnce = RepeatMode(2)
)

type AudioConfig struct {
	RepeatMode RepeatMode `json:"repeat_mode"`
	ShuffleMode bool `json:"shuffle_mode"`
	Volume uint8 `json:"volume"`
}

func (a *AudioConfig) Clone() *AudioConfig {
	if a == nil {
		return nil
	}
	clone := *a
	return &clone
}

type ImageWrapper string

func (iw *ImageWrapper) MarshalJSON() ([]byte, error) {
	if iw == nil {
		return []byte("false"), nil
	}
	return json.Marshal(string(*iw))
}

func (iw *ImageWrapper) UnmarshalJSON(data []byte) error {
	if string(data) == "false" {
		*iw = ImageWrapper("")
		return nil
	}
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	*iw = ImageWrapper(s)
	return nil
}

type NowPlaying struct {
	Album *string `json:"album"`
	Artist *string `json:"artist"`
	Audiobook bool `json:"audiobook"`
	Duration *float64 `json:"duration_ms"`
	HasNext bool `json:"hasNext"`
	HasPrev bool `json:"hasPrev"`
	Image *ImageWrapper `json:"image"`
	PlaylistID *string `json:"playlistId"`
	Service *string `json:"service"`
	Source *string `json:"source"`
	Title *string `json:"track"`
	TrackID *string `json:"trackId"`
	TrackIndex *int `json:"trackIndex"`
	URI *string `json:"uri"`
}

func (n *NowPlaying) Clone() *NowPlaying {
	if n == nil {
		return nil
	}
	clone := *n
	// TODO: pointers?
	return &clone
}

const (
	PlaybackStateStarting = "STARTING"
	PlaybackStateEnded    = "ENDED"
	PlaybackStatePlaying  = "PLAYING"
	PlaybackStatePaused   = "PAUSED"
)

type Playback struct {
	Position int `json:"position_ms"`
	State string `json:"state"`
}

func (p *Playback) Clone() *Playback {
	if p == nil {
		return nil
	}
	clone := *p
	return &clone
}

type Audio struct {
	Config *AudioConfig `json:"config"`
	NowPlaying *NowPlaying `json:"nowPlaying"`
	Playback *Playback `json:"playback"`
}

func (a *Audio) UnmarshalJSON(data []byte) error {
	type rawAudio struct {
		Config *AudioConfig `json:"config"`
		NowPlaying json.RawMessage `json:"nowPlaying"`
		Playback json.RawMessage `json:"playback"`
	}
	raw := &rawAudio{}
	err := json.Unmarshal(data, raw)
	if err != nil {
		return err
	}
	a.Config = raw.Config
	if len(raw.NowPlaying) > 0 && string(raw.NowPlaying) != "null" && string(raw.NowPlaying) != "[]" {
		a.NowPlaying = &NowPlaying{}
		err = json.Unmarshal([]byte(raw.NowPlaying), a.NowPlaying)
		if err != nil {
			return err
		}
	}
	if len(raw.Playback) > 0 && string(raw.Playback) != "null" && string(raw.Playback) != "[]" {
		a.Playback = &Playback{}
		err = json.Unmarshal([]byte(raw.Playback), a.Playback)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Audio) Clone() *Audio {
	if a == nil {
		return nil
	}
	return &Audio{
		Config: a.Config.Clone(),
		NowPlaying: a.NowPlaying.Clone(),
		Playback: a.Playback.Clone(),
	}
}

type DiskUsage struct {
	Available int64 `json:"available"`
	Total int64 `json:"total"`
	Used int64 `json:"used"`
	UsedPercent int8 `json:"usedPercent"`
}

func (d *DiskUsage) Clone() *DiskUsage {
	if d == nil {
		return nil
	}
	clone := *d
	return &clone
}

type Device struct {
	DiskUsage *DiskUsage `json:"diskUsage"`
	Firmware string `json:"firmware"`
	Flags []interface{} `json:"flags"`
	Hostname string `json:"hostname"`
	ID string `json:"id"`
	IP string `json:"ip"`
	Machine string `json:"machine"`
	ToySafe bool `json:"toy_safe"`
	Usage string `json:"usage"`
	WebApp string `json:"webapp"`
	WiFiMac string `json:"wifi_mac"`
}

func (d *Device) Clone() *Device {
	if d == nil {
		return nil
	}
	clone := *d
	clone.DiskUsage = d.DiskUsage.Clone()
	// TODO: Flags
	return &clone
}

type Mender struct {
	Event string `json:"event"`
	State string `json:"state"`
}

func (m *Mender) Clone() *Mender {
	if m == nil {
		return nil
	}
	clone := *m
	return &clone
}

type Owner struct {
	Email string `json:"email"`
	FirstName string `json:"firstName"`
	LastName string `json:"lastName"`
	Marketing bool `json:"marketing"`
}

func (o *Owner) Clone() *Owner {
	if o == nil {
		return nil
	}
	clone := *o
	return &clone
}

type PowerLevel struct {
	MV int `json:"mv"`
	P int `json:"p"`
	T int `json:"t"`
}

func (p *PowerLevel) Clone() *PowerLevel {
	if p == nil {
		return nil
	}
	clone := *p
	return &clone
}

type Power struct {
	Charging bool `json:"charging"`
	Connected bool `json:"connected"`
	Level *PowerLevel `json:"level"`
}

func (p *Power) Clone() *Power {
	if p == nil {
		return nil
	}
	clone := *p
	clone.Level = p.Level.Clone()
	return &clone
}

type Spotify struct {
	Active bool `json:"active"`
}

func (s *Spotify) Clone() *Spotify {
	if s == nil {
		return nil
	}
	clone := *s
	return &clone
}

type WiFi struct {
	Signal int `json:"signal"`
	SSID string `json:"ssid"`
}

func (w *WiFi) Clone() *WiFi {
	if w == nil {
		return nil
	}
	clone := *w
	return &clone
}

type JookiState struct {
	Settings *JookiSettings `json:"DISABLEDsettings"`
	Audio *Audio `json:"audio"`
	Bluetooth string `json:"bt"`
	Library *Library `json:"db"`
	Deezer []interface{} `json:"deezer"`
	Device *Device `json:"device"`
	Mender *Mender `json:"mender"`
	NFC interface{} `json:"nfc"`
	Owner *Owner `json:"owner"`
	Power *Power `json:"power"`
	Spotify *Spotify `json:"spotify"`
	UserMessages []interface{} `json:"userMessages"`
	WiFi *WiFi `json:"wifi"`
}

func (s *JookiState) Clone() *JookiState {
	clone := *s
	clone.Settings = s.Settings.Clone()
	clone.Audio = s.Audio.Clone()
	clone.Library = s.Library.Clone()
	// TODO: Deezer
	clone.Device = s.Device.Clone()
	clone.Mender = s.Mender.Clone()
	// TODO: NFC
	clone.Owner = s.Owner.Clone()
	clone.Power = s.Power.Clone()
	clone.Spotify = s.Spotify.Clone()
	// TODO: UserMessages
	clone.WiFi = s.WiFi.Clone()
	return &clone
	/*
	data, err := json.Marshal(s)
	if err != nil {
		log.Println("error serializing jooki state:", err)
		return nil
	}
	c := &JookiState{}
	err = json.Unmarshal(data, c)
	if err != nil {
		log.Println("error deserializing jooki state:", err)
		return nil
	}
	return c
	*/
}

