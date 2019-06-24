// +build linux

package driver

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	disksByPathDir = "/dev/disk/by-path/"
)

func getDevicePath(log *logrus.Entry, pciBus, pciSlot string) (string, error) {
	fileInfo, err := ioutil.ReadDir(disksByPathDir)
	if err != nil {
		return "", err
	}

	for _, file := range fileInfo {
		if isPart(file.Name()) {
			log.Debugf("File %s is a partition, skipping", file.Name())
			continue
		}

		if !compPathBusSlot(file.Name(), pciBus, pciSlot) {
			continue
		}

		log.Debugf("%s matched bus %s slot %s", file.Name(), pciBus, pciSlot)

		resolvedLink, err := filepath.EvalSymlinks(fmt.Sprintf("%s/%s", disksByPathDir, file.Name()))
		if err != nil {
			return "", err
		}

		log.Debugf("Resolved device on pcibus %s to device %s", file.Name(), resolvedLink)

		return resolvedLink, nil
	}

	return "", fmt.Errorf("Device not found in pci bus %s, slot %s", pciBus, pciSlot)
}

// compPathBusSlot returns true if the bus and slot match in the linuxPCIBusName
func compPathBusSlot(linuxPCIBusName, bus, slot string) bool {
	if strings.HasPrefix(bus, "0x") {
		bus = bus[2:]
	}
	if strings.HasPrefix(slot, "0x") {
		slot = slot[2:]
	}

	parts := strings.Split(linuxPCIBusName, ":")
	// invalid path
	if len(parts) < 3 {
		return false
	}
	pathBus := parts[1]
	pathSlot := strings.Split(parts[2], ".")[0]

	if pathBus == bus && pathSlot == slot {
		return true
	}

	return false
}

// isPart returns true if filename is a partition
func isPart(filename string) bool {
	parts := strings.Split(filename, "-")
	if strings.Contains(parts[len(parts)-1], "part") {
		return true
	}

	return false
}
