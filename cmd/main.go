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

package main

import (
	"flag"
	"log"

	"github.com/gig-tech/ovc-disk-csi-driver/driver"
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
