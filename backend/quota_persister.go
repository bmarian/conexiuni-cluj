package main

import (
	"conexiuni-cluj/database"
	"time"
)

type dbQuotaPersister struct{}

func (dbQuotaPersister) Load(name string) (int, time.Time, error) {
	return database.LoadTranzyQuota(name)
}

func (dbQuotaPersister) Save(name string, count int, resetAt time.Time) error {
	return database.SaveTranzyQuota(name, count, resetAt)
}

func (dbQuotaPersister) RecordUsage(name string) {
	database.RecordTranzyQuotaUsage(name)
}
