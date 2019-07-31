package config

import "os"

// Driver represents a driver config
type Driver struct {
	Endpoint string
	Verbose  bool
	G8s      []G8
}

// G8 represents a G8 config
type G8 struct {
	Name    string
	URL     string
	Account string
	JWT     string
}

// CLI represents a config file passed to the cli
type CLI struct {
	Endpoint string `json:"endpoint"`
	G8s      []struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Account string `json:"account"`
		// Environmental variable that contains the JWT
		JWTEnv string `json:"jwtEnv"`
	} `json:"g8s"`
}

// GetDriverConfig fetches the JWT from the provided JWTEnv name to convert it to driver config
func (c *CLI) GetDriverConfig(verbose bool) *Driver {
	d := &Driver{}

	d.Verbose = verbose
	d.Endpoint = c.Endpoint

	for _, g8 := range c.G8s {
		dg8 := G8{}

		dg8.Name = g8.Name
		dg8.Account = g8.Account
		dg8.URL = g8.URL
		dg8.JWT = os.Getenv(g8.JWTEnv)

		d.G8s = append(d.G8s, dg8)
	}

	return d
}
