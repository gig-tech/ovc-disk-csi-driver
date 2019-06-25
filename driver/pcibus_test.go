package driver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompPathBusSlot(t *testing.T) {
	tt := []struct {
		path string
		bus  int
		slot int
		pass bool
	}{
		{
			path: "virtio-pci-0000:00:0a.0",
			bus:  0,
			slot: 10,
			pass: true,
		},
		{
			path: "virtio-pci-0000:ab:cd",
			bus:  171,
			slot: 205,
			pass: true,
		},
		{
			path: "virtio-pci-0000:00:0a.0",
			bus:  0,
			slot: 10,
			pass: true,
		},
		{
			path: "pci-0000:00:17.0-ata-1",
			bus:  0,
			slot: 23,
			pass: true,
		},
		{
			path: "pci-0000:00:17",
			bus:  0,
			slot: 23,
			pass: true,
		},
		{
			path: "virtio-pci-0000:00:0a.0",
			bus:  0,
			slot: 16,
			pass: false,
		},
		{
			path: "foobar",
			bus:  0,
			slot: 1,
			pass: false,
		},
		{
			path: "foo:bar",
			bus:  0,
			slot: 1,
			pass: false,
		},
	}

	for _, tc := range tt {
		result := compPathBusSlot(tc.path, tc.bus, tc.slot)

		if tc.pass {
			require.True(t, result, "Expected path %s with bus %d and slot %d to pass", tc.path, tc.bus, tc.slot)
		} else {
			require.False(t, result, "Expected path %s with bus %d and slot %d to fail", tc.path, tc.bus, tc.slot)
		}
	}
}

func TestIsPart(t *testing.T) {
	tt := []struct {
		filename string
		pass     bool
	}{
		{
			filename: "pci-0000:00:17.0-ata-1",
			pass:     false,
		},
		{
			filename: "pci-0000:00:17.0-ata-1-part1",
			pass:     true,
		},
		{
			filename: "virtio-pci-0000:00:04.0",
			pass:     false,
		},
		{
			filename: "virtio-pci-0000:00:04.0-part1 ",
			pass:     true,
		}, {
			filename: "foobar",
			pass:     false,
		},
		{
			filename: "foobar-01part01",
			pass:     true,
		},
		{
			filename: "part",
			pass:     true,
		},
	}

	for _, tc := range tt {
		result := isPart(tc.filename)

		if tc.pass {
			require.True(t, result, "Expected %s to return true", tc.filename)
		} else {
			require.False(t, result, "Expected %s to return false", tc.filename)
		}
	}
}
