package main

import (
	"errors"
	"net/url"
	"time"
)

// Service is a monitored HTTP endpoint.
type Service struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	IntervalSeconds int       `json:"interval_seconds"`
	ExpectedStatus  int       `json:"expected_status"`
	CreatedAt       time.Time `json:"created_at"`
}

func (s Service) Validate() error {
	if s.Name == "" || len(s.Name) > 100 {
		return errors.New("name is required (max 100 characters)")
	}
	u, err := url.Parse(s.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("url must be a valid http or https URL")
	}
	if s.IntervalSeconds < 5 || s.IntervalSeconds > 3600 {
		return errors.New("interval_seconds must be between 5 and 3600")
	}
	if s.ExpectedStatus < 100 || s.ExpectedStatus > 599 {
		return errors.New("expected_status must be a valid HTTP status code")
	}
	return nil
}

// Check is the result of a single probe, persisted in Postgres.
type Check struct {
	ID         int64     `json:"id"`
	ServiceID  int64     `json:"service_id"`
	CheckedAt  time.Time `json:"checked_at"`
	OK         bool      `json:"ok"`
	StatusCode int       `json:"status_code"`
	LatencyMS  int       `json:"latency_ms"`
	Error      string    `json:"error,omitempty"`
}

// Status is the current view of a service. The latest one is cached in Redis.
type Status struct {
	State      string     `json:"state"` // up | down | unknown
	CheckedAt  *time.Time `json:"checked_at,omitempty"`
	StatusCode int        `json:"status_code,omitempty"`
	LatencyMS  int        `json:"latency_ms"`
	Error      string     `json:"error,omitempty"`
	Uptime24h  *float64   `json:"uptime_24h,omitempty"` // percent, only on /status
	Checks24h  int        `json:"checks_24h,omitempty"` // only on /status
}

func (c Check) Status() Status {
	st := Status{
		State:      "down",
		CheckedAt:  &c.CheckedAt,
		StatusCode: c.StatusCode,
		LatencyMS:  c.LatencyMS,
		Error:      c.Error,
	}
	if c.OK {
		st.State = "up"
	}
	return st
}
