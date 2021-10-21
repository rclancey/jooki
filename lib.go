package jooki

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Playlist struct {
	ID *string `json:"-"`
	Audiobook *bool `json:audiobook"`
	Token *string `json:"star"`
	Name string `json:"title"`
	Tracks []string `json:"tracks"`
	URL *string `json:"url,omitempty"`
}

//http://streams.calmradio.com/api/303/128/stream

func (p *Playlist) String() string {
	parts := []string{}
	if p.ID != nil {
		parts = append(parts, fmt.Sprintf(`ID:"%s"`, *p.ID))
	}
	if p.Audiobook != nil {
		parts = append(parts, fmt.Sprintf(`Audiobook:"%t"`, *p.Audiobook))
	}
	if p.Token != nil {
		parts = append(parts, fmt.Sprintf(`Token:"%s"`, *p.Token))
	}
	parts = append(parts, fmt.Sprintf(`Name:"%s"`, p.Name))
	if p.Tracks != nil {
		parts = append(parts, fmt.Sprintf(`Tracks:"[%d]"`, len(p.Tracks)))
	}
	if p.URL != nil {
		parts = append(parts, fmt.Sprintf(`URL:"%s"`, *p.URL))
	}
	return fmt.Sprintf("&jooki.Playlist{%s}", strings.Join(parts, ", "))
}

func (p *Playlist) Clone() *Playlist {
	clone := &Playlist{}
	if p.Audiobook != nil {
		v := *p.Audiobook
		clone.Audiobook = &v
	}
	if p.Token != nil {
		v := *p.Token
		clone.Token = &v
	}
	clone.Name = p.Name
	clone.Tracks = make([]string, len(p.Tracks))
	for i, v := range p.Tracks {
		clone.Tracks[i] = v
	}
	return clone
}

type Token struct {
	ID *string `json:"-"`
	Seen int64 `json:"seen"`
	StarID string `json:"starId"`
}

func (t *Token) Clone() *Token {
	if t == nil {
		return nil
	}
	clone := *t
	return &clone
}

type FloatStr float64

func (fs FloatStr) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatFloat(float64(fs), 'f', -1, 64))
}

func (fs *FloatStr) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		var f float64
		xerr := json.Unmarshal(data, &f)
		if xerr != nil {
			return err
		}
		*fs = FloatStr(f)
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*fs = FloatStr(f)
	return nil
}

type IntStr int64

func (is IntStr) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(is), 10))
}

func (is *IntStr) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		var i int64
		xerr := json.Unmarshal(data, &i)
		if xerr != nil {
			return err
		}
		*is = IntStr(i)
		return nil
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*is = IntStr(i)
	return nil
}

type Track struct {
	ID *string `json:"-"`
	Album *string `json:"album"`
	Artist *string `json:"artist"`
	Codec *string `json:"codec"`
	Duration *FloatStr `json:"duration"`
	Location *string `json:"filename"`
	Format *string `json:"format"`
	HasImage bool `json:"hasImage"`
	Size *IntStr `json:"size"`
	Name *string `json:"title"`
}

func (t *Track) String() string {
	parts := []string{}
	if t.ID != nil {
		parts = append(parts, fmt.Sprintf(`ID:"%s"`, *t.ID))
	}
	if t.Album != nil {
		parts = append(parts, fmt.Sprintf(`Album:"%s"`, *t.Album))
	}
	if t.Artist != nil {
		parts = append(parts, fmt.Sprintf(`Artist:"%s"`, *t.Artist))
	}
	if t.Codec != nil {
		parts = append(parts, fmt.Sprintf(`Codec:"%s"`, *t.Codec))
	}
	if t.Duration != nil {
		parts = append(parts, fmt.Sprintf(`Duration:"%.3f"`, *t.Duration))
	}
	if t.Location != nil {
		parts = append(parts, fmt.Sprintf(`Location:"%s"`, *t.Location))
	}
	if t.Format != nil {
		parts = append(parts, fmt.Sprintf(`Format:"%s"`, *t.Format))
	}
	parts = append(parts, fmt.Sprintf(`HasImage:"%t"`, t.HasImage))
	if t.Size != nil {
		parts = append(parts, fmt.Sprintf(`Size:"%d"`, *t.Size))
	}
	if t.Name != nil {
		parts = append(parts, fmt.Sprintf(`Name:"%s"`, *t.Name))
	}
	return fmt.Sprintf("&jooki.Track{%s}", strings.Join(parts, ", "))
}

