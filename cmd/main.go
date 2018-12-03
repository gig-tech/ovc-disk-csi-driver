package main

import (
	"flag"
	"log"

	"github.com/ovc-disk-csi-driver/driver"
)

func main() {
	var endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
	var url = flag.String("url", "", "OVC URL")
	var gid = flag.Int("gid", 0, "OVC Grid ID")
	var accountID = flag.Int("account_id", 0, "Account ID")
	var nodeID = flag.String("nodeid", "", "ID of the node")
	flag.Parse()

	drv, err := driver.NewDriver(*url, *endpoint, *nodeID, *accountID, *gid, nil)
	if err != nil {
		log.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		log.Fatalln(err)
	}
}
