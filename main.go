package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/caarlos0/env/v10"
	"io"
	"net/http"
	"os"
	"time"
)

type Config struct {
	EpgStationBaseURL   string `env:"EPGSTATION_BASE_URL" envDefault:"http://localhost:8888"`
	DeleteOlderThanHour int    `env:"DELETE_OLDER_THAN_HOUR" envDefault:"336"`
}

type EPGStationClient struct {
	baseURL string
}

func NewEPGStationClient(baseURL string) EPGStationClient {
	return EPGStationClient{baseURL: baseURL}
}

type DeletionPolicy struct {
	DeleteOlderThanHour int
}

func NewDeletionPolicy(deleteOlderThanHour int) DeletionPolicy {
	// Default is 2 weeks
	return DeletionPolicy{DeleteOlderThanHour: deleteOlderThanHour}
}

type Records struct {
	RecordItems []RecordedItem `json:"records"`
	TotalCount  int64          `json:"total"`
}

type RecordedItem struct {
	Id          int64       `json:"id"`
	Name        string      `json:"name"`
	IsEncoding  bool        `json:"isEncoding"`
	IsProtected bool        `json:"isProtected"`
	StartAt     int64       `json:"startAt"`
	EndAt       int64       `json:"endAt"`
	VideoFiles  []VideoFile `json:"videoFiles"`
}

type VideoFile struct {
	Id       int64  `json:"id"`
	Name     string `json:"name"`
	FileName string `json:"filename"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
}

func BuildHttpClient() http.Client {
	tr := http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return http.Client{Transport: &tr}
}

func (client *EPGStationClient) GetRecorded() (*Records, error) {
	url := fmt.Sprintf("%s/api/recorded?isHalfWidth=true&limit=0", client.baseURL)
	hc := BuildHttpClient()
	//resp, err := http.Get(url)
	resp, err := hc.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var r Records
	json.Unmarshal(body, &r)

	return &r, nil
}

func (client *EPGStationClient) DeleteVideoFile(videoFileId int64) error {
	url := fmt.Sprintf("%s/api/videos/%d", client.baseURL, videoFileId)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	hc := BuildHttpClient()
	res, err := hc.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("Status code is not 200 but %d. response is %v\n", res.StatusCode, res)
	}

	return nil
}

func extractTargetRecordItems(src []RecordedItem, policy DeletionPolicy, dst *[]RecordedItem) {
	for _, record := range src {
		hasTS := false
		hasEncoded := false

		for _, vf := range record.VideoFiles {
			if vf.Type == "ts" {
				hasTS = true
			} else if vf.Type == "encoded" {
				hasEncoded = true
			}
		}

		elapsed := time.Since(time.UnixMilli(record.StartAt))

		if !record.IsProtected && hasTS && hasEncoded && elapsed.Hours() > float64(policy.DeleteOlderThanHour) {
			*dst = append(*dst, record)
		}
	}
}

func main() {
	var config Config
	err := env.Parse(&config)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	epgStationClient := NewEPGStationClient(config.EpgStationBaseURL)

	r, err := epgStationClient.GetRecorded()
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	policy := NewDeletionPolicy(config.DeleteOlderThanHour)
	dst := make([]RecordedItem, 0)
	extractTargetRecordItems(r.RecordItems, policy, &dst)

	for _, record := range dst {
		for _, videoFile := range record.VideoFiles {
			if videoFile.Type == "ts" {
				fmt.Printf("Delete videoFile id: %d, filename: %s\n", videoFile.Id, videoFile.FileName)
				err := epgStationClient.DeleteVideoFile(videoFile.Id)
				if err != nil {
					fmt.Print(err)
					continue
				}
				fmt.Printf("Success to delete\n")
			}
		}
	}
}
