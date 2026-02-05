// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/regrada-ai/regrada-be/internal/domain"
	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/uptrace/bun"
)

type TestRunRepository struct {
	db *bun.DB
}

func NewTestRunRepository(db *bun.DB) *TestRunRepository {
	return &TestRunRepository{db: db}
}

func (r *TestRunRepository) Create(ctx context.Context, projectID string, testRun *domain.TestRun) error {
	resultsData, err := json.Marshal(testRun.Results)
	if err != nil {
		return err
	}

	violationsData, err := json.Marshal(testRun.Violations)
	if err != nil {
		return err
	}

	dbTestRun := &DBTestRun{
		ProjectID:        projectID,
		RunID:            testRun.RunID,
		Timestamp:        testRun.Timestamp,
		GitSHA:           testRun.GitSHA,
		GitBranch:        testRun.GitBranch,
		GitCommitMessage: testRun.GitCommitMessage,
		CIProvider:       testRun.CIProvider,
		CIPRNumber:       testRun.CIPRNumber,
		TotalCases:       testRun.TotalCases,
		PassedCases:      testRun.PassedCases,
		WarnedCases:      testRun.WarnedCases,
		FailedCases:      testRun.FailedCases,
		Results:          resultsData,
		Violations:       violationsData,
		Status:           testRun.Status,
	}

	if testRun.Status == "" {
		dbTestRun.Status = "completed"
	}

	_, err = r.db.NewInsert().Model(dbTestRun).Exec(ctx)
	return err
}

func (r *TestRunRepository) Get(ctx context.Context, projectID, runID string) (*domain.TestRun, error) {
	var dbTestRun DBTestRun
	err := r.db.NewSelect().
		Model(&dbTestRun).
		Where("project_id = ?", projectID).
		Where("run_id = ?", runID).
		Where("deleted_at IS NULL").
		Scan(ctx)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	testRun := &domain.TestRun{
		RunID:            dbTestRun.RunID,
		Timestamp:        dbTestRun.Timestamp,
		GitSHA:           dbTestRun.GitSHA,
		GitBranch:        dbTestRun.GitBranch,
		GitCommitMessage: dbTestRun.GitCommitMessage,
		CIProvider:       dbTestRun.CIProvider,
		CIPRNumber:       dbTestRun.CIPRNumber,
		TotalCases:       dbTestRun.TotalCases,
		PassedCases:      dbTestRun.PassedCases,
		WarnedCases:      dbTestRun.WarnedCases,
		FailedCases:      dbTestRun.FailedCases,
		Status:           dbTestRun.Status,
	}

	if len(dbTestRun.Results) == 0 {
		testRun.Results = []domain.CaseResult{}
	} else if err := decodeJSONField(dbTestRun.Results, &testRun.Results); err != nil {
		return nil, err
	}

	if len(dbTestRun.Violations) == 0 {
		testRun.Violations = []domain.Violation{}
	} else if err := decodeJSONField(dbTestRun.Violations, &testRun.Violations); err != nil {
		return nil, err
	}

	return testRun, nil
}

func (r *TestRunRepository) List(ctx context.Context, projectID string, limit, offset int) ([]*domain.TestRun, error) {
	var dbTestRuns []DBTestRun
	err := r.db.NewSelect().
		Model(&dbTestRuns).
		Where("project_id = ?", projectID).
		Where("deleted_at IS NULL").
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRun := &domain.TestRun{
			RunID:            dbTestRun.RunID,
			Timestamp:        dbTestRun.Timestamp,
			GitSHA:           dbTestRun.GitSHA,
			GitBranch:        dbTestRun.GitBranch,
			GitCommitMessage: dbTestRun.GitCommitMessage,
			CIProvider:       dbTestRun.CIProvider,
			CIPRNumber:       dbTestRun.CIPRNumber,
			TotalCases:       dbTestRun.TotalCases,
			PassedCases:      dbTestRun.PassedCases,
			WarnedCases:      dbTestRun.WarnedCases,
			FailedCases:      dbTestRun.FailedCases,
			Status:           dbTestRun.Status,
		}

		if len(dbTestRun.Results) == 0 {
			testRun.Results = []domain.CaseResult{}
		} else if err := decodeJSONField(dbTestRun.Results, &testRun.Results); err != nil {
			return nil, err
		}

		if len(dbTestRun.Violations) == 0 {
			testRun.Violations = []domain.Violation{}
		} else if err := decodeJSONField(dbTestRun.Violations, &testRun.Violations); err != nil {
			return nil, err
		}

		testRuns[i] = testRun
	}

	return testRuns, nil
}

func (r *TestRunRepository) Delete(ctx context.Context, projectID, runID string) error {
	res, err := r.db.NewUpdate().
		Model((*DBTestRun)(nil)).
		Set("deleted_at = ?", time.Now()).
		Where("project_id = ?", projectID).
		Where("run_id = ?", runID).
		Where("deleted_at IS NULL").
		Exec(ctx)

	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}
