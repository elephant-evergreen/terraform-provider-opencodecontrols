package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const accountsPath = "/console/api/service-accounts/"

var (
	accountPattern = regexp.MustCompile("^svcacct_[A-Za-z0-9_-]{1,116}$")
	keyIDPattern   = regexp.MustCompile("^sk_[A-Za-z0-9_-]{1,124}$")
)

type client struct {
	origin  string
	key     string
	version string
	http    *http.Client
}

type keyRecord struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Permissions string     `json:"permissions"`
	Scopes      []string   `json:"scopes"`
	Status      string     `json:"status"`
	TokenHint   string     `json:"tokenHint"`
	ExpiresAt   *time.Time `json:"expiresAt"`
}

type accountResponse struct {
	Account struct {
		ID string `json:"id"`
	} `json:"account"`
	Keys []keyRecord `json:"keys"`
}

type createKeyRequest struct {
	Name        string     `json:"name"`
	Permissions string     `json:"permissions"`
	Scopes      []string   `json:"scopes"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

type apiError struct {
	status int
}

func (e apiError) Error() string {
	return fmt.Sprintf("OpenCode Console returned HTTP %d.", e.status)
}

func apiStatus(err error, status int) bool {
	var response apiError
	return errors.As(err, &response) && response.status == status
}

func (c *client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return errors.New("Could not encode the OpenCode Console request.")
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.origin+path, body)
	if err != nil {
		return errors.New("Could not construct the OpenCode Console request.")
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "terraform-provider-opencodecontrols/"+c.version)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("Could not reach OpenCode Console. A write may have succeeded; check the account before retrying.")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError{status: resp.StatusCode}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out); err != nil {
		return errors.New("OpenCode Console returned an unreadable response. Check the account before retrying a write.")
	}
	return nil
}

func (c *client) keys(ctx context.Context, accountID string) ([]keyRecord, error) {
	if !accountPattern.MatchString(accountID) {
		return nil, errors.New("Use a service account ID starting with svcacct_.")
	}
	var result accountResponse
	if err := c.do(ctx, http.MethodGet, accountsPath+accountID, nil, &result); err != nil {
		return nil, err
	}
	if result.Account.ID != accountID || result.Keys == nil {
		return nil, errors.New("OpenCode Console returned an unexpected service account response.")
	}
	return result.Keys, nil
}

func findKey(keys []keyRecord, id string) *keyRecord {
	for i := range keys {
		if keys[i].ID == id {
			return &keys[i]
		}
	}
	return nil
}

func (c *client) create(ctx context.Context, accountID string, input createKeyRequest) (string, string, error) {
	if !accountPattern.MatchString(accountID) {
		return "", "", errors.New("Use a service account ID starting with svcacct_.")
	}
	var result struct {
		Key   keyRecord `json:"key"`
		Token string    `json:"token"`
	}
	if err := c.do(ctx, http.MethodPost, accountsPath+accountID+"/keys", input, &result); err != nil {
		return "", "", err
	}
	if !keyIDPattern.MatchString(result.Key.ID) || !tokenPattern.MatchString(result.Token) {
		return "", "", errors.New("OpenCode Console did not return a recognizable key ID and one-time value. Check the account before retrying.")
	}
	return result.Key.ID, result.Token, nil
}

func (c *client) revoke(ctx context.Context, id string) error {
	if !keyIDPattern.MatchString(id) {
		return errors.New("Use an API key ID starting with sk_.")
	}
	return c.do(ctx, http.MethodPost, accountsPath+"keys/"+id+"/revoke", nil, nil)
}

func (c *client) isManagementKey(key keyRecord) bool {
	if len(c.key) < 10 {
		return false
	}
	return key.TokenHint == c.key[6:10] || strings.HasPrefix(key.TokenHint, c.key[:10])
}
