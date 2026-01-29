// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package postgres

import (
	"context"
	"encoding/json"

	"github.com/uptrace/bun"
	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/regrada-ai/regrada-be/pkg/regrada"
)

type TestRunRepository struct {
	db *bun.DB
}

func NewTestRunRepository(db *bun.DB) *TestRunRepository {
	return &TestRunRepository{db: db}
}

func (r *TestRunRepository) Create(ctx context.Context, projectID string, testRun *regrada.TestRun) error {
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

func (r *TestRunRepository) Get(ctx context.Context, projectID, runID string) (*regrada.TestRun, error) {
	var dbTestRun DBTestRun
	err := r.db.NewSelect().
		Model(&dbTestRun).
		Where("project_id = ?", projectID).
		Where("run_id = ?", runID).
		Scan(ctx)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	testRun := &regrada.TestRun{
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

	if err := json.Unmarshal(dbTestRun.Results, &testRun.Results); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(dbTestRun.Violations, &testRun.Violations); err != nil {
		return nil, err
	}

	return testRun, nil
}

func (r *TestRunRepository) List(ctx context.Context, projectID string, limit, offset int) ([]*regrada.TestRun, error) {
	var dbTestRuns []DBTestRun
	err := r.db.NewSelect().
		Model(&dbTestRuns).
		Where("project_id = ?", projectID).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	testRuns := make([]*regrada.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRun := &regrada.TestRun{
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

		if err := json.Unmarshal(dbTestRun.Results, &testRun.Results); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(dbTestRun.Violations, &testRun.Violations); err != nil {
			return nil, err
		}

		testRuns[i] = testRun
	}

	return testRuns, nil
}
