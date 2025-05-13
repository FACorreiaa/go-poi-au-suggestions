package llmInteraction

import (
	"context"
	"log/slog"

	"github.com/FACorreiaa/go-poi-au-suggestions/internal/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ LLmInteractionRepository = (*PostgresLlmInteractionRepo)(nil)

type LLmInteractionRepository interface {
	SaveInteraction(ctx context.Context, interaction types.LlmInteraction) error
}

type PostgresLlmInteractionRepo struct {
	logger *slog.Logger
	pgpool *pgxpool.Pool
}

func NewPostgresLlmInteractionRepo(pgxpool *pgxpool.Pool, logger *slog.Logger) *PostgresLlmInteractionRepo {
	return &PostgresLlmInteractionRepo{
		logger: logger,
		pgpool: pgxpool,
	}
}

func (r *PostgresLlmInteractionRepo) SaveInteraction(ctx context.Context, interaction types.LlmInteraction) error {
	query := `
        INSERT INTO llm_interactions (
            user_id, prompt, response_text, model_used, latency_ms
        ) VALUES ($1, $2, $3, $4, $5)
    `
	_, err := r.pgpool.Exec(ctx, query,
		interaction.UserID, interaction.Prompt, interaction.ResponseText,
		interaction.ModelUsed, interaction.LatencyMs,
	)
	return err
}
