package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// BakalariClient represents a simple client for the Bakalari API.
type BakalariClient struct {
	BaseURL     string
	AccessToken string
}

// BakalariLogin performs password based authentication against a Bakalari server
// and returns a client with an access token on success.
func BakalariLogin(baseURL, username, password string) (*BakalariClient, error) {
	data := url.Values{}
	data.Set("client_id", "ANDR")
	data.Set("grant_type", "password")
	data.Set("username", username)
	data.Set("password", password)

	resp, err := http.PostForm(baseURL+"/api/login", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("missing token")
	}
	return &BakalariClient{BaseURL: baseURL, AccessToken: out.AccessToken}, nil
}

// ListMyStudents should retrieve all students available to the authenticated
// teacher account. The specific endpoint varies between installations and is
// therefore left unimplemented.
func (b *BakalariClient) ListMyStudents() ([]BakalariStudent, error) {
	return nil, fmt.Errorf("not implemented")
}

// BakalariStudent describes a student record returned by Bakalari.
type BakalariStudent struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Abbrev string `json:"Abbrev"`
}
