package health

import "time"

type Status struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

func Controller(serviceName, version string) Status {
	return Status{Service: serviceName, Status: "ok", Version: version, Timestamp: time.Now().UTC()}
}
