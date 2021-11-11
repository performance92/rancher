package config

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/creasty/defaults"
)

// LoadConfig reads a config json file and unmarshals it into an object depending on the config object.
// The functions takes a pointer of the object.
func LoadConfig(key string, config interface{}) {
	configPath := os.Getenv("CATTLE_TEST_CONFIG")
	var configMap map[string]interface{}

	configContents, err := ioutil.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(configContents, &configMap)
	if err != nil {
		panic(err)
	}

	configObject := configMap[key]
	jsonEncodedConfigObject, err := json.Marshal(configObject)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(jsonEncodedConfigObject, config)
	if err != nil {
		panic(err)
	}

	if err := defaults.Set(config); err != nil {
		panic(err)
	}

}
