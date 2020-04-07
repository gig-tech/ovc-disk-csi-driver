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
	"os"

	"github.com/gig-tech/ovc-disk-csi-driver/driver"
)

func main() {
	var endpoint = flag.String("endpoint", "unix://tmp/csi.sock", "CSI Endpoint")
	var url = flag.String("url", "", "OVC URL")
	var account = flag.String("account", "", "Account name")
	var verbose = flag.Bool("verbose", false, "Set verbose output")
	var attacher = flag.Bool("attacher", false, "Add this flag on the attacher container")
	flag.Parse()

	ovcJWT := os.Getenv("OVC_JWT")

	print(verbose)

	drv, err := driver.NewDriver(*url, *endpoint, *account, nil, ovcJWT, *verbose, *attacher)
	if err != nil {
		log.Fatalln(err)
	}
	if err := drv.Run(); err != nil {
		log.Fatalln(err)
	}
}
