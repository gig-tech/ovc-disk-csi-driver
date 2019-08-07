package driver

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	volumeIDDelimeter = "@"
)

type volumeID struct {
	g8     string
	diskID int
}

func newVolumeID(id string) (*volumeID, error) {
	if id == "" {
		return nil, fmt.Errorf("no volume ID found")
	}

	vID := &volumeID{}

	parts := strings.Split(id, volumeIDDelimeter)
	if len(parts) < 2 {
		return nil, fmt.Errorf("volume ID does not contain enough information: %s", id)
	} else if len(parts) > 2 {
		return nil, fmt.Errorf("volume ID contains too many delimeter characters: %s", id)
	}

	diskID, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("Failed to convert disk ID '%s' into an integer", parts[1])
	}

	vID.g8 = parts[0]
	vID.diskID = diskID

	return vID, nil
}

func newVolumeIDFromParts(g8 string, diskID int) *volumeID {
	return &volumeID{
		g8:     g8,
		diskID: diskID,
	}
}

func (v *volumeID) String() string {
	return fmt.Sprintf("%s%s%d", v.g8, volumeIDDelimeter, v.diskID)
}
