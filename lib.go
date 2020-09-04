package jooki

import (
	"encoding/json"
	"strconv"
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
		return err
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
		return err
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
	Filename *string `json:"filename"`
	Format *string `json:"format"`
	HasImage bool `json:"hasImage"`
	Size *IntStr `json:"size"`
	Name *string `json:"title"`
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
