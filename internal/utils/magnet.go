package utils

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type Magnet struct {
	InfoHash   string
	TrackerUrl string
	FileName   string
}

func ParseMagnetLink(magnet string) (*Magnet, error) {
	if len(magnet) < 9 || magnet[0:8] != "magnet:?" {
		return nil, errors.New("invalid magnet format")
	}

	parsed, err := url.Parse(magnet)
	if err != nil {
		return nil, err
	}

	params, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return nil, err
	}

	infoHash := params.Get("xt")
	if infoHash == "" || !strings.HasPrefix(infoHash, "urn:btih:") {
		fmt.Println(infoHash)
		return nil, errors.New("incorrect magnet format, missing xt")
	}

	parsedMagnet := &Magnet{
		InfoHash:   infoHash[9:],
		TrackerUrl: params.Get("tr"),
		FileName:   params.Get("dn"),
	}

	return parsedMagnet, nil
}
