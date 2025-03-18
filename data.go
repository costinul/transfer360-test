package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type DataSource interface {
	ID() string
	SearchURL() string
}

type LeaseCompany struct {
	CompanyName  string `json:"companyname"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	AddressLine3 string `json:"addres_line3"`
	AddressLine4 string `json:"addres_line4"`
	Postcode     string `json:"postcode"`
}

type VehicleContravention struct {
	Reference         string       `json:"reference"`
	VRM               string       `json:"vrm"`
	ContraventionDate string       `json:"contravention_date"`
	IsHirerVehicle    bool         `json:"is_hirer_vehicle"`
	LeaseCompany      LeaseCompany `json:"lease_company"`
}

type SearchBody struct {
	VRM               string    `json:"vrm"`
	ContraventionDate time.Time `json:"contravention_date"`
}

type SearchRequest struct {
	VRM     string `json:"vrm"`
	Company string `json:"company"`
}

type acmelease struct{}

type leasecompany struct{}

type fleetcompany struct{}

type hirecompany struct{}

var dataSources = make(map[string]DataSource)

func initDataSources() {
	dataSources["ACME Company Ltd"] = &acmelease{}
	dataSources["Lease Company Ltd"] = &leasecompany{}
	dataSources["Fleet Company Ltd"] = &fleetcompany{}
	dataSources["Hire Company Ltd"] = &hirecompany{}
}

func getDataSource(id string) DataSource {
	return dataSources[id]
}

func (d *acmelease) ID() string {
	return "acmelease"
}

func (d *acmelease) SearchURL() string {
	return "https://sandbox-update.transfer360.dev/test_search/acmelease"
}

func (d *leasecompany) ID() string {
	return "leasecompany"
}

func (d *leasecompany) SearchURL() string {
	return "https://sandbox-update.transfer360.dev/test_search/leasecompany"
}

func (d *fleetcompany) ID() string {
	return "fleetcompany"
}

func (d *fleetcompany) SearchURL() string {
	return "https://sandbox-update.transfer360.dev/test_search/fleetcompany"
}

func (d *hirecompany) ID() string {
	return "hirecompany"
}

func (d *hirecompany) SearchURL() string {
	return "https://sandbox-update.transfer360.dev/test_search/hirecompany"
}

func SearchContravention(source DataSource, vrm string, contraventionDate time.Time) (*VehicleContravention, error) {
	log.Printf("Searching for %s in %s\n", vrm, source.ID())
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	searchBody := SearchBody{
		VRM:               vrm,
		ContraventionDate: contraventionDate,
	}

	jsonBody, err := json.Marshal(searchBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", source.SearchURL(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var contravention VehicleContravention
	err = json.NewDecoder(resp.Body).Decode(&contravention)
	if err != nil {
		return nil, err
	}
	return &contravention, nil
}
