package novel

import (
	"encoding/json"
	"fmt"
)

type Spec struct {
    Topic       string
    Language    string
    Model       string
    Chapters    int
    Words       int
    Preset      string
    Instruction string
    System      string
    Gender      string
    Categories  []string
    Tags        []string
}

type Outline struct {
	Title    string    `json:"title"`
	Chapters []Chapter `json:"chapters"`
}

type Chapter struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type Character struct {
	Name       string     `json:"name"`
	Role       string     `json:"role"`
	Traits     StringList `json:"traits"`
	Background string     `json:"background"`
}

type ChapterContent struct {
	Index   int
	Title   string
	Content string
}

type Settings struct {
	Protagonist struct {
		Personality string `json:"personality"`
		Background  string `json:"background"`
		Goal        string `json:"goal"`
	} `json:"protagonist"`
	GoldenFinger struct {
		Name       string `json:"name"`
		Activation string `json:"activation"`
		Initial    string `json:"initial"`
		Upgrade    string `json:"upgrade"`
		Limit      string `json:"limit"`
	} `json:"golden_finger"`
	WorldFusion struct {
		Relations     string `json:"relations"`
		StartLocation string `json:"start_location"`
		InitialCrisis string `json:"initial_crisis"`
	} `json:"world_fusion"`
	Realms struct {
		Current      string            `json:"current"`
		Next         []string          `json:"next"`
		Breakthrough map[string]string `json:"breakthrough"`
	} `json:"realms"`
}

type StringList []string

func (s *StringList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		if one == "" {
			*s = []string{}
		} else {
			*s = []string{one}
		}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(b, &arr); err == nil {
		*s = arr
		return nil
	}
	var any []interface{}
	if err := json.Unmarshal(b, &any); err == nil {
		out := make([]string, 0, len(any))
		for _, v := range any {
			switch t := v.(type) {
			case string:
				out = append(out, t)
			default:
				out = append(out, fmt.Sprintf("%v", v))
			}
		}
		*s = out
		return nil
	}
	return fmt.Errorf("invalid traits format")
}
