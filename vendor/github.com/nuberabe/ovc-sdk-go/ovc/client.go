package ovc

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/dgrijalva/jwt-go"
)

// Config used to connect to the API
type Config struct {
	Hostname     string
	ClientID     string
	ClientSecret string
}

// Credentials used to authenticate
type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// OvcClient struct
type OvcClient struct {
	JWT       string
	ServerURL string
	Access    string

	Machines     MachineService
	CloudSpaces  CloudSpaceService
	Accounts     AccountService
	Disks        DiskService
	Portforwards ForwardingService
	Templates    TemplateService
	Sizes        SizesService
	Images       ImageService
}

// Do sends and API Request and returns the body as an array of bytes
func (c *OvcClient) Do(req *http.Request) ([]byte, error) {
	var body []byte
	client := &http.Client{}
	req.Header.Set("Authorization", "bearer "+c.JWT)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	body, err = ioutil.ReadAll(resp.Body)
	log.Println("Status code: " + resp.Status)
	log.Println("Body: " + string(body))
	if resp.StatusCode > 202 {
		return body, errors.New(string(body))
	}
	if err != nil {
		return body, errors.New(string(body))
	}

	if err != nil {
		return body, errors.New(string(body))
	}
	return body, nil
}

// NewClient returns a OpenVCloud API Client
func NewClient(c *Config, url string) *OvcClient {
	client := &OvcClient{}

	tokenString := NewLogin(c)
	claims := jwt.MapClaims{}
	jwt.ParseWithClaims(tokenString, claims, nil)

	client.ServerURL = url + "/restmachine"
	client.JWT = tokenString
	client.Access = claims["username"].(string) + "@itsyouonline"

	client.Machines = &MachineServiceOp{client: client}
	client.CloudSpaces = &CloudSpaceServiceOp{client: client}
	client.Accounts = &AccountServiceOp{client: client}
	client.Disks = &DiskServiceOp{client: client}
	client.Portforwards = &ForwardingServiceOp{client: client}
	client.Templates = &TemplateServiceOp{client: client}
	client.Sizes = &SizesServiceOp{client: client}
	client.Images = &ImageServiceOp{client: client}
	return client
}

// NewLogin logs into the itsyouonline platform using the comfig struct
func NewLogin(c *Config) string {

	authForm := url.Values{}
	authForm.Add("grant_type", "client_credentials")
	authForm.Add("client_id", c.ClientID)
	authForm.Add("client_secret", c.ClientSecret)
	authForm.Add("response_type", "id_token")
	req, _ := http.NewRequest("POST", "https://itsyou.online/v1/oauth/access_token", strings.NewReader(authForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error performing login request")
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading body")
	}
	jwt := string(bodyBytes)
	defer resp.Body.Close()
	return jwt
}

// GetLocation parses the URL to return the location of the API
func (c *OvcClient) GetLocation() string {
	u, _ := url.Parse(c.ServerURL)
	hostName := u.Hostname()
	return hostName[:strings.IndexByte(hostName, '.')]
}
