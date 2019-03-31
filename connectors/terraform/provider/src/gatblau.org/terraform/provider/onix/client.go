/*
   Onix CMDB - Copyright (c) 2018-2019 by www.gatblau.org

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software distributed under
   the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
   either express or implied.
   See the License for the specific language governing permissions and limitations under the License.

   Contributors to this project, hereby assign copyright in this code to the project,
   to be licensed under the same terms as the rest of the code.
*/
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	DELETE = "DELETE"
	PUT    = "PUT"
	GET    = "GET"
)

type Client struct {
	BaseURL string
	Token   string
}

type Result struct {
	Changed   bool   `json:"changed"`
	Error     bool   `json:"error"`
	Message   string `json:"message"`
	Operation string `json:"operation"`
	Ref       string `json:"ref"`
}

func (o *Client) initBasicAuthToken(user string, pwd string) {
	o.Token = fmt.Sprintf("Basic %s",
		base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", user, pwd))))
}

func (o *Client) MakeRequest(method string, resourceName string, key string, payload io.Reader) (*Result, error) {
	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s/%s", o.BaseURL, resourceName, key), payload)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", o.Token)
	response, _ := http.DefaultClient.Do(req)
	defer response.Body.Close()
	result := new(Result)
	json.NewDecoder(response.Body).Decode(result)
	return result, err
}

func (o *Client) Put(resourceName string, key string, payload io.Reader) (*Result, error) {
	return o.MakeRequest(PUT, resourceName, key, payload)
}

func (o *Client) Delete(resourceName string, key string) (*Result, error) {
	return o.MakeRequest(DELETE, resourceName, key, nil)
}

func (o *Client) Get(resourceName string, key string) (interface{}, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s", o.BaseURL, resourceName, key), nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", o.Token)
	resp, err := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	switch {
	case resourceName == "item":
		result := new(Item)
		json.NewDecoder(resp.Body).Decode(result)
		return *result, err
	case resourceName == "itemtype":
		result := new(ItemType)
		json.NewDecoder(resp.Body).Decode(result)
		return *result, err
	case resourceName == "link":
		result := new(Link)
		json.NewDecoder(resp.Body).Decode(result)
		return *result, err
	case resourceName == "linktype":
		result := new(LinkType)
		json.NewDecoder(resp.Body).Decode(result)
		return *result, err
	case resourceName == "model":
		result := new(Model)
		json.NewDecoder(resp.Body).Decode(result)
		return *result, err
	}
	return nil, nil
}
