package postgres

import (
	"context"
	"encoding/json"

	"github.com/regrada-ai/regrada-be/internal/storage"
	"github.com/regrada-ai/regrada-be/pkg/regrada"
	"github.com/uptrace/bun"
)

type TraceRepository struct {
	db *bun.DB
}

func NewTraceRepository(db *bun.DB) *TraceRepository {
	return &TraceRepository{db: db}
}

func (r *TraceRepository) Create(ctx context.Context, projectID string, trace *regrada.Trace) error {
	requestData, err := json.Marshal(trace.Request)
	if err != nil {
		return err
	}

	responseData, err := json.Marshal(trace.Response)
	if err != nil {
		return err
	}

	dbTrace := &DBTrace{
		ProjectID:        projectID,
		TraceID:          trace.TraceID,
		Timestamp:        trace.Timestamp,
		Provider:         trace.Provider,
		Model:            trace.Model,
		Environment:      trace.Environment,
		GitSHA:           trace.GitSHA,
		GitBranch:        trace.GitBranch,
		RequestData:      requestData,
		ResponseData:     responseData,
		LatencyMS:        trace.Metrics.LatencyMS,
		TokensIn:         trace.Metrics.TokensIn,
		TokensOut:        trace.Metrics.TokensOut,
		RedactionApplied: trace.RedactionApplied,
		Tags:             trace.Tags,
	}

	_, err = r.db.NewInsert().Model(dbTrace).Exec(ctx)
	return err
}

func (r *TraceRepository) CreateBatch(ctx context.Context, projectID string, traces []regrada.Trace) error {
	if len(traces) == 0 {
		return nil
	}

	dbTraces := make([]*DBTrace, len(traces))
	for i, trace := range traces {
		requestData, err := json.Marshal(trace.Request)
		if err != nil {
			return err
		}

		responseData, err := json.Marshal(trace.Response)
		if err != nil {
			return err
		}

		dbTraces[i] = &DBTrace{
			ProjectID:        projectID,
			TraceID:          trace.TraceID,
			Timestamp:        trace.Timestamp,
			Provider:         trace.Provider,
			Model:            trace.Model,
			Environment:      trace.Environment,
			GitSHA:           trace.GitSHA,
			GitBranch:        trace.GitBranch,
			RequestData:      requestData,
			ResponseData:     responseData,
			LatencyMS:        trace.Metrics.LatencyMS,
			TokensIn:         trace.Metrics.TokensIn,
			TokensOut:        trace.Metrics.TokensOut,
			RedactionApplied: trace.RedactionApplied,
			Tags:             trace.Tags,
		}
	}

	_, err := r.db.NewInsert().Model(&dbTraces).Exec(ctx)
	return err
}

func (r *TraceRepository) Get(ctx context.Context, projectID, traceID string) (*regrada.Trace, error) {
	var dbTrace DBTrace
	err := r.db.NewSelect().
		Model(&dbTrace).
		Where("project_id = ?", projectID).
		Where("trace_id = ?", traceID).
		Scan(ctx)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	trace := &regrada.Trace{
		TraceID:          dbTrace.TraceID,
		Timestamp:        dbTrace.Timestamp,
		Provider:         dbTrace.Provider,
		Model:            dbTrace.Model,
		Environment:      dbTrace.Environment,
		GitSHA:           dbTrace.GitSHA,
		GitBranch:        dbTrace.GitBranch,
		RedactionApplied: dbTrace.RedactionApplied,
		Tags:             dbTrace.Tags,
		Metrics: regrada.TraceMetrics{
			LatencyMS: dbTrace.LatencyMS,
			TokensIn:  dbTrace.TokensIn,
			TokensOut: dbTrace.TokensOut,
		},
	}

	if err := decodeJSONField(dbTrace.RequestData, &trace.Request); err != nil {
		return nil, err
	}

	if err := decodeJSONField(dbTrace.ResponseData, &trace.Response); err != nil {
		return nil, err
	}

	return trace, nil
}

func (r *TraceRepository) List(ctx context.Context, projectID string, limit, offset int) ([]*regrada.Trace, error) {
	var dbTraces []DBTrace
	err := r.db.NewSelect().
		Model(&dbTraces).
		Where("project_id = ?", projectID).
		Order("timestamp DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	traces := make([]*regrada.Trace, len(dbTraces))
	for i, dbTrace := range dbTraces {
		trace := &regrada.Trace{
			TraceID:          dbTrace.TraceID,
			Timestamp:        dbTrace.Timestamp,
			Provider:         dbTrace.Provider,
			Model:            dbTrace.Model,
			Environment:      dbTrace.Environment,
			GitSHA:           dbTrace.GitSHA,
			GitBranch:        dbTrace.GitBranch,
			RedactionApplied: dbTrace.RedactionApplied,
			Tags:             dbTrace.Tags,
			Metrics: regrada.TraceMetrics{
				LatencyMS: dbTrace.LatencyMS,
				TokensIn:  dbTrace.TokensIn,
				TokensOut: dbTrace.TokensOut,
			},
		}

		if err := decodeJSONField(dbTrace.RequestData, &trace.Request); err != nil {
			return nil, err
		}

		if err := decodeJSONField(dbTrace.ResponseData, &trace.Response); err != nil {
			return nil, err
		}

		traces[i] = trace
	}

	return traces, nil
}
