package core

/*
  Onix Pilot Host Control Service
  Copyright (c) 2018-2021 by www.gatblau.org
  Licensed under the Apache License, Version 2.0 at http://www.apache.org/licenses/LICENSE-2.0
  Contributors to this project, hereby assign copyright in this code to the project,
  to be licensed under the same terms as the rest of the code.
*/
import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gatblau/onix/artisan/data"
	"time"
)

// Cmd command information for remote host execution
type Cmd struct {
	// the natural key uniquely identifying the command
	Key string `json:"key"`
	// description of the command
	Description string `json:"description"`
	// the package to use
	Package string `json:"package"`
	// the function in the package to call
	Function string `json:"function"`
	// the function input information
	Input *data.Input `json:"input"`
	// the package registry user
	User string `json:"user"`
	// the package registry password
	Pwd string `json:"pwd"`
	// enables verbose output
	Verbose bool `json:"verbose"`
	// run command in runtime
	Containerised bool `json:"containerised"`
}

func NewCmdRequest(value CmdValue) (*CmdRequest, error) {
	// create a signature for the command value
	signature, err := sign(value)
	if err != nil {
		return nil, fmt.Errorf("cannot sign command request: %s", err)
	}
	return &CmdRequest{
		Signature: signature,
		Value:     value,
	}, nil
}

// CmdRequest a command for execution with a job reference
type CmdRequest struct {
	Signature string   `json:"signature"`
	Value     CmdValue `json:"value"`
}

type CmdValue struct {
	JobId         int64       `json:"job_id"`
	Package       string      `json:"package"`
	Function      string      `json:"function"`
	User          string      `json:"user"`
	Pwd           string      `json:"pwd"`
	Verbose       bool        `json:"verbose"`
	Containerised bool        `json:"containerised"`
	Input         *data.Input `json:"input,omitempty"`
}

func (c *CmdValue) Value() string {
	var artCmd string
	// if command is to run in a runtime
	if c.Containerised {
		// use art exec
		artCmd = "exec"
	} else {
		// otherwise use art exe
		artCmd = "exe"
	}
	// if user credentials for the Artisan registry were provided
	if len(c.User) > 0 && len(c.Pwd) > 0 {
		// pass the credentials to the art cli
		return fmt.Sprintf("art %s -u %s:%s %s %s", artCmd, c.User, c.Pwd, c.Package, c.Function)
	}
	// otherwise run the command without credentials (assume public registry)
	return fmt.Sprintf("art %s %s %s", artCmd, c.Package, c.Function)
}

func (c *CmdValue) Env() []string {
	var vars []string
	// append vars
	for _, v := range c.Input.Var {
		vars = append(vars, fmt.Sprintf("%s=%s", v.Name, v.Value))
	}
	// append secrets
	for _, s := range c.Input.Secret {
		vars = append(vars, fmt.Sprintf("%s=%s", s.Name, s.Value))
	}
	return vars
}

func (c *CmdValue) PrintEnv() string {
	var vars bytes.Buffer
	vars.WriteString("printing variables passed to the shell\n{\n")
	for _, v := range c.Input.Var {
		vars.WriteString(fmt.Sprintf("%s=%s\n", v.Name, v.Value))
	}
	vars.WriteString("}\n")
	return vars.String()
}

// Host monitoring information
type Host struct {
	MachineId string `json:"machine_id"`
	OrgGroup  string `json:"org_group"`
	Org       string `json:"org"`
	Area      string `json:"area"`
	Location  string `json:"location"`
	Connected bool   `json:"connected"`
	LastSeen  int64  `json:"last_seen"`
	Since     int    `json:"since"`
	SinceType string `json:"since_type"`
}

// Registration information for host registration
type Registration struct {
	Hostname    string  `json:"hostname"`
	MachineId   string  `json:"machine_id"`
	OS          string  `json:"os"`
	Platform    string  `json:"platform"`
	Virtual     bool    `json:"virtual"`
	TotalMemory float64 `json:"total_memory"`
	CPUs        int     `json:"cpus"`
}

// Reader Get a JSON bytes reader for the Serializable
func (r *Registration) Reader() (*bytes.Reader, error) {
	jsonBytes, err := r.Bytes()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(*jsonBytes), err
}

// Bytes Get a []byte representing the Serializable
func (r *Registration) Bytes() (*[]byte, error) {
	b, err := ToJson(r)
	return &b, err
}

// Event host events
type Event struct {
	// 0: host up, 1: host down, 2: network up, 3: network down
	Type int       `json:"type"`
	Time time.Time `json:"time"`
}

type Events []Event

// Job a job to be executed on one or more hosts
type Job struct {
	Id        int64    `json:"id"`
	MachineId string   `json:"machine_id"`
	JobRef    string   `json:"job_ref"`
	FxKey     string   `json:"fx_key"`
	FxVersion int64    `json:"fx_version"`
	Created   string   `json:"created"`
	Started   string   `json:"started"`
	Completed string   `json:"completed"`
	Log       string   `json:"log"`
	Error     bool     `json:"error"`
	OrgGroup  string   `json:"org_group"`
	Org       string   `json:"org"`
	Area      string   `json:"area"`
	Location  string   `json:"location"`
	Tag       []string `json:"tag"`
}

type NewJobInfo struct {
	JobRef    string
	MachineId []string
	FxKey     string
	FxVersion int64
}

// ToJson convert the passed-in object to a JSON byte slice
// NOTE: json.Marshal is purposely not used as it will escape any < > characters
func ToJson(object interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	// switch off the escaping!
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(object)
	return buffer.Bytes(), err
}

type Area struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Org struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Location struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type Admission struct {
	MachineId string   `json:"machine_id"`
	OrgGroup  string   `json:"org_group"`
	Org       string   `json:"org"`
	Area      string   `json:"area"`
	Location  string   `json:"location"`
	Tag       []string `json:"tag"`
}

// Result
// note: ensure it is aligned with the same struct in piloth
type Result struct {
	// the unique job id
	JobId int64
	// indicates of the job was successful
	Success bool
	// the execution log for the job
	Log string
	// the error if any
	Err string
	// the completion time
	Time time.Time
}

func (r *Result) Reader() (*bytes.Reader, error) {
	jsonBytes, err := r.Bytes()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(*jsonBytes), err
}

func (r *Result) Bytes() (*[]byte, error) {
	b, err := ToJson(r)
	return &b, err
}

// PackageInfo describes a package and all its tags
type PackageInfo struct {
	Id   string   `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
	Ref  string   `json:"ref"`
}
