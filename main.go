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

var (
	accountSid = os.Getenv("TWILIO_ACCOUNT_SID")
	authToken  = os.Getenv("TWILIO_AUTH_TOKEN")
	sender     = os.Getenv("TWILIO_SENDER")
	urlStr     = fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%v/Messages.json", accountSid)
)

type twilioConfig struct {
	accountSid string
	authToken  string
	sender     string
	urlStr     string
}

func newTwilioConfig() (*twilioConfig, error) {
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
	l *log.Logger
	c *twilioConfig
}

func newSms(l *log.Logger, c *twilioConfig) *sms {
	return &sms{l, c}
}

func (s *sms) send(w http.ResponseWriter, r *http.Request) {
	rec, ok := r.URL.Query()["recipients"]
	if !ok {
		msg := "URL param 'recipients' is missing"
		s.l.Println(msg)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rec = strings.Split(rec[0], ",")

	var data alertmanagerPayload
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&data)
	if err != nil {
		s.l.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tpl, err := template.New("tpl").Parse("Status: {{.Status}}\n{{range .Alerts}}{{.Annotations.Summary}}: {{.Annotations.Description}}\n{{end}}")
	if err != nil {
		s.l.Println(err)
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
			msgData.Set("From", s.c.sender)
			msgData.Set("Body", smsBody.String())
			msgDataReader := *strings.NewReader(msgData.Encode())

			req, _ := http.NewRequest(http.MethodPost, s.c.urlStr, &msgDataReader)
			req.SetBasicAuth(s.c.accountSid, s.c.authToken)
			req.Header.Add("Accept", "application/json")
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				s.l.Println(err)
				return
			}

			var data map[string]interface{}
			decoder := json.NewDecoder(resp.Body)
			err = decoder.Decode(&data)
			if err != nil {
				s.l.Println(err)
				return
			}
			if resp.StatusCode >= 200 && resp.StatusCode <= 300 {
				s.l.Printf(
					"Severity=Info HttpStatus=%v SmsStatus=%v SmsID=%v SmsBody=%v",
					resp.StatusCode,
					data["status"],
					data["sid"],
					data["body"],
				)
			} else {
				s.l.Printf(
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
	c, err := newTwilioConfig()
	if err != nil {
		log.Fatalln(err)
	}

	l := log.New(os.Stdout, "PromTwilio ", log.LstdFlags)

	smsHandler := newSms(l, c)

	sm := mux.NewRouter()
	postRouter := sm.Methods(http.MethodPost).PathPrefix("/alert").Subrouter()
	postRouter.HandleFunc("/send", smsHandler.send)
	log.Fatalln(http.ListenAndServe(":8080", sm))
}
