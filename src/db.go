package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

var FILE_STATE_DB string
var MISTAKE_DB string
var SCORE_DB string

func init() {
	var ok bool
	var data string
	var home string

	if home, ok = os.LookupEnv("HOME"); !ok {
		die("Could not resolve home directory.")
	}

	if data, ok = os.LookupEnv("XDG_DATA_HOME"); ok {
		data = filepath.Join(data, "/nm")
	} else {
		data = filepath.Join(home, "/.local/share/nm")
	}

	os.MkdirAll(data, 0700)

	FILE_STATE_DB = filepath.Join(data, ".db")
	MISTAKE_DB = filepath.Join(data, ".errors")
	SCORE_DB = filepath.Join(data, ".scores")
}

func readValue(path string, o interface{}) error {
	b, err := ioutil.ReadFile(path)

	if err != nil {
		return err
	}

	return json.Unmarshal(b, o)
}

func writeValue(path string, o interface{}) error {
	b, err := json.Marshal(o)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, b, 0600)
}
