package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/xshellinc/tools/lib/help"
)

type Configuration struct {
	Token             string `json:"token"`
	Email             string `json:"email"`
	MAC               string `json:"mac"`
	Allowed           bool   `json:"allowed"`
	GithubAccessToken string `json:"githubAccessToken"`
}

var s string = help.Separator()
var HomeDirIsaax string = help.UserHomeDir() + s + ".isaax"
var configFile string = help.UserHomeDir() + s + ".isaax" + s + "config"

func CreateHomeDir() {
	help.CreateDir(HomeDirIsaax)
}

func loadConfig() Configuration {
	var config Configuration
	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Debug("Config File Missing. ", err)
		fmt.Println("config file missing")
		config = newConfig()
	}

	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Debug("Config Parse Error: ", err)
		fmt.Println("config file parsing error", err.Error())
		config = newConfig()
	}
	return config
}

func newConfig() Configuration {
	var config Configuration
	jsonString, _ := json.Marshal(config)
	help.DeleteFile(configFile)
	CreateHomeDir()
	help.CreateFile(configFile)
	help.WriteFile(configFile, string(jsonString))
	return config
}

func SaveConfig(c Configuration) {
	jsonString, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Println("[-] Error saving config file: ", err.Error())
		os.Exit(1)
	}
	help.WriteFile(configFile, string(jsonString))
}

func Logout() {
	newConfig()
}

func SetToken(token string) {
	config := loadConfig()
	config.Token = token
	SaveConfig(config)
}

func Token() string {
	config := loadConfig()
	return config.Token
}

func SetEmail(email string) {
	config := loadConfig()
	config.Email = email
	SaveConfig(config)
}

func SetMAC(mac string) {
	config := loadConfig()
	config.MAC = mac
	SaveConfig(config)
}
func Email() string {
	config := loadConfig()
	return config.Email
}

func SetAllowed(allowed bool) {
	config := loadConfig()
	config.Allowed = allowed
	SaveConfig(config)
}

func SetGithubAccessToken(githubAccessToken string) {
	config := loadConfig()
	config.GithubAccessToken = githubAccessToken
	SaveConfig(config)
}

func UserId() string {
	base64string := strings.Split(Token(), ".")
	b64data := base64string[1] + "="
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		log.Debug("error:", err)
	}

	type jwtStruct struct {
		Id string `json:"Id"`
	}

	var jwt jwtStruct
	err = json.Unmarshal([]byte(data), &jwt)
	if err != nil {
		log.Debug("Config Parse Error: ", err)
	}
	return jwt.Id
}
