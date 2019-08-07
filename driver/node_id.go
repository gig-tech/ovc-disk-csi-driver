package driver

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	nodeIDDelimeter = "@"
)

type nodeID struct {
	g8        string
	machineID int
}

func newNodeID(id string) (*nodeID, error) {
	if id == "" {
		return nil, fmt.Errorf("no node ID found")
	}

	nID := &nodeID{}

	parts := strings.Split(id, nodeIDDelimeter)
	if len(parts) < 2 {
		return nil, fmt.Errorf("node ID does not contain enough information: %s", id)
	} else if len(parts) > 2 {
		return nil, fmt.Errorf("node ID contains too many delimeter characters: %s", id)
	}

	machineID, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("Failed to convert disk ID '%s' into an integer", parts[1])
	}

	nID.g8 = parts[0]
	nID.machineID = machineID

	return nID, nil
}

func newNodeIDFromParts(g8 string, machineID int) *nodeID {
	return &nodeID{
		g8:        g8,
		machineID: machineID,
	}
}

func (v *nodeID) String() string {
	return fmt.Sprintf("%s%s%d", v.g8, nodeIDDelimeter, v.machineID)
}
