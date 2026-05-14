package services

import (
	"context"

	"github.com/ablate-ai/RuleFlow/database"
)

type MaintenanceService struct {
	db *database.DB
}

func NewMaintenanceService(db *database.DB) *MaintenanceService {
	return &MaintenanceService{db: db}
}

func (s *MaintenanceService) MigrateLegacyIDsToSnowflake(ctx context.Context) (*database.SnowflakeMigrationReport, error) {
	return s.db.MigrateLegacyIDsToSnowflake(ctx)
}
