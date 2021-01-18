package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type twilioConfig struct {
	accountSid string
	authToken  string
	sender     string
	urlStr     string
}

func newTwilioConfig() (*twilioConfig, error) {
	accountSid := os.Getenv("TWILIO_ACCOUNT_SID")
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	sender := os.Getenv("TWILIO_SENDER")
	urlStr := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%v/Messages.json", accountSid)

	if accountSid == "" || authToken == "" || sender == "" {
		return nil, fmt.Errorf("Please set TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN and TWILIO_SENDER environment variables")
	}

	return &twilioConfig{
		accountSid: accountSid,
		authToken:  authToken,
		sender:     sender,
		urlStr:     urlStr,
	}, nil
}

type sms struct {
	logger *log.Logger
	config *twilioConfig
	client *http.Client
}

func newSms(logger *log.Logger, config *twilioConfig, client *http.Client) *sms {
	return &sms{logger, config, client}
}

func (s *sms) send(w http.ResponseWriter, r *http.Request) {
	rec, ok := r.URL.Query()["recipients"]
	if !ok {
		msg := "URL param 'recipients' is missing"
		s.logger.Println(msg)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rec = strings.Split(rec[0], ",")

	var data alertmanagerPayload
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&data)
	if err != nil {
		s.logger.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tpl, err := template.New("tpl").Parse("Status: {{.Status}}\n{{range .Alerts}}{{.Annotations.Summary}}: {{.Annotations.Description}}\n{{end}}")
	if err != nil {
		s.logger.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	smsBody := bytes.NewBuffer([]byte{})
	err = tpl.Execute(smsBody, data)

	var wg sync.WaitGroup
	wg.Add(len(rec))
	for _, r := range rec {
		go func(r string) {
			msgData := url.Values{}
			msgData.Set("To", fmt.Sprintf("+%v", r))
			msgData.Set("From", s.config.sender)
			msgData.Set("Body", smsBody.String())
			msgDataReader := *strings.NewReader(msgData.Encode())

			req, _ := http.NewRequest(http.MethodPost, s.config.urlStr, &msgDataReader)
			req.SetBasicAuth(s.config.accountSid, s.config.authToken)
			req.Header.Add("Accept", "application/json")
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			resp, err := s.client.Do(req)
			if err != nil {
				s.logger.Println(err)
				wg.Done()
				return
			}

			var data map[string]interface{}
			decoder := json.NewDecoder(resp.Body)
			err = decoder.Decode(&data)
			if err != nil {
				s.logger.Println(err)
				wg.Done()
				return
			}
			if resp.StatusCode >= 200 && resp.StatusCode <= 300 {
				s.logger.Printf(
					"Severity=Info HttpStatus=%v SmsStatus=%v SmsID=%v SmsBody=%v",
					resp.StatusCode,
					data["status"],
					data["sid"],
					data["body"],
				)
			} else {
				s.logger.Printf(
					"Severity=Error HttpStatus=%v SmsStatus=%v SmsCode=%v SmsMessage=%v",
					resp.StatusCode,
					data["status"],
					data["code"],
					data["message"],
				)
			}
			wg.Done()
		}(r)
	}
}

type alertmanagerPayload struct {
	Version string `json:"version"`
	Status  string `json:"status"`
	Alerts  []struct {
		Annotations struct {
			Summary     string `json:"summary"`
			Description string `json:"description"`
		} `json:"annotations"`
		StartsAt time.Time `json:"startsAt"`
	} `json:"alerts"`
}

func main() {
	config, err := newTwilioConfig()
	if err != nil {
		log.Fatalln(err)
	}

	logger := log.New(os.Stdout, "prometheus-twilio ", log.LstdFlags)
	client := &http.Client{}

	smsHandler := newSms(logger, config, client)

	serveMux := mux.NewRouter()
	postRouter := serveMux.Methods(http.MethodPost).PathPrefix("/alert").Subrouter()
	postRouter.HandleFunc("/send", smsHandler.send)
	log.Fatalln(http.ListenAndServe(":8080", serveMux))
}
