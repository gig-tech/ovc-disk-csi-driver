// +build linux

/*
Copyright 2018-2019 GIG TECHNOLOGY NV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"io/ioutil"
	"strings"

	"github.com/gig-tech/ovc-sdk-go/ovc"
)

const uuidPath = "/sys/class/dmi/id/product_uuid"

func getMachineID(client *ovc.Client) (int, error) {
	nodeUUID, err := getMachineUUID()
	if err != nil {
		return -1, err
	}
	machine, err := client.Machines.GetByReferenceID(nodeUUID)
	if err != nil {
		return -1, err
	}

	return machine.ID, nil
}

// getMachineUUID returns the node product uuid in lowercase
func getMachineUUID() (string, error) {
	rawID, err := ioutil.ReadFile(uuidPath)
	if err != nil {
		return "", err
	}

	return strings.ToLower(strings.TrimSpace(string(rawID))), nil
}