func (t *Track) Clone() *Track {
	if t == nil {
		return nil
	}
	clone := *t
	return &clone
}

type Library struct {
	Playlists map[string]*Playlist `json:"playlists"`
	Tokens map[string]*Token `json:"tokens"`
	Tracks map[string]*Track `json:"tracks"`
}

func (l *Library) UnmarshalJSON(data []byte) error {
	type rawLib struct {
		Playlists json.RawMessage `json:"playlists"`
		Tokens json.RawMessage `json:"tokens"`
		Tracks json.RawMessage `json:"tracks"`
	}
	raw := &rawLib{}
	err := json.Unmarshal(data, raw)
	if err != nil {
		return err
	}
	l.Playlists = map[string]*Playlist{}
	l.Tokens = map[string]*Token{}
	l.Tracks = map[string]*Track{}
	if len(raw.Playlists) > 0 && string(raw.Playlists) != "null" && string(raw.Playlists) != "[]" {
		err = json.Unmarshal([]byte(raw.Playlists), &l.Playlists)
		if err != nil {
			return err
		}
	}
	if len(raw.Tokens) > 0 && string(raw.Tokens) != "null" && string(raw.Tokens) != "[]" {
		err = json.Unmarshal([]byte(raw.Tokens), &l.Tokens)
		if err != nil {
			return err
		}
	}
	if len(raw.Tracks) > 0 && string(raw.Tracks) != "null" && string(raw.Tracks) != "[]" {
		err = json.Unmarshal([]byte(raw.Tracks), &l.Tracks)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *Library) Clone() *Library {
	if l == nil {
		return nil
	}
	clone := &Library{}
	if l.Playlists != nil {
		clone.Playlists = map[string]*Playlist{}
		for k, v := range l.Playlists {
			clone.Playlists[k] = v.Clone()
		}
	}
	if l.Tokens != nil {
		clone.Tokens = map[string]*Token{}
		for k, v := range l.Tokens {
			clone.Tokens[k] = v.Clone()
		}
	}
	if l.Tracks != nil {
		clone.Tracks = map[string]*Track{}
		for k, v := range l.Tracks {
			clone.Tracks[k] = v.Clone()
		}
	}
	return clone
}

type TrackSearch struct {
	JookiID *string
	Name *string
	Album *string
	Artist *string
	Size *uint64
	TotalTime *uint
}

func (l *Library) FindTrack(tr TrackSearch) *Track {
	if tr.JookiID != nil {
		id := *tr.JookiID
		jtr, ok := l.Tracks[id]
		if ok {
			jtr.ID = &id
			return jtr
		}
	}
	for id, jtr := range l.Tracks {
		if tr.Name == nil || *tr.Name == "" {
			if jtr.Name != nil && *jtr.Name != "" {
				continue
			}
		} else {
			if jtr.Name == nil || *jtr.Name != *tr.Name {
				continue
			}
		}
		if tr.Album != nil || *tr.Album == "" {
			if jtr.Album != nil && *jtr.Album != "" {
				continue
			}
		} else {
			if jtr.Album == nil || *jtr.Album != *tr.Album {
				continue
			}
		}
		if tr.Artist == nil || *tr.Artist == "" {
			if jtr.Artist != nil && *jtr.Artist != "" {
				continue
			}
		} else {
			if jtr.Artist == nil || *jtr.Artist != *tr.Artist {
				continue
			}
		}
		if tr.Size != nil && jtr.Size != nil {
			if int64(*tr.Size) != int64(*jtr.Size) {
				continue
			}
		}
		if tr.TotalTime != nil && jtr.Duration != nil {
			if math.Abs(float64(*tr.TotalTime) - float64(*jtr.Duration) * 1000) > 1000 {
				continue
			}
		}
		jtr.ID = &id
		return jtr
	}
	return nil
}
