package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

type MailchimpClient struct {
	ApiKey string
	// for example "us16"
	Server string
	ListId string
}

type MailchimpMemberResponse struct {
	Status string `json:"status"`
}

func (m *MailchimpClient) CheckIfMemberOnList(email string) (bool, error) {
	if m == nil {
		return false, fmt.Errorf("mailchimp not configured")
	}
	subscriberHash := emailToSubscriberHash(email)
	url := fmt.Sprintf("https://%s.api.mailchimp.com/3.0/lists/%s/members/%s", m.Server, m.ListId, subscriberHash)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("Mailchimp: check if member on list:", err)
		return false, nil
	}

	req.SetBasicAuth("anystring", m.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Mailchimp: check if member on list:", err)
		return false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		var response MailchimpMemberResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return false, fmt.Errorf("failed to parse JSON: %v", err)
		}

		return response.Status == "subscribed", nil
	} else {
		return false, nil
	}
}

type AddMemberToListRequestData struct {
	EmailAddress string `json:"email_address"`
	Status       string `json:"status"`
}

func (m *MailchimpClient) AddMemberToList(email string) error {
	if m == nil {
		return fmt.Errorf("mailchimp not configured")
	}
	data := AddMemberToListRequestData{
		EmailAddress: email,
		Status:       "subscribed",
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Println("Mailchimp: add member to list:", err)
		return err
	}

	subscriberHash := emailToSubscriberHash(email)
	url := fmt.Sprintf("https://%s.api.mailchimp.com/3.0/lists/%s/members/%s", m.Server, m.ListId, subscriberHash)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("Mailchimp: add member to list:", err)
		return err
	}

	req.SetBasicAuth("anystring", m.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Mailchimp: add member to list:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("Mailchimp: add member to list:", err)
			return fmt.Errorf("failed to read response body: %v", err)
		}

		log.Println("Mailchimp: add member to list:", string(body))
		return fmt.Errorf("failed to add member to list: %s", body)
	}

	return nil
}

// The subscriber hash is "The MD5 hash of the lowercase version of the list member's email address."
func emailToSubscriberHash(email string) string {
	lowerEmail := strings.ToLower(email)
	hash := md5.Sum([]byte(lowerEmail))
	subscriberHash := hex.EncodeToString(hash[:])
	return subscriberHash
}

func (r *Router) SetMailchimp(w http.ResponseWriter, req *http.Request) {
	userId, _ := getUserIDFromContext(req.Context())

	row := r.db.QueryRow("SELECT name_num FROM users WHERE buck_id = ?1", userId)
	var nameNum string
	err := row.Scan(&nameNum)
	if err != nil {
		log.Println("Failed to get user:", err, userId)
		http.Redirect(w, req, "/signout", http.StatusTemporaryRedirect)
		return
	}

	email := nameNum + "@osu.edu"

	err = r.mailchimp.AddMemberToList(email)
	if err != nil {
		http.Error(w, "Failed to add member to list", http.StatusInternalServerError)
		return
	}

	err = Templates.ExecuteTemplate(w, "mailchimp.html.tpl", map[string]interface{}{
		"isOnMailingList": true,
	})
	if err != nil {
		log.Println("Failed to render template:", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}
